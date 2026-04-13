package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sumansahoo/taskflow/backend/internal/handler/response"
	"github.com/sumansahoo/taskflow/backend/internal/middleware"
	"github.com/sumansahoo/taskflow/backend/internal/repository"
)

type UserHandler struct {
	users *repository.UserRepo
}

func NewUserHandler(users *repository.UserRepo) *UserHandler {
	return &UserHandler{users: users}
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
