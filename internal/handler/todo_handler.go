package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/DevenWen/TodoDemo/internal/config"
	"github.com/DevenWen/TodoDemo/internal/middleware"
	"github.com/DevenWen/TodoDemo/internal/model"
	"github.com/DevenWen/TodoDemo/internal/repository"
)

const (
	defaultPerPage = 20
	maxPerPage     = 100
)

// TodoStore is the subset of repository operations the todo handler needs.
// It keeps the handler testable without a live database.
type TodoStore interface {
	List(ctx context.Context, filter repository.TodoFilter) ([]model.Todo, int, error)
	Create(ctx context.Context, userID string, req model.CreateTodoRequest) (*model.Todo, error)
	Update(ctx context.Context, id, userID string, req model.UpdateTodoRequest) (*model.Todo, error)
	Delete(ctx context.Context, id, userID string) error
}

// repositoryTodoStore implements TodoStore with the package-level repository.
type repositoryTodoStore struct{}

func (repositoryTodoStore) List(ctx context.Context, filter repository.TodoFilter) ([]model.Todo, int, error) {
	return repository.ListTodos(ctx, filter)
}

func (repositoryTodoStore) Create(ctx context.Context, userID string, req model.CreateTodoRequest) (*model.Todo, error) {
	return repository.CreateTodo(ctx, userID, req)
}

func (repositoryTodoStore) Update(ctx context.Context, id, userID string, req model.UpdateTodoRequest) (*model.Todo, error) {
	return repository.UpdateTodo(ctx, id, userID, req)
}

func (repositoryTodoStore) Delete(ctx context.Context, id, userID string) error {
	return repository.SoftDeleteTodo(ctx, id, userID)
}

// TodoHandler handles todo CRUD endpoints.
type TodoHandler struct {
	cfg   *config.Config
	store TodoStore
}

// NewTodoHandler creates a new TodoHandler backed by the database.
func NewTodoHandler(cfg *config.Config) *TodoHandler {
	return newTodoHandlerWithStore(cfg, repositoryTodoStore{})
}

// newTodoHandlerWithStore lets tests inject a fake store.
func newTodoHandlerWithStore(cfg *config.Config, store TodoStore) *TodoHandler {
	return &TodoHandler{cfg: cfg, store: store}
}

// List returns a paginated list of todos for the authenticated user.
func (h *TodoHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r.Context())

	filter := repository.TodoFilter{
		UserID:  userID,
		Page:    queryInt(r, "page", 1),
		PerPage: normalizePerPage(queryInt(r, "per_page", defaultPerPage)),
	}

	// Parse completed filter
	completedStr := r.URL.Query().Get("completed")
	if completedStr == "true" {
		t := true
		filter.Completed = &t
	} else if completedStr == "false" {
		f := false
		filter.Completed = &f
	}

	todos, total, err := h.store.List(r.Context(), filter)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, model.ErrCodeInternalError, "Failed to fetch todos", err)
		return
	}

	writeListResponse(w, http.StatusOK, todos, model.TodoListMeta{
		Page:    filter.Page,
		PerPage: filter.PerPage,
		Total:   total,
	})
}

// Create creates a new todo for the authenticated user.
func (h *TodoHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r.Context())

	var req model.CreateTodoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, model.ErrCodeValidationError, "Invalid request body", err)
		return
	}

	// Validate
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeError(w, r, http.StatusBadRequest, model.ErrCodeValidationError, "Title is required", nil)
		return
	}
	if len(req.Title) > 500 {
		writeError(w, r, http.StatusBadRequest, model.ErrCodeValidationError, "Title must be 500 characters or less", nil)
		return
	}

	todo, err := h.store.Create(r.Context(), userID, req)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, model.ErrCodeInternalError, "Failed to create todo", err)
		return
	}

	writeJSON(w, http.StatusCreated, todo)
}

// Update updates an existing todo by ID.
func (h *TodoHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r.Context())
	todoID := chi.URLParam(r, "id")

	var req model.UpdateTodoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, model.ErrCodeValidationError, "Invalid request body", err)
		return
	}

	// Validate optional fields
	if req.Title != nil {
		trimmed := strings.TrimSpace(*req.Title)
		if trimmed == "" {
			writeError(w, r, http.StatusBadRequest, model.ErrCodeValidationError, "Title cannot be empty", nil)
			return
		}
		if len(trimmed) > 500 {
			writeError(w, r, http.StatusBadRequest, model.ErrCodeValidationError, "Title must be 500 characters or less", nil)
			return
		}
		req.Title = &trimmed
	}

	todo, err := h.store.Update(r.Context(), todoID, userID, req)
	if err != nil {
		if repository.IsNotFoundError(err) {
			writeError(w, r, http.StatusNotFound, model.ErrCodeNotFound, "Todo not found", err)
			return
		}
		writeError(w, r, http.StatusInternalServerError, model.ErrCodeInternalError, "Failed to update todo", err)
		return
	}

	writeJSON(w, http.StatusOK, todo)
}

// Delete soft-deletes a todo by ID.
func (h *TodoHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r.Context())
	todoID := chi.URLParam(r, "id")

	err := h.store.Delete(r.Context(), todoID, userID)
	if err != nil {
		if repository.IsNotFoundError(err) {
			writeError(w, r, http.StatusNotFound, model.ErrCodeNotFound, "Todo not found", err)
			return
		}
		writeError(w, r, http.StatusInternalServerError, model.ErrCodeInternalError, "Failed to delete todo", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

// writeListResponse writes a response with data and meta fields.
func writeListResponse(w http.ResponseWriter, status int, data interface{}, meta model.TodoListMeta) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":  data,
		"meta":  meta,
		"error": nil,
	})
}

// normalizePerPage clamps a requested page size to the API contract.
func normalizePerPage(n int) int {
	if n < 1 {
		return defaultPerPage
	}
	if n > maxPerPage {
		return maxPerPage
	}
	return n
}

// queryInt parses an integer query parameter with a default fallback.
func queryInt(r *http.Request, key string, defaultVal int) int {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil || n < 1 {
		return defaultVal
	}
	return n
}
