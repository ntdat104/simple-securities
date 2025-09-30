package postgre

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // Import the PostgreSQL driver for sqlx

	"simple-securities/infra/repository"
)

// PostgreSQLClient represents a PostgreSQL database client using sqlx
type PostgreSQLClient struct {
	DB *sqlx.DB
}

// NewPostgreSQLClient creates a new PostgreSQL client.
// It opens a connection using sqlx and configures the connection pool settings.
func NewPostgreSQLClient(dsn string) (*PostgreSQLClient, error) {
	if dsn == "" {
		return nil, repository.ErrMissingPostgreSQLConfig
	}

	// Use sqlx.Connect to establish a database connection.
	// The driver name "postgres" is required for the lib/pq driver.
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open PostgreSQL connection: %w", err)
	}

	// Ping the database to ensure the connection is live.
	if err = db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping PostgreSQL database: %w", err)
	}

	// Default connection pool settings. In a real application, these should be
	// configured from a config file.
	db.SetMaxIdleConns(10)
	db.SetMaxOpenConns(100)
	db.SetConnMaxLifetime(time.Hour)
	db.SetConnMaxIdleTime(30 * time.Minute)

	return &PostgreSQLClient{DB: db}, nil
}

// GetDB returns the sqlx database instance. The context is included for signature
// compatibility, but sqlx query methods take the context directly.
func (c *PostgreSQLClient) GetDB(ctx context.Context) *sqlx.DB {
	return c.DB
}

// SetDB sets the sqlx database instance.
func (c *PostgreSQLClient) SetDB(db *sqlx.DB) {
	c.DB = db
}

// Close closes the PostgreSQL database connection.
func (c *PostgreSQLClient) Close(ctx context.Context) error {
	if c.DB != nil {
		if err := c.DB.Close(); err != nil {
			return fmt.Errorf("failed to close PostgreSQL connection: %w", err)
		}
	}
	return nil
}

// ConfigureConnectionPool configures the underlying sql.DB connection pool.
// This function directly manipulates the pool settings.
func ConfigureConnectionPool(db *sqlx.DB, maxIdleConns, maxOpenConns int, maxLifetime, maxIdleTime time.Duration) error {
	sqlDB := db.DB
	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetConnMaxLifetime(maxLifetime)
	sqlDB.SetConnMaxIdleTime(maxIdleTime)

	return nil
}
