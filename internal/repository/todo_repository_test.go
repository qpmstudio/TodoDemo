package repository

import (
	"errors"
	"testing"

	"github.com/DevenWen/TodoDemo/internal/model"
)

func TestIsNotFoundError(t *testing.T) {
	err := &notFoundError{}
	if !IsNotFoundError(err) {
		t.Error("expected IsNotFoundError to return true")
	}

	if IsNotFoundError(nil) {
		t.Error("expected IsNotFoundError to return false for nil")
	}

	plainErr := errors.New("some other error")
	if IsNotFoundError(plainErr) {
		t.Error("expected IsNotFoundError to return false for other types")
	}
}

func TestNotFoundError_Message(t *testing.T) {
	err := &notFoundError{}
	if err.Error() != "todo not found" {
		t.Errorf("expected 'todo not found', got '%s'", err.Error())
	}
}

func TestTodoFilter_Defaults(t *testing.T) {
	filter := TodoFilter{
		UserID:  "test-user",
		Page:    0,
		PerPage: 0,
	}

	// These defaults are applied in ListTodos
	if filter.UserID != "test-user" {
		t.Error("expected UserID to be set")
	}
}

func TestTodoModel(t *testing.T) {
	todo := model.Todo{
		ID:          "id-1",
		Title:       "Test todo",
		Description: "A test description",
		Completed:   false,
	}

	if todo.ID != "id-1" {
		t.Error("expected ID to be set")
	}
	if todo.Title != "Test todo" {
		t.Error("expected Title to be set")
	}
	if todo.Description != "A test description" {
		t.Error("expected Description to be set")
	}
	if todo.Completed != false {
		t.Error("expected Completed to be false")
	}
}

func TestCreateTodoRequest(t *testing.T) {
	req := model.CreateTodoRequest{
		Title:       "Buy coffee",
		Description: "Colombian, medium roast",
	}

	if req.Title != "Buy coffee" {
		t.Errorf("expected 'Buy coffee', got '%s'", req.Title)
	}
}

func TestUpdateTodoRequest(t *testing.T) {
	title := "Updated title"
	desc := "Updated description"
	completed := true

	req := model.UpdateTodoRequest{
		Title:       &title,
		Description: &desc,
		Completed:   &completed,
	}

	if *req.Title != "Updated title" {
		t.Errorf("expected 'Updated title', got '%s'", *req.Title)
	}
	if *req.Description != "Updated description" {
		t.Errorf("expected 'Updated description', got '%s'", *req.Description)
	}
	if *req.Completed != true {
		t.Error("expected Completed to be true")
	}
}
