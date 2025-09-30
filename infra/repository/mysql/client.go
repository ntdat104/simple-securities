package mysql

import (
	"context"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql" // Import the MySQL driver
	"github.com/jmoiron/sqlx"

	"simple-securities/infra/repository"
)

// MySQLClient represents a MySQL database client using sqlx
type MySQLClient struct {
	DB *sqlx.DB
}

// NewMySQLClient creates a new MySQL client by opening a connection and configuring the pool.
// It uses sqlx.Connect to establish a database connection and handles connection pool settings.
func NewMySQLClient(dsn string) (*MySQLClient, error) {
	if dsn == "" {
		return nil, repository.ErrMissingMySQLConfig
	}

	// Use sqlx.Connect to open a new database connection.
	// The driver name "mysql" is required for sqlx.
	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open MySQL connection: %w", err)
	}

	// Configure connection pool settings.
	// In a real application, these values should be loaded from a configuration file.
	db.SetMaxIdleConns(10)
	db.SetMaxOpenConns(100)
	db.SetConnMaxLifetime(time.Hour)
	db.SetConnMaxIdleTime(30 * time.Minute)

	// Ping the database to ensure the connection is live.
	if err = db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping MySQL database: %w", err)
	}

	return &MySQLClient{DB: db}, nil
}

// GetDB returns the sqlx database instance. The context is included for signature
// compatibility with the GORM version, but sqlx query methods take context directly.
func (c *MySQLClient) GetDB(ctx context.Context) *sqlx.DB {
	return c.DB
}

// SetDB sets the sqlx database instance.
func (c *MySQLClient) SetDB(db *sqlx.DB) {
	c.DB = db
}

// Close closes the MySQL database connection.
func (c *MySQLClient) Close(ctx context.Context) error {
	if c.DB != nil {
		if err := c.DB.Close(); err != nil {
			return fmt.Errorf("failed to close MySQL connection: %w", err)
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
