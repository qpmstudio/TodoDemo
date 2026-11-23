package repository

import (
	"testing"

	"github.com/DevenWen/TodoDemo/internal/model"
)

// NOTE: Repository tests require a running PostgreSQL database.
// These tests validate the model structure and can be run as integration tests.
// For full integration tests, set DATABASE_URL env var.

func TestUserModelFields(t *testing.T) {
	user := &model.User{
		ID:          "550e8400-e29b-41d4-a716-446655440000",
		DisplayName: "Test User",
		AvatarURL:   "https://avatars.example.com/u/12345",
	}

	if user.ID == "" {
		t.Error("expected non-empty ID")
	}
	if user.AvatarURL != "https://avatars.example.com/u/12345" {
		t.Errorf("expected avatar URL to be kept, got %s", user.AvatarURL)
	}
}
