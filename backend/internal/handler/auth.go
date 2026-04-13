package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/sumansahoo/taskflow/backend/internal/handler/response"
	"github.com/sumansahoo/taskflow/backend/internal/repository"
)

type AuthHandler struct {
	users     *repository.UserRepo
	jwtSecret string
}

func NewAuthHandler(users *repository.UserRepo, jwtSecret string) *AuthHandler {
	return &AuthHandler{users: users, jwtSecret: jwtSecret}
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string   `json:"token"`
	User  userJSON `json:"user"`
}

type userJSON struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	errs := map[string]string{}
	if strings.TrimSpace(req.Name) == "" {
		errs["name"] = "is required"
	}
	if strings.TrimSpace(req.Email) == "" {
		errs["email"] = "is required"
	}
	if len(req.Password) < 6 {
		errs["password"] = "must be at least 6 characters"
	}
	if len(errs) > 0 {
		response.ValidationError(w, errs)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user, err := h.users.Create(r.Context(), strings.TrimSpace(req.Name), strings.ToLower(strings.TrimSpace(req.Email)), string(hash))
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			response.ValidationError(w, map[string]string{"email": "already registered"})
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	token, err := h.generateToken(user.ID, user.Email)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	response.JSON(w, http.StatusCreated, authResponse{
		Token: token,
		User:  userJSON{ID: user.ID, Name: user.Name, Email: user.Email},
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	errs := map[string]string{}
	if strings.TrimSpace(req.Email) == "" {
		errs["email"] = "is required"
	}
	if req.Password == "" {
		errs["password"] = "is required"
	}
	if len(errs) > 0 {
		response.ValidationError(w, errs)
		return
	}

	user, err := h.users.GetByEmail(r.Context(), strings.ToLower(strings.TrimSpace(req.Email)))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.Error(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to query user")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		response.Error(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	token, err := h.generateToken(user.ID, user.Email)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	response.JSON(w, http.StatusOK, authResponse{
		Token: token,
		User:  userJSON{ID: user.ID, Name: user.Name, Email: user.Email},
	})
}

func (h *AuthHandler) generateToken(userID, email string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}
