package testutil

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

type TestDB struct {
	Pool        *pgxpool.Pool
	DatabaseURL string
}

func StartPostgres(t *testing.T) *TestDB {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("taskflow_test"),
		postgres.WithUsername("taskflow"),
		postgres.WithPassword("taskflow"),
	)
	if err != nil {
		t.Fatalf("starting postgres container: %v", err)
	}
	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	dbURL, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("postgres connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)

	for i := range 30 {
		pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := pool.Ping(pingCtx)
		cancel()
		if err == nil {
			break
		}
		if i == 29 {
			t.Fatalf("postgres ping failed: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
	}

	return &TestDB{
		Pool:        pool,
		DatabaseURL: dbURL,
	}
}

func MigrationsDir() string {
	_, thisFile, _, _ := runtime.Caller(0)
	// backend/internal/testutil -> backend/migrations
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "migrations"))
}

func RunMigrations(t *testing.T, databaseURL string) {
	t.Helper()

	path := "file://" + MigrationsDir()
	m, err := migrate.New(path, databaseURL)
	if err != nil {
		t.Fatalf("migrate.New: %v", err)
	}
	t.Cleanup(func() {
		_, _ = m.Close()
	})

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("migrate.Up: %v", err)
	}
}

func TestLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

func MustURL(t *testing.T, base, path string) string {
	t.Helper()
	return fmt.Sprintf("%s%s", base, path)
}

