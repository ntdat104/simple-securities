package repository

import (
	"fmt"
	"io/ioutil"
	"log"

	"github.com/go-redis/redis/v8"
	"github.com/jmoiron/sqlx"
)

type RepositoryOption func(*Client)

// Clients holds all the database and client connections.
type Client struct {
	MySQL      *sqlx.DB
	PostgreSQL *sqlx.DB
	SQLite     *sqlx.DB
	Redis      *redis.Client
}

// InitClients initializes and returns a Client struct with all database connections.
func InitializeRepositories(opts ...RepositoryOption) *Client {
	client := &Client{}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

// WithMySQLite returns an option to initialize SQLite
func WithMySQLite() RepositoryOption {
	return func(c *Client) {
		if c.SQLite == nil {
			sqlite, err := NewSqliteConn()
			if err != nil {
				panic("Failed to initialize SQLite: " + err.Error())
			}
			c.SQLite = sqlite
		}
	}
}

// WithMySQL returns an option to initialize MySQL
func WithMySQL() RepositoryOption {
	return func(c *Client) {
		if c.MySQL == nil {
			mysql, err := NewMySQLConn()
			if err != nil {
				panic("Failed to initialize MySQL: " + err.Error())
			}
			if err := RunMigration(mysql, "./schema/mysql.sql"); err != nil {
				log.Fatalf("Migration failed: %v", err)
			}
			c.MySQL = mysql
		}
	}
}

// WithMyPostgreSQL returns an option to initialize PostgreSQL
func WithMyPostgreSQL() RepositoryOption {
	return func(c *Client) {
		if c.PostgreSQL == nil {
			postgre, err := NewPostgreConn()
			if err != nil {
				panic("Failed to initialize PostgreSQL: " + err.Error())
			}
			c.PostgreSQL = postgre
		}
	}
}

// WithRedis returns an option to initialize Redis
func WithRedis() RepositoryOption {
	return func(c *Client) {
		if c.Redis == nil {
			redis, err := NewRedisConn()
			if err != nil {
				panic("Failed to initialize Redis: " + err.Error())
			}
			c.Redis = redis
		}
	}
}

func RunMigration(db *sqlx.DB, schemaFile string) error {
	sqlBytes, err := ioutil.ReadFile(schemaFile)
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}

	// Execute the entire SQL file
	_, err = db.Exec(string(sqlBytes))
	if err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}
	return nil
}
