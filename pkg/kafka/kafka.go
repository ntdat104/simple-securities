// Package kafka wraps franz-go with the three primitives the platform needs:
//   - Producer: idempotent, acks=all producer (used by the outbox dispatcher and market streams).
//   - GroupConsumer: at-least-once consumer group, per-partition batches, manual commit after the
//     handler (and its DB transaction with inbox dedup) succeeded, retry + DLQ for poison messages.
//   - TailConsumer: group-less "broadcast" consumer that starts at the end (each instance sees all).
package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"

	"simple-securities/pkg/events"
)

// Message is a decoded Kafka record.
type Message struct {
	Topic     string
	Partition int32
	Offset    int64
	Key       []byte
	Envelope  events.Envelope
}

// EnsureTopics creates the given topics (name -> partitions) if they do not exist.
func EnsureTopics(ctx context.Context, brokers []string, topics map[string]int32, replication int16) error {
	cl, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		return err
	}
	defer cl.Close()
	adm := kadm.NewClient(cl)
	for attempt := 1; ; attempt++ {
		err = nil
		for name, parts := range topics {
			resp, cerr := adm.CreateTopic(ctx, parts, replication, map[string]*string{
				"min.insync.replicas": ptr("1"),
			}, name)
			if cerr == nil {
				cerr = resp.Err
			}
			if cerr != nil && !errors.Is(cerr, kerrTopicExists) && !isTopicExists(cerr) {
				err = fmt.Errorf("create topic %s: %w", name, cerr)
				break
			}
		}
		if err == nil || attempt >= 30 {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

// ---------------------------------------------------------------- producer

// Producer is a thin, idempotent producer.
type Producer struct {
	cl  *kgo.Client
	log *zap.Logger
}

// NewProducer builds an idempotent producer (franz-go enables idempotence by default with acks=all).
func NewProducer(brokers []string, clientID string, log *zap.Logger) (*Producer, error) {
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ClientID(clientID),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.ProducerLinger(2*time.Millisecond),
		kgo.ProducerBatchCompression(kgo.Lz4Compression(), kgo.NoCompression()),
		kgo.RecordPartitioner(kgo.StickyKeyPartitioner(nil)), // murmur2 by key, Java compatible
		kgo.ProduceRequestTimeout(10*time.Second),
	)
	if err != nil {
		return nil, err
	}
	return &Producer{cl: cl, log: log}, nil
}

// Record builds a record carrying the envelope.
func Record(topic, key string, env events.Envelope) (*kgo.Record, error) {
	v, err := json.Marshal(env)
	if err != nil {
		return nil, err
	}
	return &kgo.Record{
		Topic: topic,
		Key:   []byte(key),
		Value: v,
		Headers: []kgo.RecordHeader{
			{Key: "event_id", Value: []byte(env.ID)},
			{Key: "event_type", Value: []byte(env.Type)},
		},
	}, nil
}

// ProduceSync produces records and waits until all are acknowledged (used by the outbox).
func (p *Producer) ProduceSync(ctx context.Context, recs ...*kgo.Record) error {
	return p.cl.ProduceSync(ctx, recs...).FirstErr()
}

// PublishAsync fires a best-effort message (market streams). Errors are only logged.
func (p *Producer) PublishAsync(ctx context.Context, topic, key string, env events.Envelope) {
	rec, err := Record(topic, key, env)
	if err != nil {
		p.log.Error("encode record", zap.Error(err))
		return
	}
	p.cl.Produce(ctx, rec, func(_ *kgo.Record, err error) {
		if err != nil {
			p.log.Warn("async publish failed", zap.String("topic", topic), zap.Error(err))
		}
	})
}

// Close flushes and closes the producer.
func (p *Producer) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = p.cl.Flush(ctx)
	p.cl.Close()
}

// ---------------------------------------------------------------- group consumer

// BatchHandler handles a batch of messages from ONE topic-partition, in offset order.
// Return nil only when the whole batch is durably processed; the offsets are then committed.
type BatchHandler func(ctx context.Context, msgs []Message) error

// GroupConsumerConfig configures a GroupConsumer.
type GroupConsumerConfig struct {
	Brokers     []string
	Group       string
	Topics      []string
	MaxAttempts int // before a batch is sent to the DLQ (default 10)
	// OnAssigned / OnRevoked let stateful consumers (matching engine) load/drop partition state.
	OnAssigned func(ctx context.Context, assigned map[string][]int32)
	OnRevoked  func(ctx context.Context, revoked map[string][]int32)
}

// GroupConsumer is an at-least-once consumer. Pair it with inbox dedup for effectively-once.
type GroupConsumer struct {
	cfg     GroupConsumerConfig
	cl      *kgo.Client
	dlq     *kgo.Client
	handler BatchHandler
	log     *zap.Logger
}

