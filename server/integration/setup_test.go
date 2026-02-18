package integration

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"ota/storage"
)

// TestMain runs before all tests and loads .env file
func TestMain(m *testing.M) {
	// Load .env file from server directory (../.env from integration/)
	if err := godotenv.Load("../.env"); err != nil {
		log.Printf("Warning: Could not load .env file: %v", err)
		log.Println("Environment variables must be set manually or tests requiring API keys will be skipped")
	}

	// Run tests
	code := m.Run()

	os.Exit(code)
}

type TestDB struct {
	Container *postgres.PostgresContainer
	Pool      *pgxpool.Pool
	ConnStr   string
}

func SetupTestDB(t *testing.T) *TestDB {
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("ota_test"),
		postgres.WithUsername("ota"),
		postgres.WithPassword("ota_test_password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second)),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	// Run migrations
	if err := storage.RunMigrations(connStr, "../migrations"); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	pool, err := storage.NewPool(ctx, connStr)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	})

	return &TestDB{
		Container: pgContainer,
		Pool:      pool,
		ConnStr:   connStr,
	}
}

func (db *TestDB) Truncate(t *testing.T, tables ...string) {
	ctx := context.Background()
	for _, table := range tables {
		_, err := db.Pool.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
		if err != nil {
			t.Fatalf("failed to truncate table %s: %v", table, err)
		}
	}
}
