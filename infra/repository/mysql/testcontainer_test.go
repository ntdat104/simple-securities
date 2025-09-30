package mysql

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetupMySQLContainer(t *testing.T) {
	// Skip this test in CI environments or when running quick tests
	if testing.Short() {
		t.Skip("Skipping MySQL container test in short mode")
	}

	// Create MySQL container
	config := SetupMySQLContainer(t)

	// Validate configuration
	assert.NotEmpty(t, config.Host, "Host should not be empty")
	assert.NotZero(t, config.Port, "Port should be greater than 0")
	assert.Equal(t, "root", config.User)
	assert.Equal(t, "mysqlroot", config.Password)
	assert.Equal(t, "go_clean_architecture", config.Database)
	assert.Equal(t, "utf8mb4", config.CharSet)
	assert.Equal(t, true, config.ParseTime)
	assert.Equal(t, "UTC", config.TimeZone)

	// Validate additional config fields
	assert.Equal(t, 10, config.MaxIdleConns)
	assert.Equal(t, 100, config.MaxOpenConns)
	assert.Equal(t, "1h", config.MaxLifeTime)
	assert.Equal(t, "30m", config.MaxIdleTime)

	// Get database connection
	db := GetTestDB(t, config)
	defer db.Close(context.Background())

	// Verify connection by executing a simple query
	var result int
	// Replaced gorm's Raw/Scan with sqlx.Get
	err := db.DB.Get(&result, "SELECT 1")
	assert.NoError(t, err, "Should be able to execute a simple query")
	assert.Equal(t, 1, result, "Query result should be 1")

	// Test creating a table
	// This Exec call is compatible with both GORM and sqlx, so no change is needed.
	_, err = db.DB.Exec("CREATE TABLE IF NOT EXISTS test_table (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(255) NOT NULL)")
	assert.NoError(t, err, "Should be able to create a table")

	// Test MockMySQLData function
	mockSQLs := []string{
		"INSERT INTO test_table (name) VALUES ('test1')",
		"INSERT INTO test_table (name) VALUES ('test2')",
	}
	MockMySQLData(t, db, mockSQLs)

	// Verify data was inserted
	var count int
	// Replaced gorm's Table/Count with sqlx.Get
	err = db.DB.Get(&count, "SELECT count(*) FROM test_table")
	assert.NoError(t, err, "Should be able to count rows")
	assert.Equal(t, 2, count, "There should be 2 rows in the table")

	// Verify specific data
	type TestRow struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}

	var rows []TestRow
	// Replaced gorm's Table/Find with sqlx.Select
	err = db.DB.Select(&rows, "SELECT * FROM test_table")
	assert.NoError(t, err, "Should be able to query rows")
	assert.Len(t, rows, 2, "There should be 2 rows")
	assert.Equal(t, "test1", rows[0].Name, "First row should have name 'test1'")
	assert.Equal(t, "test2", rows[1].Name, "Second row should have name 'test2'")
}
