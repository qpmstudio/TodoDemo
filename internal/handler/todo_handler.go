package handler

import (
	"encoding/json"
	"net/http"

	"github.com/DevenWen/TodoDemo/internal/config"
	"github.com/DevenWen/TodoDemo/internal/model"
)

// TodoHandler handles todo CRUD endpoints.
type TodoHandler struct {
	cfg *config.Config
}

// NewTodoHandler creates a new TodoHandler.
func NewTodoHandler(cfg *config.Config) *TodoHandler {
	return &TodoHandler{cfg: cfg}
}

// List returns a paginated list of todos.
func (h *TodoHandler) List(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, []model.Todo{}, model.TodoListMeta{Page: 1, PerPage: 20, Total: 0})
}

// Create creates a new todo.
func (h *TodoHandler) Create(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "not implemented"})
}

// Update updates an existing todo.
func (h *TodoHandler) Update(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"message": "not implemented"}, nil)
}

// Delete soft-deletes a todo.
func (h *TodoHandler) Delete(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func respondJSON(w http.ResponseWriter, status int, data interface{}, meta interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := model.APIResponse{Data: data}
	if meta != nil {
		// For list responses, we need to include meta
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data":  data,
			"meta":  meta,
			"error": nil,
		})
		return
	}

	json.NewEncoder(w).Encode(resp)
}
