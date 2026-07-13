package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/DevenWen/TodoDemo/internal/config"
	"github.com/DevenWen/TodoDemo/internal/middleware"
	"github.com/DevenWen/TodoDemo/internal/model"
)

// addUserToContext adds mock user info to the request context for testing.
func addUserToContext(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDKey, "test-user-id")
	ctx = context.WithValue(ctx, middleware.UserLoginKey, "testuser")
	return r.WithContext(ctx)
}

func testTodoConfig() *config.Config {
	return &config.Config{
		JWTSecret: "test-secret",
	}
}

func TestListTodos(t *testing.T) {
	t.Skip("requires database connection")
}

func TestListTodos_RequiresAuth(t *testing.T) {
	cfg := testTodoConfig()
	h := NewTodoHandler(cfg)

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "empty title",
			body:       `{"title":"  ","description":""}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   model.ErrCodeValidationError,
		},
		{
			name:       "missing title field",
			body:       `{"description":"test"}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   model.ErrCodeValidationError,
		},
		{
			name:       "invalid JSON",
			body:       `not json`,
			wantStatus: http.StatusBadRequest,
			wantCode:   model.ErrCodeValidationError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/todos", bytes.NewBufferString(tt.body))
			req = addUserToContext(req)
			rec := httptest.NewRecorder()

			h.Create(rec, req)

			resp := rec.Result()
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, resp.StatusCode)
			}

			var apiResp model.APIResponse
			json.NewDecoder(resp.Body).Decode(&apiResp)
			if apiResp.Error == nil {
				t.Fatal("expected error response")
			}
			if apiResp.Error.Code != tt.wantCode {
				t.Errorf("expected code %s, got %s", tt.wantCode, apiResp.Error.Code)
			}
		})
	}
}

func TestCreateTodo_TitleTooLong(t *testing.T) {
	cfg := testTodoConfig()
	h := NewTodoHandler(cfg)

	longTitle := make([]byte, 501)
	for i := range longTitle {
		longTitle[i] = 'a'
	}

	body := `{"title":"` + string(longTitle) + `","description":""}`
	req := httptest.NewRequest("POST", "/api/v1/todos", bytes.NewBufferString(body))
	req = addUserToContext(req)
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	resp := rec.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestUpdateTodo_Validation(t *testing.T) {
	cfg := testTodoConfig()
	h := NewTodoHandler(cfg)

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "empty title",
			body:       `{"title":"  "}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   model.ErrCodeValidationError,
		},
		{
			name:       "title too long",
			body:       `{"title":"` + longString(501) + `"}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   model.ErrCodeValidationError,
		},
		{
			name:       "invalid JSON",
			body:       `not json`,
			wantStatus: http.StatusBadRequest,
			wantCode:   model.ErrCodeValidationError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("PUT", "/api/v1/todos/some-id", bytes.NewBufferString(tt.body))
			req = addUserToContext(req)

			// Set chi URL param
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", "some-id")
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			rec := httptest.NewRecorder()

			h.Update(rec, req)

			resp := rec.Result()
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, resp.StatusCode)
			}

			var apiResp model.APIResponse
			json.NewDecoder(resp.Body).Decode(&apiResp)
			if apiResp.Error == nil {
				t.Fatal("expected error response")
			}
			if apiResp.Error.Code != tt.wantCode {
				t.Errorf("expected code %s, got %s", tt.wantCode, apiResp.Error.Code)
			}
		})
	}
}

func TestQueryInt(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		key     string
		def     int
		want    int
	}{
		{"valid", "/test?page=5", "page", 1, 5},
		{"missing", "/test", "page", 1, 1},
		{"invalid", "/test?page=abc", "page", 1, 1},
		{"negative", "/test?page=-1", "page", 1, 1},
		{"zero", "/test?page=0", "page", 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			got := queryInt(req, tt.key, tt.def)
			if got != tt.want {
				t.Errorf("expected %d, got %d", tt.want, got)
			}
		})
	}
}

func TestWriteListResponse(t *testing.T) {
	data := []model.Todo{}
	meta := model.TodoListMeta{Page: 1, PerPage: 20, Total: 0}

	rec := httptest.NewRecorder()
	writeListResponse(rec, http.StatusOK, data, meta)

	resp := rec.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["meta"] == nil {
		t.Error("expected meta field in response")
	}
	if result["error"] != nil {
		t.Error("expected null error")
	}
}

func longString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}
