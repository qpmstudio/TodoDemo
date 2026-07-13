package handler

import (
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

// TodoHandler handles todo CRUD endpoints.
type TodoHandler struct {
	cfg *config.Config
}

// NewTodoHandler creates a new TodoHandler.
func NewTodoHandler(cfg *config.Config) *TodoHandler {
	return &TodoHandler{cfg: cfg}
}

// List returns a paginated list of todos for the authenticated user.
func (h *TodoHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r.Context())

	filter := repository.TodoFilter{
		UserID:  userID,
		Page:    queryInt(r, "page", 1),
		PerPage: queryInt(r, "per_page", 20),
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

	todos, total, err := repository.ListTodos(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, model.ErrCodeInternalError, "Failed to fetch todos")
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
		writeError(w, http.StatusBadRequest, model.ErrCodeValidationError, "Invalid request body")
		return
	}

	// Validate
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, model.ErrCodeValidationError, "Title is required")
		return
	}
	if len(req.Title) > 500 {
		writeError(w, http.StatusBadRequest, model.ErrCodeValidationError, "Title must be 500 characters or less")
		return
	}

	todo, err := repository.CreateTodo(r.Context(), userID, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, model.ErrCodeInternalError, "Failed to create todo")
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
		writeError(w, http.StatusBadRequest, model.ErrCodeValidationError, "Invalid request body")
		return
	}

	// Validate optional fields
	if req.Title != nil {
		trimmed := strings.TrimSpace(*req.Title)
		if trimmed == "" {
			writeError(w, http.StatusBadRequest, model.ErrCodeValidationError, "Title cannot be empty")
			return
		}
		if len(trimmed) > 500 {
			writeError(w, http.StatusBadRequest, model.ErrCodeValidationError, "Title must be 500 characters or less")
			return
		}
		req.Title = &trimmed
	}

	todo, err := repository.UpdateTodo(r.Context(), todoID, userID, req)
	if err != nil {
		if repository.IsNotFoundError(err) {
			writeError(w, http.StatusNotFound, model.ErrCodeNotFound, "Todo not found")
			return
		}
		writeError(w, http.StatusInternalServerError, model.ErrCodeInternalError, "Failed to update todo")
		return
	}

	writeJSON(w, http.StatusOK, todo)
}

// Delete soft-deletes a todo by ID.
func (h *TodoHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r.Context())
	todoID := chi.URLParam(r, "id")

	err := repository.SoftDeleteTodo(r.Context(), todoID, userID)
	if err != nil {
		if repository.IsNotFoundError(err) {
			writeError(w, http.StatusNotFound, model.ErrCodeNotFound, "Todo not found")
			return
		}
		writeError(w, http.StatusInternalServerError, model.ErrCodeInternalError, "Failed to delete todo")
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
