package app

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	_ "github.com/sumansahoo/taskflow/backend/docs"
	"github.com/sumansahoo/taskflow/backend/internal/config"
	"github.com/sumansahoo/taskflow/backend/internal/handler"
	"github.com/sumansahoo/taskflow/backend/internal/middleware"
	"github.com/sumansahoo/taskflow/backend/internal/repository"
)

func NewRouter(cfg *config.Config, pool *pgxpool.Pool, logger *slog.Logger) http.Handler {
	userRepo := repository.NewUserRepo(pool)
	prefsRepo := repository.NewPreferencesRepo(pool)
	projectRepo := repository.NewProjectRepo(pool)
	taskRepo := repository.NewTaskRepo(pool)

	authHandler := handler.NewAuthHandler(userRepo, cfg.JWTSecret)
	projectHandler := handler.NewProjectHandler(projectRepo, taskRepo)
	taskHandler := handler.NewTaskHandler(taskRepo, projectRepo)
	userHandler := handler.NewUserHandler(userRepo, prefsRepo, projectRepo)

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

	r.Get("/docs/*", httpSwagger.WrapHandler)

	r.Post("/auth/register", authHandler.Register)
	r.Post("/auth/login", authHandler.Login)

	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(cfg.JWTSecret))

		r.Get("/me", userHandler.Me)
		r.Get("/me/preferences", userHandler.GetPreferences)
		r.Patch("/me/preferences", userHandler.UpdatePreferences)
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

	return r
}

