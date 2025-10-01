package mysql

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

type MySQLConfig struct {
	User      string
	Password  string
	Host      string
	Port      int
	Database  string
	CharSet   string
	ParseTime bool
	TimeZone  string

	MaxIdleConns int
	MaxOpenConns int
	MaxLifeTime  time.Duration
	MaxIdleTime  time.Duration
}

type MySQLClient struct {
	DB *sqlx.DB
}

func NewMySQLClient(conf MySQLConfig) (*MySQLClient, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%v&loc=%s",
		conf.User,
		conf.Password,
		conf.Host,
		conf.Port,
		conf.Database,
		conf.CharSet,
		conf.ParseTime,
		conf.TimeZone,
	)

	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open MySQL connection: %w", err)
	}

	db.SetMaxIdleConns(conf.MaxIdleConns)
	db.SetMaxOpenConns(conf.MaxOpenConns)
	db.SetConnMaxLifetime(conf.MaxLifeTime)
	db.SetConnMaxIdleTime(conf.MaxIdleTime)

	if err = db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping MySQL database: %w", err)
	}

	return &MySQLClient{DB: db}, nil
}

func (c *MySQLClient) GetDB(ctx context.Context) *sqlx.DB {
	return c.DB
}

func (c *MySQLClient) SetDB(db *sqlx.DB) {
	c.DB = db
}

func (c *MySQLClient) Close(ctx context.Context) error {
	if c.DB != nil {
		if err := c.DB.Close(); err != nil {
			return fmt.Errorf("failed to close MySQL connection: %w", err)
		}
	}
	return nil
}
