package main

//	@title			TaskFlow API
//	@version		1.0
//	@description	REST API for TaskFlow (projects, tasks, auth).
//
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				Type "Bearer <token>"
//
//	@BasePath	/

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sumansahoo/taskflow/backend/internal/config"
	"github.com/sumansahoo/taskflow/backend/internal/app"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	pool, err := connectDB(cfg.DatabaseURL, logger)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	runMigrations(cfg.DatabaseURL, logger)
	runSeed(pool, logger)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      app.NewRouter(cfg, pool, logger),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down gracefully")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("forced shutdown", "error", err)
	}
	logger.Info("server stopped")
}

func connectDB(databaseURL string, logger *slog.Logger) (*pgxpool.Pool, error) {
	var pool *pgxpool.Pool
	var err error

	for i := range 30 {
		pool, err = pgxpool.New(context.Background(), databaseURL)
		if err == nil {
			if pingErr := pool.Ping(context.Background()); pingErr == nil {
				logger.Info("connected to database")
				return pool, nil
			}
			pool.Close()
		}
		logger.Info("waiting for database", "attempt", i+1)
		time.Sleep(time.Second)
	}
	return nil, fmt.Errorf("could not connect to database after 30 attempts: %w", err)
}

func migrationsPath() string {
	if p := os.Getenv("MIGRATIONS_PATH"); p != "" {
		return "file://" + p
	}
	// Try relative path for local dev, fall back to Docker container path
	if _, err := os.Stat("migrations"); err == nil {
		abs, _ := os.Getwd()
		return "file://" + abs + "/migrations"
	}
	return "file:///app/migrations"
}

func seedPath() string {
	if p := os.Getenv("SEED_PATH"); p != "" {
		return p
	}
	if _, err := os.Stat("seed/seed.sql"); err == nil {
		return "seed/seed.sql"
	}
	return "/app/seed/seed.sql"
}

func runMigrations(databaseURL string, logger *slog.Logger) {
	path := migrationsPath()
	logger.Info("running migrations", "path", path)

	m, err := migrate.New(path, databaseURL)
	if err != nil {
		logger.Error("failed to create migrator", "error", err)
		os.Exit(1)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		logger.Error("migration failed", "error", err)
		os.Exit(1)
	}
	logger.Info("migrations applied successfully")
}

func runSeed(pool *pgxpool.Pool, logger *slog.Logger) {
	path := seedPath()
	seedSQL, err := os.ReadFile(path)
	if err != nil {
		logger.Warn("seed file not found, skipping", "error", err)
		return
	}

	var count int
	err = pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM projects
		 WHERE owner_id = 'a1b2c3d4-e5f6-7890-abcd-ef1234567890'
		   AND (name = 'Website Redesign' OR name LIKE 'Seed Project %')`,
	).Scan(&count)
	if err == nil && count > 0 {
		logger.Info("seed data already present, skipping")
		return
	}

	if _, err := pool.Exec(context.Background(), string(seedSQL)); err != nil {
		logger.Warn("seed execution had issues", "error", err)
		return
	}
	logger.Info("seed data applied")
}
