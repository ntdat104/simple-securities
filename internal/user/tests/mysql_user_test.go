package tests

import (
	"simple-securities/config"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockMySQLData(t *testing.T) {
	testCases := []struct {
		Name    string
		sqlData []string
	}{
		{
			Name: "insert and select user",
			sqlData: []string{
				`INSERT INTO users 
					(uuid, email, hashed_password, status, created_by, updated_by)
				 VALUES 
					('abcdefghijklmnopqrstuvwxyz12', 'testing@gmail.com', 'hashedpwd', 'ACTIVE', 1, 1);`,
			},
		},
	}

	// 1️⃣ Start MySQL test container
	mysqlDBConf := SetupMySQL(t)
	config.GlobalConfig.MySQL = mysqlDBConf
	config.GlobalConfig.MigrationDir = "./migrations"

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			// 2️⃣ Prepare database & insert mock data
			db := MockMySQLData(t, config.GlobalConfig, tc.sqlData)
			defer db.Close()

			// 3️⃣ Query to verify
			type UserVO struct {
				ID    uint64 `db:"id"`
				UUID  string `db:"uuid"`
				Email string `db:"email"`
			}

			var user UserVO
			err := db.Get(&user, "SELECT id, uuid, email FROM users WHERE email = ?", "testing@gmail.com")
			if err != nil {
				t.Fatalf("query failed: %v", err)
			}

			// 4️⃣ Assertions
			assert.Equal(t, "abcdefghijklmnopqrstuvwxyz12", user.UUID)
			assert.Equal(t, "testing@gmail.com", user.Email)
			assert.NotZero(t, user.ID)
		})
	}
}
