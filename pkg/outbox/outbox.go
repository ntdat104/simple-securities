// Package outbox implements the transactional outbox pattern.
//
// Write path: inside the SAME GORM transaction that changes business state, call Add().
// After the transaction commits, call Dispatcher.Notify(): a separate goroutine wakes up
// immediately and publishes pending rows to Kafka (low latency). A poll ticker is the safety net
// for crashes between commit and publish, so delivery is guaranteed (at-least-once).
//
// Ordering: exactly one dispatcher per service is active at a time (leader elected through a
// Postgres session advisory lock). It publishes rows strictly by id, so per-key ordering on Kafka
// matches commit order. Standby instances take over within seconds if the leader dies.
package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"simple-securities/pkg/database"
	"simple-securities/pkg/events"
	"simple-securities/pkg/inbox"
	"simple-securities/pkg/kafka"
)

// Event is a pending/published outbox row (table outbox_events).
type Event struct {
	ID           int64  `gorm:"primaryKey;autoIncrement;index:idx_outbox_pending,where:status = 0"`
	EventID      string `gorm:"type:uuid;not null;uniqueIndex"`
	Topic        string `gorm:"not null"`
	PartitionKey string `gorm:"not null"`
	EventType    string `gorm:"not null"`
	AggregateID  string `gorm:"not null"`
	Payload      []byte `gorm:"type:jsonb;not null"`
	Status       int16  `gorm:"not null;default:0"` // 0 pending, 1 published
	Attempts     int    `gorm:"not null;default:0"`
	LastError    *string
	CreatedAt    time.Time  `gorm:"not null;default:now()"`
	PublishedAt  *time.Time `gorm:"index:idx_outbox_published,where:status = 1"`
}

// TableName implements gorm.Tabler.
func (Event) TableName() string { return "outbox_events" }

// Models returns the outbox and inbox models; every service passes them to database.Migrate.
func Models() []any { return []any{&Event{}, &inbox.Event{}} }

var (
	mPublished = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "outbox_published_total", Help: "Outbox events published to Kafka",
	}, []string{"service"})
	mFailed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "outbox_publish_failures_total", Help: "Outbox publish failures",
	}, []string{"service"})
	mLag = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "outbox_lag_seconds", Help: "Age of the oldest pending outbox event",
	}, []string{"service"})
)

// Add stores an event in the outbox using the caller's transaction.
func Add(ctx context.Context, tx *gorm.DB, topic, key string, env events.Envelope) error {
	payload, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}
	row := Event{EventID: env.ID, Topic: topic, PartitionKey: key, EventType: env.Type,
		AggregateID: env.AggregateID, Payload: payload, CreatedAt: time.Now().UTC()}
	if err := tx.WithContext(ctx).Create(&row).Error; err != nil {
		return fmt.Errorf("insert outbox: %w", err)
	}
	return nil
}

// Config tunes the dispatcher.
type Config struct {
	Service      string
	BatchSize    int           // default 500
	PollInterval time.Duration // safety net poll, default 1s
	Retention    time.Duration // published rows retention, default 72h
}

// Dispatcher relays outbox rows to Kafka.
type Dispatcher struct {
	cfg    Config
	db     *gorm.DB
	prod   *kafka.Producer
	log    *zap.Logger
	wake   chan struct{}
	lockID int64
}

// NewDispatcher creates a dispatcher; Run must be started in its own goroutine.
func NewDispatcher(cfg Config, db *gorm.DB, prod *kafka.Producer, log *zap.Logger) *Dispatcher {
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 500
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = time.Second
	}
	if cfg.Retention == 0 {
		cfg.Retention = 72 * time.Hour
	}
	return &Dispatcher{cfg: cfg, db: db, prod: prod, log: log.Named("outbox"),
		wake: make(chan struct{}, 1), lockID: database.LockKey("outbox:" + cfg.Service)}
}

// Notify wakes the dispatcher goroutine right after a transaction committed. Never blocks.
func (d *Dispatcher) Notify() {
	select {
	case d.wake <- struct{}{}:
	default:
	}
}

