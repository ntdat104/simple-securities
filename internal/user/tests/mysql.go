package tests

import (
	"context"
	"fmt"
	"simple-securities/config"
	"simple-securities/internal/user/tests/migrations/migrate"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	_ "github.com/go-sql-driver/mysql"
)

const (
	MysqlStartTimeout = 2 * time.Minute
)

func SetupMySQL(t *testing.T) *config.MySQLConfig {
	t.Log("Setting up an instance of MySQL with testcontainers-go")
	ctx := context.Background()

	user, password, dbName := "user", "123456", "test"

	req := testcontainers.ContainerRequest{
		Image:        "mysql:8.0",
		ExposedPorts: []string{"3308/tcp"},
		Env: map[string]string{
			"MYSQL_USER":          user,
			"MYSQL_ROOT_PASSWORD": password,
			"MYSQL_PASSWORD":      password,
			"MYSQL_DATABASE":      dbName,
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("3308/tcp").WithStartupTimeout(MysqlStartTimeout),
			wait.ForLog("ready for connections"),
		),
	}

	dbContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("could not start Docker container, err: %s", err)
	}

	t.Cleanup(func() {
		t.Log("Removing MySQL container from Docker")
		if err := dbContainer.Terminate(ctx); err != nil {
			t.Errorf("failed to terminate MySQL container, err: %s", err)
		}
	})

	host, err := dbContainer.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get host where the container is exposed, err: %s", err)
	}

	port, err := dbContainer.MappedPort(ctx, "3308/tcp")
	if err != nil {
		t.Fatalf("failed to get externally mapped port to MySQL database, err: %s", err)
	}

	t.Log("Got connection port to MySQL: ", port)

	return &config.MySQLConfig{
		User:      user,
		Password:  password,
		Host:      host,
		Port:      port.Int(),
		Database:  dbName,
		CharSet:   "utf8mb4",
		ParseTime: true,
		TimeZone:  "UTC",
	}
}

func MockMySQLData(t *testing.T, conf *config.Config, sqls []string) *sqlx.DB {
	err := migrate.MySQLMigrateDrop(conf)
	if err != nil {
		t.Fatalf("MySQLMigrateDrop failed: %+v", err)
	}

	err = migrate.MySQLMigrateUp(conf)
	if err != nil {
		t.Fatalf("MySQLMigrateUp failed: %+v", err)
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%t&loc=%s",
		conf.MySQL.User,
		conf.MySQL.Password,
		conf.MySQL.Host,
		conf.MySQL.Port,
		conf.MySQL.Database,
		conf.MySQL.CharSet,
		conf.MySQL.ParseTime,
		conf.MySQL.TimeZone,
	)

	// Connect using sqlx
	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		t.Fatalf("unable to ping MySQL: %v", err)
	}

	for _, sqlStmt := range sqls {
		_, err := db.Exec(sqlStmt)
		if err != nil {
			t.Fatalf("failed to execute SQL: %v\nSQL: %s", err, sqlStmt)
		}
	}

	return db
}
