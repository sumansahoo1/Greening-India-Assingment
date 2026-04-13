package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/sumansahoo/taskflow/backend/internal/handler/response"
	"github.com/sumansahoo/taskflow/backend/internal/middleware"
	"github.com/sumansahoo/taskflow/backend/internal/model"
	"github.com/sumansahoo/taskflow/backend/internal/repository"
)

type TaskHandler struct {
	tasks    *repository.TaskRepo
	projects *repository.ProjectRepo
}

func NewTaskHandler(tasks *repository.TaskRepo, projects *repository.ProjectRepo) *TaskHandler {
	return &TaskHandler{tasks: tasks, projects: projects}
}

type createTaskRequest struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Priority    string  `json:"priority"`
	AssigneeID  *string `json:"assignee_id"`
	DueDate     *string `json:"due_date"`
}

type updateTaskRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
	Priority    *string `json:"priority"`
	AssigneeID  *string `json:"assignee_id"`
	DueDate     *string `json:"due_date"`
}

var validStatuses = map[string]bool{"todo": true, "in_progress": true, "done": true}
var validPriorities = map[string]bool{"low": true, "medium": true, "high": true}

func (h *TaskHandler) ListByProject(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	status := r.URL.Query().Get("status")
	assignee := r.URL.Query().Get("assignee")
	page, limit := parsePagination(r)

	_, err := h.projects.GetByID(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.Error(w, http.StatusNotFound, "not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to get project")
		return
	}

	tasks, total, err := h.tasks.ListByProject(r.Context(), projectID, status, assignee, page, limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to list tasks")
		return
	}

	if tasks == nil {
		tasks = []model.Task{}
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"tasks": tasks,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var req createTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	errs := map[string]string{}
	if strings.TrimSpace(req.Title) == "" {
		errs["title"] = "is required"
	}
	priority := req.Priority
	if priority == "" {
		priority = "medium"
	}
	if !validPriorities[priority] {
		errs["priority"] = "must be low, medium, or high"
	}
	if len(errs) > 0 {
		response.ValidationError(w, errs)
		return
	}

	task, err := h.tasks.Create(
		r.Context(),
		strings.TrimSpace(req.Title),
		req.Description,
		"todo",
		priority,
		projectID,
		req.AssigneeID,
		userID,
		req.DueDate,
	)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to create task")
		return
	}

	response.JSON(w, http.StatusCreated, task)
}

func (h *TaskHandler) Update(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")

	_, err := h.tasks.GetByID(r.Context(), taskID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.Error(w, http.StatusNotFound, "not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to get task")
		return
	}

	var req updateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	fields := map[string]any{}
	errs := map[string]string{}

	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			errs["title"] = "cannot be empty"
		}
		fields["title"] = title
	}
	if req.Description != nil {
		fields["description"] = *req.Description
	}
	if req.Status != nil {
		if !validStatuses[*req.Status] {
			errs["status"] = "must be todo, in_progress, or done"
		}
		fields["status"] = *req.Status
	}
	if req.Priority != nil {
		if !validPriorities[*req.Priority] {
			errs["priority"] = "must be low, medium, or high"
		}
		fields["priority"] = *req.Priority
	}
	if req.AssigneeID != nil {
		if *req.AssigneeID == "" {
			fields["assignee_id"] = nil
		} else {
			fields["assignee_id"] = *req.AssigneeID
		}
	}
	if req.DueDate != nil {
		if *req.DueDate == "" {
			fields["due_date"] = nil
		} else {
			fields["due_date"] = *req.DueDate
		}
	}

	if len(errs) > 0 {
		response.ValidationError(w, errs)
		return
	}

	if len(fields) == 0 {
		response.Error(w, http.StatusBadRequest, "no fields to update")
		return
	}

	updated, err := h.tasks.Update(r.Context(), taskID, fields)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to update task")
		return
	}

	response.JSON(w, http.StatusOK, updated)
}

func (h *TaskHandler) Delete(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	task, err := h.tasks.GetByID(r.Context(), taskID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.Error(w, http.StatusNotFound, "not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to get task")
		return
	}

	project, err := h.projects.GetByID(r.Context(), task.ProjectID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to get project")
		return
	}

	if project.OwnerID != userID && task.CreatedBy != userID {
		response.Error(w, http.StatusForbidden, "forbidden")
		return
	}

	if err := h.tasks.Delete(r.Context(), taskID); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to delete task")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
