package repository

import (
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	_ "github.com/go-sql-driver/mysql" // MySQL driver
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"           // PostgreSQL driver
	_ "github.com/mattn/go-sqlite3" // SQLite driver

	"simple-securities/config"
)

// NewSqliteConn creates a new SQLite database connection based on the SQLite configuration.
func NewSqliteConn() (*sqlx.DB, error) {
	// Use sqlx.Connect to open a new database connection
	db, err := sqlx.Connect("sqlite3", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite connection: %w", err)
	}

	return db, nil
}

// NewMySQLConn creates a new MySQL database connection based on the MySQL configuration
func NewMySQLConn() (*sqlx.DB, error) {
	if config.GlobalConfig.MySQL == nil {
		return nil, ErrMissingMySQLConfig
	}

	// Construct DSN from configuration
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%v&loc=%s",
		config.GlobalConfig.MySQL.User,
		config.GlobalConfig.MySQL.Password,
		config.GlobalConfig.MySQL.Host,
		config.GlobalConfig.MySQL.Port,
		config.GlobalConfig.MySQL.Database,
		config.GlobalConfig.MySQL.CharSet,
		config.GlobalConfig.MySQL.ParseTime,
		config.GlobalConfig.MySQL.TimeZone,
	)

	// Open database connection with sqlx
	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open MySQL connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxIdleConns(config.GlobalConfig.MySQL.MaxIdleConns)
	db.SetMaxOpenConns(config.GlobalConfig.MySQL.MaxOpenConns)
	db.SetConnMaxLifetime(config.GetDuration(config.GlobalConfig.MySQL.MaxLifeTime))
	db.SetConnMaxIdleTime(config.GetDuration(config.GlobalConfig.MySQL.MaxIdleTime))

	return db, nil
}

// NewRedisConn creates a new Redis client connection based on the Redis configuration
func NewRedisConn() (*redis.Client, error) {
	if config.GlobalConfig.Redis == nil {
		return nil, ErrMissingRedisConfig
	}

	// Create Redis client from configuration
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", config.GlobalConfig.Redis.Host, config.GlobalConfig.Redis.Port),
		Password:     config.GlobalConfig.Redis.Password,
		DB:           config.GlobalConfig.Redis.DB,
		PoolSize:     config.GlobalConfig.Redis.PoolSize,
		MinIdleConns: config.GlobalConfig.Redis.MinIdleConns,
		IdleTimeout:  time.Duration(config.GlobalConfig.Redis.IdleTimeout) * time.Second,
	})

	return client, nil
}

// NewPostgreConn creates a new PostgreSQL connection pool based on the PostgreSQL configuration
func NewPostgreConn() (*sqlx.DB, error) {
	if config.GlobalConfig.Postgre == nil {
		return nil, ErrMissingPostgreSQLConfig
	}

	// Construct DSN from configuration
	connString := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		config.GlobalConfig.Postgre.User,
		config.GlobalConfig.Postgre.Password,
		config.GlobalConfig.Postgre.Host,
		config.GlobalConfig.Postgre.Port,
		config.GlobalConfig.Postgre.Database,
		config.GlobalConfig.Postgre.SSLMode,
	)

	// Open database connection with sqlx
	db, err := sqlx.Connect("postgres", connString)
	if err != nil {
		return nil, fmt.Errorf("failed to open PostgreSQL connection: %w", err)
	}

	// Configure connection pool
	if config.GlobalConfig.Postgre.MaxConnections > 0 {
		db.SetMaxOpenConns(int(config.GlobalConfig.Postgre.MaxConnections))
	}
	if config.GlobalConfig.Postgre.MinConnections > 0 {
		db.SetMaxIdleConns(int(config.GlobalConfig.Postgre.MinConnections))
	}
	if config.GlobalConfig.Postgre.MaxConnLifetime > 0 {
		db.SetConnMaxLifetime(time.Duration(config.GlobalConfig.Postgre.MaxConnLifetime) * time.Second)
	}
	if config.GlobalConfig.Postgre.IdleTimeout > 0 {
		db.SetConnMaxIdleTime(time.Duration(config.GlobalConfig.Postgre.IdleTimeout) * time.Second)
	}

	return db, nil
}