// NewGroupConsumer creates the consumer; call Run to start.
func NewGroupConsumer(cfg GroupConsumerConfig, handler BatchHandler, log *zap.Logger) (*GroupConsumer, error) {
	if cfg.MaxAttempts == 0 {
		cfg.MaxAttempts = 10
	}
	opts := []kgo.Opt{
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ConsumerGroup(cfg.Group),
		kgo.ConsumeTopics(cfg.Topics...),
		kgo.DisableAutoCommit(),
		kgo.BlockRebalanceOnPoll(),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.Balancers(kgo.CooperativeStickyBalancer()),
		kgo.FetchMaxWait(100 * time.Millisecond),
	}
	if cfg.OnAssigned != nil {
		opts = append(opts, kgo.OnPartitionsAssigned(func(ctx context.Context, _ *kgo.Client, m map[string][]int32) {
			cfg.OnAssigned(ctx, m)
		}))
	}
	if cfg.OnRevoked != nil {
		cb := func(ctx context.Context, _ *kgo.Client, m map[string][]int32) { cfg.OnRevoked(ctx, m) }
		opts = append(opts, kgo.OnPartitionsRevoked(cb), kgo.OnPartitionsLost(cb))
	}
	cl, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, err
	}
	dlq, err := kgo.NewClient(kgo.SeedBrokers(cfg.Brokers...), kgo.AllowAutoTopicCreation())
	if err != nil {
		cl.Close()
		return nil, err
	}
	return &GroupConsumer{cfg: cfg, cl: cl, dlq: dlq, handler: handler,
		log: log.With(zap.String("group", cfg.Group))}, nil
}

// Run polls until ctx is cancelled.
func (c *GroupConsumer) Run(ctx context.Context) error {
	defer c.dlq.Close()
	defer c.cl.Close()
	c.log.Info("consumer started", zap.Strings("topics", c.cfg.Topics))
	for {
		fetches := c.cl.PollRecords(ctx, 1000)
		if ctx.Err() != nil {
			c.cl.AllowRebalance()
			return nil
		}
		fetches.EachError(func(t string, p int32, err error) {
			c.log.Warn("fetch error", zap.String("topic", t), zap.Int32("partition", p), zap.Error(err))
		})
		fetches.EachPartition(func(ftp kgo.FetchTopicPartition) {
			if len(ftp.Records) == 0 || ctx.Err() != nil {
				return
			}
			c.processPartition(ctx, ftp.Records)
		})
		c.cl.AllowRebalance()
	}
}

func (c *GroupConsumer) processPartition(ctx context.Context, recs []*kgo.Record) {
	msgs := make([]Message, 0, len(recs))
	for _, r := range recs {
		var env events.Envelope
		if err := json.Unmarshal(r.Value, &env); err != nil || env.ID == "" {
			c.log.Error("undecodable record -> DLQ", zap.String("topic", r.Topic), zap.Int64("offset", r.Offset))
			c.toDLQ(ctx, []*kgo.Record{r}, fmt.Errorf("decode: %v", err))
			continue
		}
		msgs = append(msgs, Message{Topic: r.Topic, Partition: r.Partition, Offset: r.Offset, Key: r.Key, Envelope: env})
	}
	backoff := 100 * time.Millisecond
	for attempt := 1; len(msgs) > 0; attempt++ {
		err := c.handler(ctx, msgs)
		if err == nil {
			break
		}
		if ctx.Err() != nil {
			return // not committed -> redelivered after restart
		}
		c.log.Error("handler failed", zap.Int("attempt", attempt), zap.Int("batch", len(msgs)), zap.Error(err))
		if attempt >= c.cfg.MaxAttempts {
			// Retry one by one so a single poison message does not block the partition.
			for _, m := range msgs {
				if herr := c.handler(ctx, []Message{m}); herr != nil {
					c.toDLQ(ctx, findRecords(recs, m.Offset), herr)
				}
			}
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 5*time.Second {
			backoff *= 2
		}
	}
	if err := c.cl.CommitRecords(ctx, recs...); err != nil {
		c.log.Warn("commit failed (will redeliver, inbox dedups)", zap.Error(err))
	}
}

func (c *GroupConsumer) toDLQ(ctx context.Context, recs []*kgo.Record, cause error) {
	for _, r := range recs {
		d := &kgo.Record{Topic: r.Topic + ".dlq", Key: r.Key, Value: r.Value,
			Headers: append(r.Headers, kgo.RecordHeader{Key: "dlq_error", Value: []byte(cause.Error())},
				kgo.RecordHeader{Key: "dlq_group", Value: []byte(c.cfg.Group)})}
		if err := c.dlq.ProduceSync(ctx, d).FirstErr(); err != nil {
			c.log.Error("DLQ produce failed", zap.Error(err))
		}
	}
}

func findRecords(recs []*kgo.Record, offset int64) []*kgo.Record {
	for _, r := range recs {
		if r.Offset == offset {
			return []*kgo.Record{r}
		}
	}
	return nil
}

// ---------------------------------------------------------------- tail consumer

// TailConsumer reads topics from the latest offset without a group: every instance receives every
// message. Used for fan-out (websocket gateway, read caches).
type TailConsumer struct {
	cl      *kgo.Client
	handler func(ctx context.Context, m Message)
	log     *zap.Logger
}

// NewTailConsumer creates a broadcast consumer.
func NewTailConsumer(brokers, topics []string, handler func(ctx context.Context, m Message), log *zap.Logger) (*TailConsumer, error) {
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumeTopics(topics...),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()),
		kgo.FetchMaxWait(50*time.Millisecond),
	)
	if err != nil {
		return nil, err
	}
	return &TailConsumer{cl: cl, handler: handler, log: log}, nil
}

// Run polls until ctx is cancelled.
func (t *TailConsumer) Run(ctx context.Context) error {
	defer t.cl.Close()
	for {
		fetches := t.cl.PollFetches(ctx)
		if ctx.Err() != nil {
			return nil
		}
		fetches.EachRecord(func(r *kgo.Record) {
			var env events.Envelope
			if err := json.Unmarshal(r.Value, &env); err != nil {
				return
			}
			t.handler(ctx, Message{Topic: r.Topic, Partition: r.Partition, Offset: r.Offset, Key: r.Key, Envelope: env})
		})
	}
}

func ptr[T any](v T) *T { return &v }
