// Package database opens the GORM connection pool, runs schema migrations (GORM AutoMigrate) and
// provides transaction helpers shared by every service.
package database

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"simple-securities/pkg/config"
)

// Config tunes the pool.
type Config struct {
	URL          string
	MaxOpenConns int           // default 20
	MaxIdleConns int           // default MaxOpenConns/2
	SlowQuery    time.Duration // logged as warning, default 200ms
	LogLevel     string        // gorm log level: silent | error | warn | info (default warn)
}

// Connect opens a GORM DB and verifies connectivity with retries (containers start out of order).
func Connect(ctx context.Context, cfg Config, log *zap.Logger) (*gorm.DB, error) {
	if cfg.MaxOpenConns == 0 {
		cfg.MaxOpenConns = 20
	}
	if cfg.MaxIdleConns == 0 {
		cfg.MaxIdleConns = max(cfg.MaxOpenConns/2, 2)
	}
	if cfg.SlowQuery == 0 {
		cfg.SlowQuery = 200 * time.Millisecond
	}
	gcfg := &gorm.Config{
		Logger:                 NewLogger(log, cfg.SlowQuery, cfg.LogLevel),
		SkipDefaultTransaction: true, // we open transactions explicitly
		// No GORM PrepareStmt: pgx already caches prepared statements per connection (bounded LRU).
		TranslateError: true, // unique violation -> gorm.ErrDuplicatedKey
		NowFunc:        func() time.Time { return time.Now().UTC() },
	}
	var (
		db  *gorm.DB
		err error
	)
	autoCreate := config.Bool("DB_AUTO_CREATE", false)
	for attempt := 1; ; attempt++ {
		db, err = gorm.Open(postgres.Open(cfg.URL), gcfg)
		if err != nil && autoCreate && isMissingDatabase(err) {
			if cerr := createDatabase(ctx, cfg.URL, log); cerr != nil {
				log.Warn("create database failed", zap.Error(cerr))
			}
			continue
		}
		if err == nil {
			sqlDB, derr := db.DB()
			if derr == nil {
				sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
				sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
				sqlDB.SetConnMaxIdleTime(5 * time.Minute)
				sqlDB.SetConnMaxLifetime(time.Hour)
				if err = sqlDB.PingContext(ctx); err == nil {
					return db, nil
				}
				_ = sqlDB.Close()
			} else {
				err = derr
			}
		}
		if attempt >= 30 {
			return nil, fmt.Errorf("connect postgres: %w", err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

// Migrate runs GORM AutoMigrate for the given models. A Postgres advisory lock serialises
// migrations so several replicas starting at once never race on DDL.
func Migrate(ctx context.Context, db *gorm.DB, name string, models ...any) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	key := LockKey("migrate:" + name)
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, key); err != nil {
		return fmt.Errorf("migration lock: %w", err)
	}
	defer func() { _, _ = conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, key) }()

	if err := db.WithContext(ctx).AutoMigrate(models...); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}
	return nil
}

// WithTx runs fn in a transaction (commit on nil error, rollback otherwise, panic-safe).
func WithTx(ctx context.Context, db *gorm.DB, fn func(tx *gorm.DB) error) error {
	return db.WithContext(ctx).Transaction(fn)
}

// Ping checks connectivity (readiness probe).
func Ping(ctx context.Context, db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// Close closes the pool.
func Close(db *gorm.DB) {
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
}

// isMissingDatabase reports SQLSTATE 3D000 (invalid_catalog_name).
func isMissingDatabase(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "3D000" || strings.Contains(err.Error(), "SQLSTATE 3D000")
}

// createDatabase creates the database named in url using the "postgres" maintenance database.
// Dev convenience (DB_AUTO_CREATE=true); production databases are provisioned by infrastructure.
func createDatabase(ctx context.Context, rawURL string, log *zap.Logger) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	name := strings.TrimPrefix(u.Path, "/")
	if name == "" || strings.ContainsAny(name, `"; `) {
		return fmt.Errorf("invalid database name %q", name)
	}
	u.Path = "/postgres"
	admin, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return err
	}
	defer Close(admin)
	err = admin.WithContext(ctx).Exec(`CREATE DATABASE "` + name + `"`).Error
	if err != nil && !strings.Contains(err.Error(), "42P04") { // duplicate_database: created concurrently
		return err
	}
	log.Info("database created", zap.String("database", name))
	return nil
}

// IsUniqueViolation reports a unique constraint violation.
func IsUniqueViolation(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// LockKey derives a stable advisory-lock key from a name.
func LockKey(name string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(name))
	return int64(h.Sum64() >> 1)
}

// NewLogger adapts GORM logging to zap.
func NewLogger(log *zap.Logger, slow time.Duration, level string) logger.Interface {
	lvl := logger.Warn
	switch level {
	case "silent":
		lvl = logger.Silent
	case "error":
		lvl = logger.Error
	case "info":
		lvl = logger.Info
	}
	return &zapLogger{log: log.Named("gorm").WithOptions(zap.AddCallerSkip(3)), slow: slow, level: lvl}
}

type zapLogger struct {
	log   *zap.Logger
	slow  time.Duration
	level logger.LogLevel
}

func (l *zapLogger) LogMode(level logger.LogLevel) logger.Interface {
	c := *l
	c.level = level
	return &c
}

func (l *zapLogger) Info(_ context.Context, msg string, args ...any) {
	if l.level >= logger.Info {
		l.log.Sugar().Infof(msg, args...)
	}
}

func (l *zapLogger) Warn(_ context.Context, msg string, args ...any) {
	if l.level >= logger.Warn {
		l.log.Sugar().Warnf(msg, args...)
	}
}

func (l *zapLogger) Error(_ context.Context, msg string, args ...any) {
	if l.level >= logger.Error {
		l.log.Sugar().Errorf(msg, args...)
	}
}

func (l *zapLogger) Trace(_ context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.level <= logger.Silent {
		return
	}
	elapsed := time.Since(begin)
	switch {
	case err != nil && l.level >= logger.Error && !errors.Is(err, gorm.ErrRecordNotFound) && !IsUniqueViolation(err):
		sql, rows := fc()
		l.log.Error("query failed", zap.Error(err), zap.Duration("elapsed", elapsed), zap.Int64("rows", rows), zap.String("sql", sql))
	case elapsed > l.slow && l.level >= logger.Warn:
		sql, rows := fc()
		l.log.Warn("slow query", zap.Duration("elapsed", elapsed), zap.Int64("rows", rows), zap.String("sql", sql))
	case l.level >= logger.Info:
		sql, rows := fc()
		l.log.Debug("query", zap.Duration("elapsed", elapsed), zap.Int64("rows", rows), zap.String("sql", sql))
	}
}
