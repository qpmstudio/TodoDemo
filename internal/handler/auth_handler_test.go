package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/DevenWen/TodoDemo/internal/config"
	"github.com/DevenWen/TodoDemo/internal/logging"
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

	// Verify the redirect points at GitHub with the configured OAuth parameters.
	location := resp.Header.Get("Location")
	if location == "" {
		t.Fatal("expected Location header")
	}
	authURL, err := url.Parse(location)
	if err != nil {
		t.Fatalf("Location is not a valid URL: %v", err)
	}
	if authURL.Scheme != "https" || authURL.Host != "github.com" || authURL.Path != "/login/oauth/authorize" {
		t.Errorf("redirect should go to GitHub authorize, got %s", location)
	}
	query := authURL.Query()
	if got := query.Get("client_id"); got != cfg.GitHubClientID {
		t.Errorf("expected client_id %q, got %q", cfg.GitHubClientID, got)
	}
	if got := query.Get("redirect_uri"); got != cfg.GitHubRedirectURL {
		t.Errorf("expected redirect_uri %q, got %q", cfg.GitHubRedirectURL, got)
	}
	if got := query.Get("scope"); got != "read:user" {
		t.Errorf("expected scope read:user, got %q", got)
	}
	if query.Get("state") == "" {
		t.Error("expected non-empty state in redirect")
	}
}

func TestGitHubLogin_Returns503WhenOAuthNotConfigured(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*config.Config)
	}{
		{"missing client id", func(c *config.Config) { c.GitHubClientID = "" }},
		{"missing client secret", func(c *config.Config) { c.GitHubClientSecret = "" }},
		{"missing both", func(c *config.Config) {
			c.GitHubClientID = ""
			c.GitHubClientSecret = ""
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig()
			tt.mutate(cfg)
			h := NewAuthHandler(cfg)

			var logs bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn}))
			req := httptest.NewRequest("GET", "/auth/github/login", nil)
			req = req.WithContext(logging.WithLogger(req.Context(), logger))
			rec := httptest.NewRecorder()

			h.GitHubLogin(rec, req)

			resp := rec.Result()
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusServiceUnavailable {
				t.Errorf("expected 503, got %d", resp.StatusCode)
			}
			if location := resp.Header.Get("Location"); location != "" {
				t.Errorf("must not redirect to GitHub, got Location: %s", location)
			}
			for _, cookie := range resp.Cookies() {
				if cookie.Name == "oauth_state" {
					t.Error("must not set oauth_state cookie when OAuth is not configured")
				}
			}

			var apiResp model.APIResponse
			if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if apiResp.Error == nil {
				t.Fatal("expected an error response")
			}
			if apiResp.Error.Code != model.ErrCodeServiceUnavailable {
				t.Errorf("expected %s, got %s", model.ErrCodeServiceUnavailable, apiResp.Error.Code)
			}
			if !strings.Contains(apiResp.Error.Message, "not configured") {
				t.Errorf("expected a clear configuration message, got %q", apiResp.Error.Message)
			}

			logged := logs.String()
			if !strings.Contains(logged, `"level":"WARN"`) {
				t.Errorf("expected a warn log, got %q", logged)
			}
			if !strings.Contains(logged, "OAuth app not configured") {
				t.Errorf("expected the warn log to explain the missing config, got %q", logged)
			}
		})
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

func TestRegister_InvalidEmail(t *testing.T) {
	cfg := testConfig()
	h := NewAuthHandler(cfg)

	tests := []struct {
		name  string
		email string
	}{
		{"missing @", "notanemail"},
		{"empty", ""},
		{"no domain", "user@"},
		{"no user", "@domain.com"},
		{"with spaces", "user @domain.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"email":"` + tt.email + `","password":"password123"}`
			req := httptest.NewRequest("POST", "/auth/register", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			h.Register(rec, req)

			resp := rec.Result()
			if resp.StatusCode != http.StatusUnprocessableEntity {
				t.Errorf("expected 422, got %d", resp.StatusCode)
			}

			var apiResp model.APIResponse
			json.NewDecoder(resp.Body).Decode(&apiResp)
			if apiResp.Error == nil {
				t.Fatal("expected error response")
			}
			if apiResp.Error.Code != model.ErrCodeValidationError {
				t.Errorf("expected VALIDATION_ERROR, got %s", apiResp.Error.Code)
			}
		})
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	cfg := testConfig()
	h := NewAuthHandler(cfg)

	tests := []struct {
		name     string
		password string
	}{
		{"empty password", ""},
		{"1 char", "a"},
		{"7 chars", "1234567"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"email":"user@example.com","password":"` + tt.password + `"}`
			req := httptest.NewRequest("POST", "/auth/register", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			h.Register(rec, req)

			resp := rec.Result()
			if resp.StatusCode != http.StatusUnprocessableEntity {
				t.Errorf("expected 422, got %d", resp.StatusCode)
			}

			var apiResp model.APIResponse
			json.NewDecoder(resp.Body).Decode(&apiResp)
			if apiResp.Error == nil {
				t.Fatal("expected error response")
			}
			if apiResp.Error.Code != model.ErrCodeValidationError {
				t.Errorf("expected VALIDATION_ERROR, got %s", apiResp.Error.Code)
			}
		})
	}
}

func TestRegister_InvalidJSON(t *testing.T) {
	cfg := testConfig()
	h := NewAuthHandler(cfg)

	req := httptest.NewRequest("POST", "/auth/register", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	resp := rec.Result()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", resp.StatusCode)
	}
}

func TestLogin_InvalidJSON(t *testing.T) {
	cfg := testConfig()
	h := NewAuthHandler(cfg)

	req := httptest.NewRequest("POST", "/auth/login", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	resp := rec.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestLogin_NonexistentUser(t *testing.T) {
	t.Skip("requires database connection")
}

func TestRegister_Success(t *testing.T) {
	t.Skip("requires database connection")
}

func TestLogin_Success(t *testing.T) {
	t.Skip("requires database connection")
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
