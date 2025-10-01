package postgre

import (
	"context"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL driver for sqlx
	"github.com/jmoiron/sqlx"
)

type PostgreSQLConfig struct {
	User     string
	Password string
	Host     string
	Port     int
	Database string
	SSLMode  string // e.g. "disable", "require"

	MaxIdleConns int
	MaxOpenConns int
	MaxLifeTime  time.Duration
	MaxIdleTime  time.Duration
}

type PostgreSQLClient struct {
	DB *sqlx.DB
}

func NewPostgreSQLClient(conf PostgreSQLConfig) (*PostgreSQLClient, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		conf.User,
		conf.Password,
		conf.Host,
		conf.Port,
		conf.Database,
		conf.SSLMode,
	)

	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open PostgreSQL connection: %w", err)
	}

	db.SetMaxIdleConns(conf.MaxIdleConns)
	db.SetMaxOpenConns(conf.MaxOpenConns)
	db.SetConnMaxLifetime(conf.MaxLifeTime)
	db.SetConnMaxIdleTime(conf.MaxIdleTime)

	if err = db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping PostgreSQL database: %w", err)
	}

	return &PostgreSQLClient{DB: db}, nil
}

func (c *PostgreSQLClient) GetDB(ctx context.Context) *sqlx.DB {
	return c.DB
}

func (c *PostgreSQLClient) SetDB(db *sqlx.DB) {
	c.DB = db
}

func (c *PostgreSQLClient) Close(ctx context.Context) error {
	if c.DB != nil {
		if err := c.DB.Close(); err != nil {
			return fmt.Errorf("failed to close PostgreSQL connection: %w", err)
		}
	}
	return nil
}
