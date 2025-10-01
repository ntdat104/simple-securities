package sqlite

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

type SQLiteClient struct {
	DB *sqlx.DB
}

func NewSQLiteClient() (*SQLiteClient, error) {
	dsn := ":memory:"

	db, err := sqlx.Connect("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite connection: %w", err)
	}

	if err = db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping SQLite database: %w", err)
	}

	return &SQLiteClient{DB: db}, nil
}

func (c *SQLiteClient) GetDB(ctx context.Context) *sqlx.DB {
	return c.DB
}

func (c *SQLiteClient) SetDB(db *sqlx.DB) {
	c.DB = db
}

func (c *SQLiteClient) Close(ctx context.Context) error {
	if c.DB != nil {
		if err := c.DB.Close(); err != nil {
			return fmt.Errorf("failed to close SQLite connection: %w", err)
		}
	}
	return nil
}
