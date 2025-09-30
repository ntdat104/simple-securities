package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3" // Import the SQLite driver for sqlx
)

// ExampleTable represents a sample table for testing purposes.
// The `db` tags are used by sqlx to map database columns to struct fields.
type ExampleTable struct {
	ID        uint           `db:"id"`
	Name      string         `db:"name"`
	Alias     sql.NullString `db:"alias"`
	CreatedAt time.Time      `db:"created_at"`
	UpdatedAt time.Time      `db:"updated_at"`
	DeletedAt sql.NullTime   `db:"deleted_at"`
}

// TableName returns the table name.
// This function is included for clarity but is not used by sqlx for queries.
func (ExampleTable) TableName() string {
	return "example"
}

// SetupTestDB creates a new in-memory SQLite database for testing.
// It returns a configured SQLiteClient and handles cleanup when the test finishes.
func SetupTestDB(t *testing.T) *SQLiteClient {
	t.Helper()

	// The DSN "file::memory:?cache=shared" creates a unique in-memory database
	// that is safe for concurrent test runs.
	client, err := NewSQLiteClient("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("Failed to create SQLite client: %v", err)
	}

	// Create the example table schema directly with a raw SQL statement.
	// sqlx does not have an AutoMigrate feature, so we define the schema explicitly.
	schema := `
	CREATE TABLE IF NOT EXISTS example (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name VARCHAR(255) NOT NULL,
		alias VARCHAR(255),
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		deleted_at DATETIME
	);`

	if _, err := client.DB.Exec(schema); err != nil {
		t.Fatalf("Failed to create example table: %v", err)
	}

	// Add a cleanup function to close the database connection after the test.
	t.Cleanup(func() {
		if err := client.Close(context.Background()); err != nil {
			t.Fatalf("Failed to close SQLite client: %v", err)
		}
	})

	return client
}

// MockSQLiteData executes SQL statements directly on the test database using sqlx.
func MockSQLiteData(t *testing.T, client *SQLiteClient, sqls []string) {
	t.Helper()

	for _, sql := range sqls {
		// Use sqlx's ExecContext for executing raw SQL statements.
		_, err := client.DB.ExecContext(context.Background(), sql)
		if err != nil {
			t.Fatalf("Unable to insert data: %v", err)
		}
	}
}
