package sqlite

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// This test file demonstrates how to use the test helpers from the `sqlite` package.
func TestSQLiteClientAndHelpers(t *testing.T) {
	// Set up the in-memory database using the helper function.
	client := SetupTestDB(t)
	assert.NotNil(t, client.DB, "DB should not be nil")
	defer client.Close(context.Background())

	// Verify connection by executing a simple query
	var result int
	// Use sqlx's Get method to query a single value.
	err := client.DB.Get(&result, "SELECT 1")
	assert.NoError(t, err, "Should be able to execute a simple query")
	assert.Equal(t, 1, result, "Query result should be 1")

	// Test creating a table
	// This Exec call is compatible with both GORM and sqlx, so no change is needed.
	_, err = client.DB.Exec(`
		CREATE TABLE IF NOT EXISTS test_table (
			id INTEGER PRIMARY KEY,
			name VARCHAR(255) NOT NULL
		)
	`)
	assert.NoError(t, err, "Should be able to create a table")

	// Test MockSQLiteData function
	mockSQLs := []string{
		"INSERT INTO test_table (name) VALUES ('test1')",
		"INSERT INTO test_table (name) VALUES ('test2')",
	}

	MockSQLiteData(t, client, mockSQLs)

	// Verify data was inserted
	var count int
	// Use sqlx's Get method to get the count of rows.
	err = client.DB.Get(&count, "SELECT count(*) FROM test_table")
	assert.NoError(t, err, "Should be able to count rows")
	assert.Equal(t, 2, count, "There should be 2 rows in the table")

	// Verify specific data
	type TestRow struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}

	var rows []TestRow
	// Use sqlx's Select method to query multiple rows and map them to a slice of structs.
	err = client.DB.Select(&rows, "SELECT * FROM test_table")
	assert.NoError(t, err, "Should be able to query rows")
	assert.Len(t, rows, 2, "There should be 2 rows")
	assert.Equal(t, "test1", rows[0].Name, "First row should have name 'test1'")
	assert.Equal(t, "test2", rows[1].Name, "Second row should have name 'test2'")
}
