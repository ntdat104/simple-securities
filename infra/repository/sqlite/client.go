package sqlite

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3" // Import the SQLite driver for sqlx

	"simple-securities/infra/repository"
)

// SQLiteClient represents a SQLite database client using sqlx
type SQLiteClient struct {
	DB *sqlx.DB
}

// NewSQLiteClient creates a new SQLite client.
// The DSN is the file path to the SQLite database.
func NewSQLiteClient(dsn string) (*SQLiteClient, error) {
	if dsn == "" {
		return nil, repository.ErrMissingSQLiteConfig
	}

	// Use sqlx.Connect to open a new database connection.
	// The driver name "sqlite3" is required for sqlx.
	db, err := sqlx.Connect("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite connection: %w", err)
	}

	// Ping the database to ensure the connection is live.
	if err = db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping SQLite database: %w", err)
	}

	return &SQLiteClient{DB: db}, nil
}

// GetDB returns the sqlx database instance.
func (c *SQLiteClient) GetDB(ctx context.Context) *sqlx.DB {
	return c.DB
}

// SetDB sets the sqlx database instance.
func (c *SQLiteClient) SetDB(db *sqlx.DB) {
	c.DB = db
}

// Close closes the SQLite database connection.
func (c *SQLiteClient) Close(ctx context.Context) error {
	if c.DB != nil {
		if err := c.DB.Close(); err != nil {
			return fmt.Errorf("failed to close SQLite connection: %w", err)
		}
	}
	return nil
}

// Since SQLite is a file-based database, connection pool settings are not applicable
// in the same way they are for client-server databases like MySQL or PostgreSQL.
// Therefore, the ConfigureConnectionPool function is not needed and has been removed.