// Run elects a leader and relays events until ctx is cancelled.
func (d *Dispatcher) Run(ctx context.Context) error {
	go d.cleanupLoop(ctx)
	for ctx.Err() == nil {
		if err := d.lead(ctx); err != nil && ctx.Err() == nil {
			d.log.Warn("dispatcher leadership lost", zap.Error(err))
		}
		select {
		case <-ctx.Done():
		case <-time.After(2 * time.Second):
		}
	}
	return nil
}

// lead holds the advisory lock on a dedicated connection and relays while it is held.
func (d *Dispatcher) lead(ctx context.Context) error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	var ok bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, d.lockID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return nil // another instance is the leader; retry later
	}
	defer func() {
		ctx2, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(ctx2, `SELECT pg_advisory_unlock($1)`, d.lockID)
	}()
	// A GORM session bound to the leader connection (the lock is session scoped).
	tx, err := gorm.Open(postgres.New(postgres.Config{Conn: conn}), &gorm.Config{
		Logger: d.db.Config.Logger, SkipDefaultTransaction: true})
	if err != nil {
		return err
	}
	d.log.Info("outbox dispatcher is leader")

	ticker := time.NewTicker(d.cfg.PollInterval)
	defer ticker.Stop()
	backoff := 100 * time.Millisecond
	for {
		n, err := d.relayBatch(ctx, tx)
		switch {
		case err != nil:
			mFailed.WithLabelValues(d.cfg.Service).Inc()
			d.log.Warn("relay failed", zap.Error(err))
			if perr := conn.PingContext(ctx); perr != nil {
				return perr // leader connection lost: give up leadership
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, 5*time.Second)
			continue
		case n == d.cfg.BatchSize:
			backoff = 100 * time.Millisecond
			continue // more pending, drain immediately
		}
		backoff = 100 * time.Millisecond
		select {
		case <-ctx.Done():
			return nil
		case <-d.wake:
		case <-ticker.C:
		}
	}
}

func (d *Dispatcher) relayBatch(ctx context.Context, db *gorm.DB) (int, error) {
	var rows []Event
	if err := db.WithContext(ctx).Where("status = 0").Order("id").Limit(d.cfg.BatchSize).Find(&rows).Error; err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		mLag.WithLabelValues(d.cfg.Service).Set(0)
		return 0, nil
	}
	mLag.WithLabelValues(d.cfg.Service).Set(time.Since(rows[0].CreatedAt).Seconds())

	ids := make([]int64, len(rows))
	recs := make([]*kgo.Record, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
		recs[i] = &kgo.Record{Topic: r.Topic, Key: []byte(r.PartitionKey), Value: r.Payload,
			Headers: []kgo.RecordHeader{{Key: "event_id", Value: []byte(r.EventID)}, {Key: "event_type", Value: []byte(r.EventType)}}}
	}
	if err := d.prod.ProduceSync(ctx, recs...); err != nil {
		db.WithContext(ctx).Model(&Event{}).Where("id IN ?", ids).
			Updates(map[string]any{"attempts": gorm.Expr("attempts + 1"), "last_error": err.Error()})
		return 0, fmt.Errorf("produce: %w", err)
	}
	if err := db.WithContext(ctx).Model(&Event{}).Where("id IN ?", ids).
		Updates(map[string]any{"status": 1, "published_at": time.Now().UTC()}).Error; err != nil {
		// Events are on Kafka but not marked: they will be re-sent; consumers dedup via inbox.
		return 0, err
	}
	mPublished.WithLabelValues(d.cfg.Service).Add(float64(len(ids)))
	return len(ids), nil
}

func (d *Dispatcher) cleanupLoop(ctx context.Context) {
	t := time.NewTicker(10 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			cutoff := time.Now().Add(-d.cfg.Retention)
			db := d.db.WithContext(ctx)
			err := db.Exec(`DELETE FROM outbox_events WHERE id IN (
				SELECT id FROM outbox_events WHERE status = 1 AND published_at < ? LIMIT 10000)`, cutoff).Error
			if err == nil {
				err = db.Exec(`DELETE FROM inbox_events WHERE ctid IN (
					SELECT ctid FROM inbox_events WHERE received_at < ? LIMIT 10000)`, cutoff).Error
			}
			if err != nil && ctx.Err() == nil {
				d.log.Warn("cleanup failed", zap.Error(err))
			}
		}
	}
}
