package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/sumansahoo/taskflow/backend/internal/handler/response"
	"github.com/sumansahoo/taskflow/backend/internal/middleware"
	"github.com/sumansahoo/taskflow/backend/internal/repository"
)

type UserHandler struct {
	users *repository.UserRepo
	prefs *repository.PreferencesRepo
	projects *repository.ProjectRepo
}

func NewUserHandler(users *repository.UserRepo, prefs *repository.PreferencesRepo, projects *repository.ProjectRepo) *UserHandler {
	return &UserHandler{users: users, prefs: prefs, projects: projects}
}

func (h *UserHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	user, err := h.users.GetByID(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to get user")
		return
	}

	response.JSON(w, http.StatusOK, userJSON{
		ID:    user.ID,
		Name:  user.Name,
		Email: user.Email,
	})
}

func (h *UserHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	users, err := h.users.ListAll(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to list users")
		return
	}

	type memberJSON struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	members := make([]memberJSON, 0, len(users))
	for _, u := range users {
		members = append(members, memberJSON{ID: u.ID, Name: u.Name, Email: u.Email})
	}

	response.JSON(w, http.StatusOK, map[string]any{"users": members})
}

func (h *UserHandler) ListByProject(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	_, err := h.projects.GetByID(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.Error(w, http.StatusNotFound, "not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to get project")
		return
	}

	canAccess, err := h.projects.CanAccess(r.Context(), projectID, userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to check access")
		return
	}
	if !canAccess {
		response.Error(w, http.StatusForbidden, "forbidden")
		return
	}

	users, err := h.users.ListByProject(r.Context(), projectID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to list members")
		return
	}

	type memberJSON struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	members := make([]memberJSON, 0, len(users))
	for _, u := range users {
		members = append(members, memberJSON{ID: u.ID, Name: u.Name, Email: u.Email})
	}

	response.JSON(w, http.StatusOK, map[string]any{"users": members})
}

type preferencesJSON struct {
	ProjectsPageSize *int `json:"projects_page_size"`
	TasksPageSize    *int `json:"tasks_page_size"`
}

func (h *UserHandler) GetPreferences(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	prefs, err := h.prefs.Get(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to get preferences")
		return
	}

	projects := prefs.ProjectsPageSize
	tasks := prefs.TasksPageSize
	response.JSON(w, http.StatusOK, preferencesJSON{
		ProjectsPageSize: &projects,
		TasksPageSize:    &tasks,
	})
}

func (h *UserHandler) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req preferencesJSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	errs := map[string]string{}
	if req.ProjectsPageSize != nil && *req.ProjectsPageSize != 6 && *req.ProjectsPageSize != 12 && *req.ProjectsPageSize != 24 {
		errs["projects_page_size"] = "must be 6, 12, or 24"
	}
	if req.TasksPageSize != nil && *req.TasksPageSize != 6 && *req.TasksPageSize != 12 && *req.TasksPageSize != 24 {
		errs["tasks_page_size"] = "must be 6, 12, or 24"
	}
	if len(errs) > 0 {
		response.ValidationError(w, errs)
		return
	}

	saved, err := h.prefs.Set(r.Context(), userID, req.ProjectsPageSize, req.TasksPageSize)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to update preferences")
		return
	}

	projects := saved.ProjectsPageSize
	tasks := saved.TasksPageSize
	response.JSON(w, http.StatusOK, preferencesJSON{
		ProjectsPageSize: &projects,
		TasksPageSize:    &tasks,
	})
}
