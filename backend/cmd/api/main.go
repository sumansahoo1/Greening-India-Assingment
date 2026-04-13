package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sumansahoo/taskflow/backend/internal/config"
	"github.com/sumansahoo/taskflow/backend/internal/handler"
	"github.com/sumansahoo/taskflow/backend/internal/middleware"
	"github.com/sumansahoo/taskflow/backend/internal/repository"
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

	userRepo := repository.NewUserRepo(pool)
	projectRepo := repository.NewProjectRepo(pool)
	taskRepo := repository.NewTaskRepo(pool)

	authHandler := handler.NewAuthHandler(userRepo, cfg.JWTSecret)
	projectHandler := handler.NewProjectHandler(projectRepo, taskRepo)
	taskHandler := handler.NewTaskHandler(taskRepo, projectRepo)
	userHandler := handler.NewUserHandler(userRepo)

	r := chi.NewRouter()

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	r.Use(middleware.Logging(logger))

	r.Post("/auth/register", authHandler.Register)
	r.Post("/auth/login", authHandler.Login)

	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(cfg.JWTSecret))

		r.Get("/me", userHandler.Me)
		r.Get("/users", userHandler.ListAll)
		r.Get("/projects/{id}/members", userHandler.ListByProject)

		r.Get("/projects", projectHandler.List)
		r.Post("/projects", projectHandler.Create)
		r.Get("/projects/{id}", projectHandler.Get)
		r.Patch("/projects/{id}", projectHandler.Update)
		r.Delete("/projects/{id}", projectHandler.Delete)
		r.Get("/projects/{id}/stats", projectHandler.Stats)

		r.Get("/projects/{id}/tasks", taskHandler.ListByProject)
		r.Post("/projects/{id}/tasks", taskHandler.Create)
		r.Patch("/tasks/{id}", taskHandler.Update)
		r.Delete("/tasks/{id}", taskHandler.Delete)
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
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

	if _, err := pool.Exec(context.Background(), string(seedSQL)); err != nil {
		logger.Warn("seed execution had issues (may already be seeded)", "error", err)
		return
	}
	logger.Info("seed data applied")
}
