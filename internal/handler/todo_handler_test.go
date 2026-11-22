package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/DevenWen/TodoDemo/internal/config"
	"github.com/DevenWen/TodoDemo/internal/middleware"
	"github.com/DevenWen/TodoDemo/internal/model"
	"github.com/DevenWen/TodoDemo/internal/repository"
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
		name string
		url  string
		key  string
		def  int
		want int
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

// fakeTodoStore records the filter a handler passes to the repository and
// returns canned results, so handler behaviour can be tested without a DB.
type fakeTodoStore struct {
	listFilter repository.TodoFilter
	listItems  []model.Todo
	listTotal  int
	listErr    error
	updateErr  error
	deleteErr  error
}

func (f *fakeTodoStore) List(_ context.Context, filter repository.TodoFilter) ([]model.Todo, int, error) {
	f.listFilter = filter
	if f.listErr != nil {
		return nil, 0, f.listErr
	}
	items := f.listItems
	if len(items) > filter.PerPage {
		items = items[:filter.PerPage]
	}
	return items, f.listTotal, nil
}

func (f *fakeTodoStore) Create(_ context.Context, userID string, req model.CreateTodoRequest) (*model.Todo, error) {
	return &model.Todo{ID: "new-id", UserID: userID, Title: req.Title}, nil
}

func (f *fakeTodoStore) Update(_ context.Context, id, userID string, req model.UpdateTodoRequest) (*model.Todo, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	return &model.Todo{ID: id, UserID: userID}, nil
}

func (f *fakeTodoStore) Delete(_ context.Context, _, _ string) error {
	return f.deleteErr
}

func TestUpdateTodo_NotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"direct", repository.ErrNotFound},
		{"wrapped", fmt.Errorf("repository: %w", repository.ErrNotFound)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeTodoStore{updateErr: tt.err}
			h := newTodoHandlerWithStore(testTodoConfig(), store)

			req := httptest.NewRequest("PUT", "/api/v1/todos/some-id", bytes.NewBufferString(`{"title":"updated"}`))
			req = addUserToContext(req)
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", "some-id")
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
			rec := httptest.NewRecorder()

			h.Update(rec, req)

			resp := rec.Result()
			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("expected status 404, got %d", resp.StatusCode)
			}
			var apiResp model.APIResponse
			if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if apiResp.Error == nil || apiResp.Error.Code != model.ErrCodeNotFound {
				t.Fatalf("expected NOT_FOUND error, got %+v", apiResp.Error)
			}
		})
	}
}

func TestListTodos_PerPageNormalized(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  int
	}{
		{"too large", "?per_page=999", 100},
		{"at max", "?per_page=100", 100},
		{"valid", "?per_page=50", 50},
		{"zero", "?per_page=0", 20},
		{"negative", "?per_page=-5", 20},
		{"not a number", "?per_page=abc", 20},
		{"default", "", 20},
	}

	const total = 27
	items := make([]model.Todo, total)
	for i := range items {
		items[i] = model.Todo{ID: fmt.Sprintf("todo-%d", i)}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeTodoStore{listItems: items, listTotal: total}
			h := newTodoHandlerWithStore(testTodoConfig(), store)

			req := httptest.NewRequest("GET", "/api/v1/todos"+tt.query, nil)
			req = addUserToContext(req)
			rec := httptest.NewRecorder()

			h.List(rec, req)

			resp := rec.Result()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected status 200, got %d", resp.StatusCode)
			}
			if store.listFilter.PerPage != tt.want {
				t.Fatalf("expected repository per_page %d, got %d", tt.want, store.listFilter.PerPage)
			}

			var body struct {
				Data []model.Todo       `json:"data"`
				Meta model.TodoListMeta `json:"meta"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Meta.PerPage != tt.want {
				t.Fatalf("expected meta.per_page %d, got %d", tt.want, body.Meta.PerPage)
			}
			if body.Meta.Total != total {
				t.Fatalf("expected meta.total %d, got %d", total, body.Meta.Total)
			}
			wantLen := total
			if tt.want < wantLen {
				wantLen = tt.want
			}
			if len(body.Data) != wantLen {
				t.Fatalf("expected %d rows, got %d", wantLen, len(body.Data))
			}
		})
	}
}
func TestListTodos_PageNormalized(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  int
	}{
		{"zero", "?page=0", 1},
		{"negative", "?page=-5", 1},
		{"valid", "?page=2", 2},
		{"default", "", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeTodoStore{listTotal: 1}
			h := newTodoHandlerWithStore(testTodoConfig(), store)

			req := httptest.NewRequest("GET", "/api/v1/todos"+tt.query, nil)
			req = addUserToContext(req)
			rec := httptest.NewRecorder()

			h.List(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d", rec.Code)
			}
			if store.listFilter.Page != tt.want {
				t.Fatalf("expected repository page %d, got %d", tt.want, store.listFilter.Page)
			}

			var body struct {
				Meta model.TodoListMeta `json:"meta"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Meta.Page != tt.want {
				t.Fatalf("expected meta.page %d, got %d", tt.want, body.Meta.Page)
			}
		})
	}
}
