package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/DevenWen/TodoDemo/internal/database"
	"github.com/DevenWen/TodoDemo/internal/model"
)

// TodoFilter holds query parameters for listing todos.
type TodoFilter struct {
	UserID    string
	Completed *bool
	Page      int
	PerPage   int
}

// ListTodos retrieves a paginated list of todos for a user.
func ListTodos(ctx context.Context, filter TodoFilter) ([]model.Todo, int, error) {
	// Default pagination
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PerPage < 1 || filter.PerPage > 100 {
		filter.PerPage = 20
	}

	offset := (filter.Page - 1) * filter.PerPage

	// Build query
	where := "WHERE user_id = $1 AND deleted_at IS NULL"
	args := []interface{}{filter.UserID}
	argIdx := 2

	if filter.Completed != nil {
		where += " AND completed = $" + itoa(argIdx)
		args = append(args, *filter.Completed)
		argIdx++
	}

	// Count total
	var total int
	countQuery := "SELECT COUNT(*) FROM todos " + where
	if err := database.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Select todos
	selectQuery := "SELECT id, title, description, completed, completed_at, created_at, updated_at FROM todos " +
		where + " ORDER BY created_at DESC LIMIT $" + itoa(argIdx) + " OFFSET $" + itoa(argIdx+1)
	args = append(args, filter.PerPage, offset)

	rows, err := database.Pool.Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var todos []model.Todo
	for rows.Next() {
		var t model.Todo
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Completed, &t.CompletedAt, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, 0, err
		}
		t.UserID = filter.UserID
		todos = append(todos, t)
	}

	if todos == nil {
		todos = []model.Todo{}
	}

	return todos, total, rows.Err()
}

// CreateTodo inserts a new todo for a user.
func CreateTodo(ctx context.Context, userID string, req model.CreateTodoRequest) (*model.Todo, error) {
	now := time.Now()
	todo := &model.Todo{
		UserID:      userID,
		Title:       req.Title,
		Description: req.Description,
		Completed:   false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	err := database.Pool.QueryRow(ctx,
		`INSERT INTO todos (user_id, title, description, completed, created_at, updated_at)
		 VALUES ($1, $2, $3, false, $4, $5)
		 RETURNING id`,
		todo.UserID, todo.Title, todo.Description, todo.CreatedAt, todo.UpdatedAt,
	).Scan(&todo.ID)

	return todo, err
}

// GetTodoByID retrieves a single todo by ID, ensuring it belongs to the user.
func GetTodoByID(ctx context.Context, id string, userID string) (*model.Todo, error) {
	todo := &model.Todo{UserID: userID}
	err := database.Pool.QueryRow(ctx,
		`SELECT id, title, description, completed, completed_at, created_at, updated_at
		 FROM todos WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`,
		id, userID,
	).Scan(&todo.ID, &todo.Title, &todo.Description, &todo.Completed, &todo.CompletedAt, &todo.CreatedAt, &todo.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return todo, nil
}

// UpdateTodo updates an existing todo's fields.
func UpdateTodo(ctx context.Context, id string, userID string, req model.UpdateTodoRequest) (*model.Todo, error) {
	// Fetch existing todo first
	todo, err := GetTodoByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	// Apply updates
	if req.Title != nil {
		todo.Title = *req.Title
	}
	if req.Description != nil {
		todo.Description = *req.Description
	}
	if req.Completed != nil {
		todo.Completed = *req.Completed
		if *req.Completed {
			now := time.Now()
			todo.CompletedAt = &now
		} else {
			todo.CompletedAt = nil
		}
	}

	todo.UpdatedAt = time.Now()

	err = database.Pool.QueryRow(ctx,
		`UPDATE todos SET title = $1, description = $2, completed = $3, completed_at = $4, updated_at = $5
		 WHERE id = $6 AND user_id = $7 AND deleted_at IS NULL
		 RETURNING id, title, description, completed, completed_at, created_at, updated_at`,
		todo.Title, todo.Description, todo.Completed, todo.CompletedAt, todo.UpdatedAt, id, userID,
	).Scan(&todo.ID, &todo.Title, &todo.Description, &todo.Completed, &todo.CompletedAt, &todo.CreatedAt, &todo.UpdatedAt)

	return todo, err
}

// SoftDeleteTodo sets deleted_at on a todo.
func SoftDeleteTodo(ctx context.Context, id string, userID string) error {
	result, err := database.Pool.Exec(ctx,
		`UPDATE todos SET deleted_at = $1 WHERE id = $2 AND user_id = $3 AND deleted_at IS NULL`,
		time.Now(), id, userID,
	)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return errNotFound
	}

	return nil
}

var errNotFound = &notFoundError{}

type notFoundError struct{}

func (e *notFoundError) Error() string { return "todo not found" }

// IsNotFoundError checks if the error is a not-found error.
func IsNotFoundError(err error) bool {
	_, ok := err.(*notFoundError)
	return ok
}

func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}
