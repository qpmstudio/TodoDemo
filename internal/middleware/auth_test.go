package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DevenWen/TodoDemo/internal/config"
)

func TestAuthMissingCookie(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	mw := Auth(cfg)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	})

	req := httptest.NewRequest("GET", "/api/v1/todos", nil)
	rec := httptest.NewRecorder()

	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestGetUserID(t *testing.T) {
	ctx := context.WithValue(t.Context(), UserIDKey, "test-user-id")
	id, ok := GetUserID(ctx)
	if !ok {
		t.Error("expected to get user ID")
	}
	if id != "test-user-id" {
		t.Errorf("expected 'test-user-id', got '%s'", id)
	}
}

func TestGetUserIDMissing(t *testing.T) {
	_, ok := GetUserID(t.Context())
	if ok {
		t.Error("expected no user ID")
	}
}

func TestGetUserLogin(t *testing.T) {
	ctx := context.WithValue(t.Context(), UserLoginKey, "test-login")
	login, ok := GetUserLogin(ctx)
	if !ok {
		t.Error("expected to get user login")
	}
	if login != "test-login" {
		t.Errorf("expected 'test-login', got '%s'", login)
	}
}
