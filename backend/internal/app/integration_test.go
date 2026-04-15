package app_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/sumansahoo/taskflow/backend/internal/app"
	"github.com/sumansahoo/taskflow/backend/internal/config"
	"github.com/sumansahoo/taskflow/backend/internal/testutil"
)

type authResp struct {
	Token string `json:"token"`
	User  struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	} `json:"user"`
}

func register(t *testing.T, baseURL string, name string) authResp {
	t.Helper()

	body := map[string]any{
		"name":     name,
		"email":    name + "-" + strconv.FormatInt(time.Now().UnixNano(), 10) + "@example.com",
		"password": "password123",
	}
	b, _ := json.Marshal(body)
	res, err := http.Post(testutil.MustURL(t, baseURL, "/auth/register"), "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("register request: %v", err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })

	if res.StatusCode != http.StatusCreated {
		t.Fatalf("register status: got %d", res.StatusCode)
	}
	var out authResp
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatalf("decode register response: %v", err)
	}
	if out.Token == "" || out.User.ID == "" {
		t.Fatalf("expected token and user in register response")
	}
	return out
}

func doJSON(t *testing.T, method, url, token string, payload any) (*http.Response, map[string]any) {
	t.Helper()

	var body *bytes.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		body = bytes.NewReader(b)
	} else {
		body = bytes.NewReader(nil)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}

	var out map[string]any
	if res.StatusCode != http.StatusNoContent {
		_ = json.NewDecoder(res.Body).Decode(&out)
	}
	return res, out
}

func TestIntegration_AuthorizationFlows(t *testing.T) {
	db := testutil.StartPostgres(t)
	testutil.RunMigrations(t, db.DatabaseURL)

	cfg := &config.Config{
		DatabaseURL: db.DatabaseURL,
		JWTSecret:   "test-secret",
		Port:        "0",
	}

	srv := httptest.NewServer(app.NewRouter(cfg, db.Pool, testutil.TestLogger()))
	t.Cleanup(srv.Close)

	owner := register(t, srv.URL, "Owner")
	assignee := register(t, srv.URL, "Assignee")

	// 401 without token
	res, _ := doJSON(t, http.MethodGet, testutil.MustURL(t, srv.URL, "/projects"), "", nil)
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.StatusCode)
	}
	_ = res.Body.Close()

	// Create project as owner
	res, out := doJSON(t, http.MethodPost, testutil.MustURL(t, srv.URL, "/projects"), owner.Token, map[string]any{
		"name":        "Test Project",
		"description": "desc",
	})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create project status: got %d", res.StatusCode)
	}
	_ = res.Body.Close()
	projectID, _ := out["id"].(string)
	if projectID == "" {
		t.Fatalf("expected project id")
	}

	// Create 2 tasks: one assigned to assignee, one to owner
	res, out = doJSON(t, http.MethodPost, testutil.MustURL(t, srv.URL, "/projects/"+projectID+"/tasks"), owner.Token, map[string]any{
		"title":       "Assigned Task",
		"description": "",
		"priority":    "medium",
		"assignee_id": assignee.User.ID,
	})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create task status: got %d", res.StatusCode)
	}
	_ = res.Body.Close()
	assignedTaskID, _ := out["id"].(string)
	if assignedTaskID == "" {
		t.Fatalf("expected task id")
	}

	res, out = doJSON(t, http.MethodPost, testutil.MustURL(t, srv.URL, "/projects/"+projectID+"/tasks"), owner.Token, map[string]any{
		"title":       "Owner Task",
		"description": "",
		"priority":    "medium",
		"assignee_id": owner.User.ID,
	})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create task2 status: got %d", res.StatusCode)
	}
	_ = res.Body.Close()

	// Assignee-only cannot change title (400 validation error)
	res, _ = doJSON(t, http.MethodPatch, testutil.MustURL(t, srv.URL, "/tasks/"+assignedTaskID), assignee.Token, map[string]any{
		"title": "Hacked",
	})
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("assignee title patch expected 400, got %d", res.StatusCode)
	}
	_ = res.Body.Close()

	// Assignee-only can change status
	res, _ = doJSON(t, http.MethodPatch, testutil.MustURL(t, srv.URL, "/tasks/"+assignedTaskID), assignee.Token, map[string]any{
		"status": "done",
	})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("assignee status patch expected 200, got %d", res.StatusCode)
	}
	_ = res.Body.Close()

	// Assignee cannot delete task unless creator (should be 403)
	res, _ = doJSON(t, http.MethodDelete, testutil.MustURL(t, srv.URL, "/tasks/"+assignedTaskID), assignee.Token, nil)
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("assignee delete expected 403, got %d", res.StatusCode)
	}
	_ = res.Body.Close()

	// Non-owner GET /projects/:id should only include tasks assigned to them
	res, out = doJSON(t, http.MethodGet, testutil.MustURL(t, srv.URL, "/projects/"+projectID), assignee.Token, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("assignee get project expected 200, got %d", res.StatusCode)
	}
	_ = res.Body.Close()

	tasksAny, _ := out["tasks"].([]any)
	if len(tasksAny) != 1 {
		t.Fatalf("expected 1 task for assignee view, got %d", len(tasksAny))
	}

	// Owner can delete
	res, _ = doJSON(t, http.MethodDelete, testutil.MustURL(t, srv.URL, "/tasks/"+assignedTaskID), owner.Token, nil)
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("owner delete expected 204, got %d", res.StatusCode)
	}
	_ = res.Body.Close()
}

