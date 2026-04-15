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
	AssigneeID  optionalString `json:"assignee_id"`
	DueDate     *string `json:"due_date"`
}

// optionalString distinguishes between a missing JSON field and an explicit null.
// This is needed so PATCH can clear assignee_id (set to NULL).
type optionalString struct {
	Set   bool
	Value *string
}

func (o *optionalString) UnmarshalJSON(b []byte) error {
	o.Set = true
	if string(b) == "null" {
		o.Value = nil
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	o.Value = &s
	return nil
}

var validStatuses = map[string]bool{"todo": true, "in_progress": true, "done": true}
var validPriorities = map[string]bool{"low": true, "medium": true, "high": true}

// ListByProject godoc
//
//	@Summary	List tasks for a project
//	@Tags		tasks
//	@Security	BearerAuth
//	@Produce	json
//	@Param		id			path		string	true	"Project ID"
//	@Param		status		query		string	false	"Filter by status"	Enums(todo,in_progress,done)
//	@Param		assignee		query		string	false	"Filter by assignee id or 'unassigned'"
//	@Param		page		query		int		false	"Page number"	default(1)
//	@Param		limit		query		int		false	"Page size"		default(20)
//	@Success	200			{object}	map[string]any
//	@Failure	401			{object}	map[string]string
//	@Failure	403			{object}	map[string]string
//	@Failure	404			{object}	map[string]string
//	@Failure	500			{object}	map[string]string
//	@Router		/projects/{id}/tasks [get]
func (h *TaskHandler) ListByProject(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())
	status := r.URL.Query().Get("status")
	page, limit := parsePagination(r)

	project, err := h.projects.GetByID(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.Error(w, http.StatusNotFound, "not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to get project")
		return
	}

	assignee := r.URL.Query().Get("assignee")
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

// Create godoc
//
//	@Summary	Create a task in a project
//	@Tags		tasks
//	@Security	BearerAuth
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string				true	"Project ID"
//	@Param		request	body		createTaskRequest	true	"Task payload"
//	@Success	201		{object}	model.Task
//	@Failure	400		{object}	map[string]any
//	@Failure	401		{object}	map[string]string
//	@Failure	403		{object}	map[string]string
//	@Failure	404		{object}	map[string]string
//	@Failure	500		{object}	map[string]string
//	@Router		/projects/{id}/tasks [post]
func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	if req.AssigneeID != nil && (strings.TrimSpace(*req.AssigneeID) == "" || *req.AssigneeID == "unassigned") {
		req.AssigneeID = nil
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

		if req.AssigneeID == nil {
			req.AssigneeID = &userID
		} else if *req.AssigneeID != userID {
			response.Error(w, http.StatusForbidden, "forbidden")
			return
		}
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

// Update godoc
//
//	@Summary	Update a task
//	@Tags		tasks
//	@Security	BearerAuth
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string				true	"Task ID"
//	@Param		request	body		updateTaskRequest	true	"Update payload"
//	@Success	200		{object}	model.Task
//	@Failure	400		{object}	map[string]any
//	@Failure	401		{object}	map[string]string
//	@Failure	403		{object}	map[string]string
//	@Failure	404		{object}	map[string]string
//	@Failure	500		{object}	map[string]string
//	@Router		/tasks/{id} [patch]
func (h *TaskHandler) Update(w http.ResponseWriter, r *http.Request) {
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
		if errors.Is(err, pgx.ErrNoRows) {
			response.Error(w, http.StatusNotFound, "not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to get project")
		return
	}

	isOwner := project.OwnerID == userID
	isCreator := task.CreatedBy == userID
	isAssignee := task.AssigneeID != nil && *task.AssigneeID == userID

	if !isOwner && !isCreator && !isAssignee {
		response.Error(w, http.StatusForbidden, "forbidden")
		return
	}

	// Assignees who are not the owner or creator may only update status.
	assigneeOnly := !isOwner && !isCreator && isAssignee

	var req updateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if assigneeOnly {
		restricted := map[string]string{}
		if req.Title != nil {
			restricted["title"] = "cannot be modified"
		}
		if req.Description != nil {
			restricted["description"] = "cannot be modified"
		}
		if req.Priority != nil {
			restricted["priority"] = "cannot be modified"
		}
		if req.AssigneeID.Set {
			restricted["assignee_id"] = "cannot be modified"
		}
		if req.DueDate != nil {
			restricted["due_date"] = "cannot be modified"
		}
		if len(restricted) > 0 {
			response.ValidationError(w, restricted)
			return
		}
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
	if req.AssigneeID.Set {
		if req.AssigneeID.Value == nil || strings.TrimSpace(*req.AssigneeID.Value) == "" || *req.AssigneeID.Value == "unassigned" {
			fields["assignee_id"] = nil
		} else {
			fields["assignee_id"] = *req.AssigneeID.Value
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

// Delete godoc
//
//	@Summary	Delete a task
//	@Tags		tasks
//	@Security	BearerAuth
//	@Param		id	path	string	true	"Task ID"
//	@Success	204
//	@Failure	401	{object}	map[string]string
//	@Failure	403	{object}	map[string]string
//	@Failure	404	{object}	map[string]string
//	@Failure	500	{object}	map[string]string
//	@Router		/tasks/{id} [delete]
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
		if errors.Is(err, pgx.ErrNoRows) {
			response.Error(w, http.StatusNotFound, "not found")
			return
		}
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
