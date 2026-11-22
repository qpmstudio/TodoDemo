package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestRegister_Retired(t *testing.T) {
	cfg := testConfig()
	cfg.ZitadelRegisterURL = "https://auth.example.com/ui/v2/login/register"
	h := NewAuthHandler(cfg)

	bodies := []string{
		`{"email":"user@example.com","password":"password123"}`,
		`{"email":"notanemail","password":"password123"}`,
		`{"email":"user@example.com","password":"short"}`,
		`not json`,
	}
	for _, body := range bodies {
		req := httptest.NewRequest("POST", "/auth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.Register(rec, req)

		resp := rec.Result()
		if resp.StatusCode != http.StatusGone {
			t.Errorf("body %q: expected 410, got %d", body, resp.StatusCode)
		}
		if got := errorCode(t, rec); got != model.ErrCodeRegistrationMoved {
			t.Errorf("body %q: code = %q, want %q", body, got, model.ErrCodeRegistrationMoved)
		}
		if !strings.Contains(rec.Body.String(), cfg.ZitadelRegisterURL) {
			t.Errorf("body %q: response should point at the ZITADEL register page: %s", body, rec.Body.String())
		}
		for _, c := range resp.Cookies() {
			if c.Name == "jwt" && c.Value != "" {
				t.Errorf("body %q: retired register must not set a jwt cookie", body)
			}
		}
	}
}

func TestRegister_RetiredWithoutZitadel(t *testing.T) {
	cfg := testConfig()
	cfg.ZitadelIssuer = ""
	h := NewAuthHandler(cfg)

	req := httptest.NewRequest("POST", "/auth/register", strings.NewReader(`{"email":"user@example.com","password":"password123"}`))
	rec := httptest.NewRecorder()
	h.Register(rec, req)

	if rec.Code != http.StatusGone {
		t.Fatalf("status = %d, want 410", rec.Code)
	}
	if got := errorCode(t, rec); got != model.ErrCodeRegistrationMoved {
		t.Errorf("code = %q, want %q", got, model.ErrCodeRegistrationMoved)
	}
}

func TestLogin_Retired(t *testing.T) {
	cfg := testConfig()
	cfg.ZitadelVerifyURL = "https://auth.example.com/ui/v2/login"
	h := NewAuthHandler(cfg)

	bodies := []string{
		`{"email":"user@example.com","password":"password123"}`,
		`{"email":"notanemail","password":"password123"}`,
		`not json`,
		``,
	}
	for _, body := range bodies {
		req := httptest.NewRequest("POST", "/auth/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.Login(rec, req)

		resp := rec.Result()
		if resp.StatusCode != http.StatusGone {
			t.Errorf("body %q: expected 410, got %d", body, resp.StatusCode)
		}
		if got := errorCode(t, rec); got != model.ErrCodeLoginMoved {
			t.Errorf("body %q: code = %q, want %q", body, got, model.ErrCodeLoginMoved)
		}
		if !strings.Contains(rec.Body.String(), cfg.ZitadelVerifyURL) {
			t.Errorf("body %q: response should point at the ZITADEL login: %s", body, rec.Body.String())
		}
		for _, c := range resp.Cookies() {
			if c.Name == "jwt" && c.Value != "" {
				t.Errorf("body %q: retired login must not set a jwt cookie", body)
			}
		}
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
