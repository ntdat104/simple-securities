package postgre

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
	// PostgreSQLStartTimeout defines the timeout for starting the PostgreSQL container
	PostgreSQLStartTimeout = 2 * time.Minute
)

// SetupPostgreSQLContainer creates and starts a PostgreSQL test container.
// It uses a testcontainers-go library to spin up a PostgreSQL Docker container.
func SetupPostgreSQLContainer(t *testing.T) *config.PostgreSQLConfig {
	t.Helper()

	ctx := context.Background()

	// Create a temporary SQL file with init script
	tempFile, err := os.CreateTemp("", "postgres-init-*.sql")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Write SQL schema directly - note PostgreSQL syntax differences from MySQL
	initSQL := "CREATE TABLE IF NOT EXISTS example (\n" +
		"    id SERIAL PRIMARY KEY,\n" +
		"    name VARCHAR(255) NOT NULL,\n" +
		"    alias VARCHAR(255),\n" +
		"    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,\n" +
		"    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,\n" +
		"    deleted_at TIMESTAMP\n" +
		");\n\n" +
		"CREATE INDEX idx_example_name ON example(name);\n" +
		"CREATE INDEX idx_example_deleted_at ON example(deleted_at);\n" +
		"COMMENT ON TABLE example IS 'Example table for Clean Architecture';\n" +
		"COMMENT ON COLUMN example.id IS 'Primary key ID';\n" +
		"COMMENT ON COLUMN example.name IS 'Name';\n" +
		"COMMENT ON COLUMN example.alias IS 'Alias';\n" +
		"COMMENT ON COLUMN example.created_at IS 'Creation time';\n" +
		"COMMENT ON COLUMN example.updated_at IS 'Update time';\n" +
		"COMMENT ON COLUMN example.deleted_at IS 'Deletion time';"

	if _, err := tempFile.WriteString(initSQL); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	// Define PostgreSQL port
	postgresPort := "5432/tcp"

	// Get the absolute path to the init SQL script
	initScriptPath, err := filepath.Abs(tempFile.Name())
	if err != nil {
		t.Fatalf("Failed to get absolute path to init script: %v", err)
	}

	// PostgreSQL container configuration
	containerReq := testcontainers.ContainerRequest{
		Image:        "postgres:13-alpine",
		ExposedPorts: []string{postgresPort},
		Env: map[string]string{
			"POSTGRES_USER":     "postgres",
			"POSTGRES_PASSWORD": "123456",
			"POSTGRES_DB":       "postgres",
		},
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      initScriptPath,
				ContainerFilePath: "/docker-entrypoint-initdb.d/init.sql",
				FileMode:          0644,
			},
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("5432/tcp"),
			wait.ForExec([]string{"pg_isready"}).
				WithPollInterval(1*time.Second).
				WithExitCodeMatcher(func(exitCode int) bool {
					return exitCode == 0
				}),
			wait.ForLog("database system is ready to accept connections"),
		).WithStartupTimeout(PostgreSQLStartTimeout),
	}

	// Start PostgreSQL container
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: containerReq,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to start PostgreSQL container: %v", err)
	}

	// Add cleanup function to terminate container after test
	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Fatalf("Failed to terminate PostgreSQL container: %v", err)
		}
	})

	// Get host and port
	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get PostgreSQL container host: %v", err)
	}

	port, err := container.MappedPort(ctx, nat.Port(postgresPort))
	if err != nil {
		t.Fatalf("Failed to get PostgreSQL container port: %v", err)
	}

	// Create config using config.PostgreSQLConfig
	postgresConfig := &config.PostgreSQLConfig{
		User:            "postgres",
		Password:        "123456",
		Host:            host,
		Port:            port.Int(),
		Database:        "postgres",
		SSLMode:         "disable",
		Options:         "",
		MaxConnections:  100,
		MinConnections:  10,
		MaxConnLifetime: 3600,
		IdleTimeout:     300,
		ConnectTimeout:  10,
		TimeZone:        "UTC",
	}

	// Wait a bit for initialization to complete
	time.Sleep(2 * time.Second)

	return postgresConfig
}

// GetTestDB creates an sqlx connection based on PostgreSQL configuration
func GetTestDB(t *testing.T, config *config.PostgreSQLConfig) *PostgreSQLClient {
	t.Helper()

	// Create DSN
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
		config.Host,
		config.Port,
		config.User,
		config.Password,
		config.Database,
		config.SSLMode,
		config.TimeZone,
	)

	// Use the NewPostgreSQLClient from the other file, which now uses sqlx
	client, err := NewPostgreSQLClient(dsn)
	if err != nil {
		t.Fatalf("Failed to create PostgreSQL client: %v", err)
	}

	return client
}

// ExampleTable represents the example table for testing
type ExampleTable struct {
	ID        int            `db:"id"`
	Name      string         `db:"name"`
	Alias     sql.NullString `db:"alias"`
	CreatedAt time.Time      `db:"created_at"`
	UpdatedAt time.Time      `db:"updated_at"`
	DeletedAt sql.NullTime   `db:"deleted_at"`
}

// MockPostgreSQLData executes SQL statements in the PostgreSQL database using sqlx.DB.Exec.
func MockPostgreSQLData(t *testing.T, client *PostgreSQLClient, sqls []string) {
	t.Helper()

	// Execute all SQL statements
	for _, sql := range sqls {
		_, err := client.DB.ExecContext(context.Background(), sql)
		if err != nil {
			t.Fatalf("Unable to insert data: %v", err)
		}
	}
}
