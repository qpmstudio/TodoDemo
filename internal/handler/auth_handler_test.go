package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DevenWen/TodoDemo/internal/config"
	"github.com/DevenWen/TodoDemo/internal/model"
)

func testConfig() *config.Config {
	return &config.Config{
		GitHubClientID:     "test-client-id",
		GitHubClientSecret: "test-client-secret",
		JWTSecret:          "test-jwt-secret",
		FrontendURL:        "http://localhost:5173",
		GitHubRedirectURL:  "http://localhost:8080/auth/github/callback",
	}
}

func TestGitHubLogin(t *testing.T) {
	cfg := testConfig()
	h := NewAuthHandler(cfg)

	req := httptest.NewRequest("GET", "/auth/github/login", nil)
	rec := httptest.NewRecorder()

	h.GitHubLogin(rec, req)

	resp := rec.Result()
	if resp.StatusCode != http.StatusFound {
		t.Errorf("expected 302, got %d", resp.StatusCode)
	}

	// Verify state cookie was set
	cookies := resp.Cookies()
	var stateCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "oauth_state" {
			stateCookie = c
			break
		}
	}
	if stateCookie == nil {
		t.Error("expected oauth_state cookie")
	}
	if stateCookie.HttpOnly != true {
		t.Error("state cookie should be HttpOnly")
	}

	// Verify redirect URL contains client ID
	location := resp.Header.Get("Location")
	if location == "" {
		t.Error("expected Location header")
	}
	if !contains(location, "github.com/login/oauth/authorize") {
		t.Error("redirect should go to GitHub authorize")
	}
	if !contains(location, "client_id=test-client-id") {
		t.Error("redirect should include client_id")
	}
}

func TestGitHubLogin_ConfiguresRedirectURI(t *testing.T) {
	cfg := testConfig()
	cfg.GitHubRedirectURL = "http://example.com/callback"
	h := NewAuthHandler(cfg)

	req := httptest.NewRequest("GET", "/auth/github/login", nil)
	rec := httptest.NewRecorder()

	h.GitHubLogin(rec, req)

	location := rec.Header().Get("Location")
	if !contains(location, "redirect_uri=http%3A%2F%2Fexample.com%2Fcallback") {
		t.Errorf("redirect should include encoded redirect_uri, got: %s", location)
	}
}

func TestGitHubCallback_MissingStateCookie(t *testing.T) {
	cfg := testConfig()
	h := NewAuthHandler(cfg)

	req := httptest.NewRequest("GET", "/auth/github/callback?code=test&state=abc", nil)
	rec := httptest.NewRecorder()

	h.GitHubCallback(rec, req)

	resp := rec.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}

	var apiResp model.APIResponse
	json.NewDecoder(resp.Body).Decode(&apiResp)
	if apiResp.Error == nil || apiResp.Error.Code != model.ErrCodeValidationError {
		t.Error("expected validation error")
	}
}

func TestGitHubCallback_MissingCode(t *testing.T) {
	cfg := testConfig()
	h := NewAuthHandler(cfg)

	req := httptest.NewRequest("GET", "/auth/github/callback?state=abc", nil)
	req.AddCookie(&http.Cookie{
		Name:  "oauth_state",
		Value: "abc",
	})
	rec := httptest.NewRecorder()

	h.GitHubCallback(rec, req)

	resp := rec.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestGitHubCallback_StateMismatch(t *testing.T) {
	cfg := testConfig()
	h := NewAuthHandler(cfg)

	req := httptest.NewRequest("GET", "/auth/github/callback?code=test&state=attacker", nil)
	req.AddCookie(&http.Cookie{
		Name:  "oauth_state",
		Value: "original",
	})
	rec := httptest.NewRecorder()

	h.GitHubCallback(rec, req)

	resp := rec.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
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
		GitHubLogin: "testlogin",
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
	writeError(rec, http.StatusNotFound, model.ErrCodeNotFound, "Not found")

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

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
