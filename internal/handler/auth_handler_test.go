package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DevenWen/TodoDemo/internal/config"
	"github.com/DevenWen/TodoDemo/internal/logging"
	"github.com/DevenWen/TodoDemo/internal/model"
)

func testConfig() *config.Config {
	return &config.Config{
		JWTSecret:     "test-jwt-secret",
		FrontendURL:   "http://localhost:5173",
		ZitadelIssuer: "https://auth.example.com",
	}
}

func TestLogout(t *testing.T) {
	cfg := testConfig()
	h := NewAuthHandler(cfg)

	req := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	rec := httptest.NewRecorder()

	h.Logout(rec, req)

	resp := rec.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// Verify JWT cookie is cleared
	cookies := resp.Cookies()
	var jwtCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "jwt" {
			jwtCookie = c
			break
		}
	}
	if jwtCookie == nil {
		t.Error("expected jwt cookie to be set (cleared)")
	}
	if jwtCookie.MaxAge != -1 {
		t.Error("expected jwt cookie to be expired (MaxAge=-1)")
	}
}

func TestCreateJWT(t *testing.T) {
	cfg := testConfig()
	h := NewAuthHandler(cfg)

	user := &model.User{
		ID:          "test-user-id",
		DisplayName: "testlogin",
	}

	token, err := h.createJWT(user)
	if err != nil {
		t.Fatalf("failed to create JWT: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestGenerateRandomState(t *testing.T) {
	state1, err := generateRandomState()
	if err != nil {
		t.Fatalf("failed to generate state: %v", err)
	}
	if len(state1) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("expected 64-char state, got %d", len(state1))
	}

	state2, err := generateRandomState()
	if err != nil {
		t.Fatalf("failed to generate second state: %v", err)
	}
	if state1 == state2 {
		t.Error("state should be unique")
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	req = req.WithContext(logging.WithLogger(req.Context(), logger))
	writeError(rec, req, http.StatusNotFound, model.ErrCodeNotFound, "Not found", nil)

	resp := rec.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}

	var apiResp model.APIResponse
	json.NewDecoder(resp.Body).Decode(&apiResp)
	if apiResp.Error == nil {
		t.Fatal("expected error")
	}
	if apiResp.Error.Code != model.ErrCodeNotFound {
		t.Errorf("expected NOT_FOUND, got %s", apiResp.Error.Code)
	}
	if apiResp.Data != nil {
		t.Error("expected null data")
	}
}

func TestSetJWTCookie(t *testing.T) {
	cfg := testConfig()
	h := NewAuthHandler(cfg)

	rec := httptest.NewRecorder()
	h.setJWTCookie(rec, "test-token")

	cookies := rec.Result().Cookies()
	var jwtCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "jwt" {
			jwtCookie = c
			break
		}
	}
	if jwtCookie == nil {
		t.Fatal("expected jwt cookie")
	}
	if jwtCookie.Value != "test-token" {
		t.Errorf("expected token value test-token, got %s", jwtCookie.Value)
	}
	if jwtCookie.HttpOnly != true {
		t.Error("expected HttpOnly cookie")
	}
	if jwtCookie.MaxAge != 7*24*60*60 {
		t.Errorf("expected 7-day MaxAge, got %d", jwtCookie.MaxAge)
	}
}

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusCreated, map[string]string{"status": "created"})

	resp := rec.Result()
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}

	var apiResp model.APIResponse
	json.NewDecoder(resp.Body).Decode(&apiResp)
	if apiResp.Error != nil {
		t.Error("expected no error")
	}
}
