package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/sumansahoo/taskflow/backend/internal/handler/response"
	"github.com/sumansahoo/taskflow/backend/internal/middleware"
	"github.com/sumansahoo/taskflow/backend/internal/model"
	"github.com/sumansahoo/taskflow/backend/internal/repository"
)

type ProjectHandler struct {
	projects *repository.ProjectRepo
	tasks    *repository.TaskRepo
}

func NewProjectHandler(projects *repository.ProjectRepo, tasks *repository.TaskRepo) *ProjectHandler {
	return &ProjectHandler{projects: projects, tasks: tasks}
}

type createProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type updateProjectRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	page, limit := parsePagination(r)

	projects, total, err := h.projects.List(r.Context(), userID, page, limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to list projects")
		return
	}

	if projects == nil {
		projects = []model.Project{}
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"projects": projects,
		"total":    total,
		"page":     page,
		"limit":    limit,
	})
}

func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req createProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	errs := map[string]string{}
	if strings.TrimSpace(req.Name) == "" {
		errs["name"] = "is required"
	}
	if len(errs) > 0 {
		response.ValidationError(w, errs)
		return
	}

	project, err := h.projects.Create(r.Context(), strings.TrimSpace(req.Name), req.Description, userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to create project")
		return
	}

	response.JSON(w, http.StatusCreated, project)
}

func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	project, err := h.projects.GetByID(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.Error(w, http.StatusNotFound, "not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to get project")
		return
	}

	assignee := ""
	if project.OwnerID != userID {
		canAccess, err := h.projects.CanAccess(r.Context(), projectID, userID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to check access")
			return
		}
		if !canAccess {
			response.Error(w, http.StatusForbidden, "forbidden")
			return
		}
		assignee = userID
	}

	tasks, _, err := h.tasks.ListByProject(r.Context(), projectID, "", assignee, 1, 1000)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to list tasks")
		return
	}

	if tasks == nil {
		tasks = []model.Task{}
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"id":          project.ID,
		"name":        project.Name,
		"description": project.Description,
		"owner_id":    project.OwnerID,
		"created_at":  project.CreatedAt,
		"tasks":       tasks,
	})
}

func (h *ProjectHandler) Update(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	project, err := h.projects.GetByID(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.Error(w, http.StatusNotFound, "not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to get project")
		return
	}

	if project.OwnerID != userID {
		response.Error(w, http.StatusForbidden, "forbidden")
		return
	}

	var req updateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	name := project.Name
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
	}
	desc := project.Description
	if req.Description != nil {
		desc = *req.Description
	}

	if name == "" {
		response.ValidationError(w, map[string]string{"name": "cannot be empty"})
		return
	}

	updated, err := h.projects.Update(r.Context(), projectID, name, desc)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to update project")
		return
	}

	response.JSON(w, http.StatusOK, updated)
}

func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	project, err := h.projects.GetByID(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.Error(w, http.StatusNotFound, "not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to get project")
		return
	}

	if project.OwnerID != userID {
		response.Error(w, http.StatusForbidden, "forbidden")
		return
	}

	if err := h.projects.Delete(r.Context(), projectID); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to delete project")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ProjectHandler) Stats(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	project, err := h.projects.GetByID(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.Error(w, http.StatusNotFound, "not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to get project")
		return
	}

	if project.OwnerID != userID {
		canAccess, err := h.projects.CanAccess(r.Context(), projectID, userID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to check access")
			return
		}
		if !canAccess {
			response.Error(w, http.StatusForbidden, "forbidden")
			return
		}
		stats, err := h.tasks.StatsByProjectForAssignee(r.Context(), projectID, userID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to get stats")
			return
		}
		response.JSON(w, http.StatusOK, stats)
		return
	}

	stats, err := h.tasks.StatsByProject(r.Context(), projectID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to get stats")
		return
	}

	response.JSON(w, http.StatusOK, stats)
}

func parsePagination(r *http.Request) (int, int) {
	page := 1
	limit := 20

	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}

	return page, limit
}
