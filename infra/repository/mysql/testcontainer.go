package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"simple-securities/config"
)

const (
	// MySQLStartTimeout defines the timeout for starting the MySQL container
	MySQLStartTimeout = 2 * time.Minute
)

// SetupMySQLContainer creates a MySQL container for testing.
// It uses a testcontainers-go library to spin up a MySQL Docker container.
func SetupMySQLContainer(t *testing.T) *config.MySQLConfig {
	t.Helper()

	ctx := context.Background()

	// Create a temporary SQL file with init script
	tempFile, err := os.CreateTemp("", "mysql-init-*.sql")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Write SQL schema directly. This script will be executed when the container starts.
	initSQL := "CREATE TABLE IF NOT EXISTS `example` (\n" +
		"    `id` INT(11) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'Primary key ID',\n" +
		"    `name` VARCHAR(255) NOT NULL COMMENT 'Name',\n" +
		"    `alias` VARCHAR(255) DEFAULT NULL COMMENT 'Alias',\n" +
		"    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation time',\n" +
		"    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Update time',\n" +
		"    `deleted_at` TIMESTAMP NULL DEFAULT NULL COMMENT 'Deletion time',\n" +
		"    PRIMARY KEY (`id`),\n" +
		"    KEY `idx_name` (`name`),\n" +
		"    KEY `idx_deleted_at` (`deleted_at`)\n" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Example table for Clean Architecture';"

	if _, err := tempFile.WriteString(initSQL); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	// Define MySQL port
	mysqlPort := "3306/tcp"

	// Get the absolute path to the init SQL script
	initScriptPath, err := filepath.Abs(tempFile.Name())
	if err != nil {
		t.Fatalf("Failed to get absolute path to init script: %v", err)
	}

	// MySQL container configuration
	containerReq := testcontainers.ContainerRequest{
		Image:        "mysql:8.0",
		ExposedPorts: []string{mysqlPort},
		Env: map[string]string{
			"MYSQL_ROOT_PASSWORD": "mysqlroot",
			"MYSQL_DATABASE":      "go_clean_architecture",
			"MYSQL_USER":          "user",
			"MYSQL_PASSWORD":      "mysqlroot",
		},
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      initScriptPath,
				ContainerFilePath: "/docker-entrypoint-initdb.d/init.sql",
				FileMode:          0644,
			},
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("3306/tcp"),
			wait.ForLog("port: 3306  MySQL Community Server - GPL"),
		).WithStartupTimeout(MySQLStartTimeout),
	}

	// Start MySQL container
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: containerReq,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to start MySQL container: %v", err)
	}

	// Add cleanup function to terminate container after test
	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Fatalf("Failed to terminate MySQL container: %v", err)
		}
	})

	// Get host and port
	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get MySQL container host: %v", err)
	}

	port, err := container.MappedPort(ctx, nat.Port(mysqlPort))
	if err != nil {
		t.Fatalf("Failed to get MySQL container port: %v", err)
	}

	// Create config using config.MySQLConfig
	mySQLConfig := &config.MySQLConfig{
		User:         "root",
		Password:     "mysqlroot",
		Host:         host,
		Port:         port.Int(),
		Database:     "go_clean_architecture",
		MaxIdleConns: 10,
		MaxOpenConns: 100,
		MaxLifeTime:  "1h",
		MaxIdleTime:  "30m",
		CharSet:      "utf8mb4",
		ParseTime:    true,
		TimeZone:     "UTC",
	}

	// Wait a bit for initialization to complete
	time.Sleep(2 * time.Second)

	return mySQLConfig
}

// GetTestDB returns a test MySQL client. It connects to the container using the provided config.
func GetTestDB(t *testing.T, config *config.MySQLConfig) *MySQLClient {
	t.Helper()

	// Create DSN
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%t&loc=%s",
		config.User,
		config.Password,
		config.Host,
		config.Port,
		config.Database,
		config.CharSet,
		config.ParseTime,
		config.TimeZone,
	)

	// Use the NewMySQLClient from the other file, which now uses sqlx
	client, err := NewMySQLClient(dsn)
	if err != nil {
		t.Fatalf("Failed to create MySQL client: %v", err)
	}

	return client
}

// ExampleTable represents the example table for testing.
// The GORM-specific tags have been removed since sqlx maps fields by name by default.
type ExampleTable struct {
	ID        uint           `db:"id"`
	Name      string         `db:"name"`
	Alias     sql.NullString `db:"alias"`
	CreatedAt time.Time      `db:"created_at"`
	UpdatedAt time.Time      `db:"updated_at"`
	DeletedAt sql.NullTime   `db:"deleted_at"`
}

// MockMySQLData executes SQL statements in the test database using sqlx.DB.Exec.
func MockMySQLData(t *testing.T, client *MySQLClient, sqls []string) {
	t.Helper()

	// Execute all SQL statements
	for _, sql := range sqls {
		// Use sqlx's ExecContext which is a direct replacement for gorm's Exec for raw SQL
		_, err := client.DB.ExecContext(context.Background(), sql)
		if err != nil {
			t.Fatalf("Unable to insert data: %v", err)
		}
	}
}
