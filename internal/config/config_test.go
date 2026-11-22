package config

import (
	"bytes"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	// Set required env vars
	os.Setenv("DATABASE_URL", "postgres://localhost:5432/tododemo")
	os.Setenv("GITHUB_CLIENT_ID", "test-client-id")
	os.Setenv("GITHUB_CLIENT_SECRET", "test-client-secret")
	os.Setenv("JWT_SECRET", "test-jwt-secret")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("GITHUB_CLIENT_ID")
		os.Unsetenv("GITHUB_CLIENT_SECRET")
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("PORT")
		os.Unsetenv("FRONTEND_URL")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("expected default port 8080, got %s", cfg.Port)
	}

	if cfg.DatabaseURL != "postgres://localhost:5432/tododemo" {
		t.Errorf("unexpected DatabaseURL: %s", cfg.DatabaseURL)
	}

	if cfg.FrontendURL != "http://localhost:5173" {
		t.Errorf("expected default FrontendURL, got %s", cfg.FrontendURL)
	}
}

func TestLoadCustomPort(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost:5432/tododemo")
	os.Setenv("GITHUB_CLIENT_ID", "test-client-id")
	os.Setenv("GITHUB_CLIENT_SECRET", "test-client-secret")
	os.Setenv("JWT_SECRET", "test-jwt-secret")
	os.Setenv("PORT", "3000")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("GITHUB_CLIENT_ID")
		os.Unsetenv("GITHUB_CLIENT_SECRET")
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("PORT")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Port != "3000" {
		t.Errorf("expected port 3000, got %s", cfg.Port)
	}
}

func TestLoadLogLevelDefaultsToDebug(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/tododemo")
	t.Setenv("JWT_SECRET", "test")
	t.Setenv("LOG_LEVEL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected default log level debug, got %q", cfg.LogLevel)
	}
}

func TestLoadCustomLogLevel(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/tododemo")
	t.Setenv("JWT_SECRET", "test")
	t.Setenv("LOG_LEVEL", "warn")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("expected log level warn, got %q", cfg.LogLevel)
	}
}

func TestLoadMissingDatabaseURL(t *testing.T) {
	os.Setenv("GITHUB_CLIENT_ID", "test")
	os.Setenv("GITHUB_CLIENT_SECRET", "test")
	os.Setenv("JWT_SECRET", "test")
	defer func() {
		os.Unsetenv("GITHUB_CLIENT_ID")
		os.Unsetenv("GITHUB_CLIENT_SECRET")
		os.Unsetenv("JWT_SECRET")
	}()

	_, err := Load()
	if err == nil {
		t.Error("expected error for missing DATABASE_URL")
	}
}

func TestLoadMissingJWTSecret(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost/tododemo")
	os.Setenv("GITHUB_CLIENT_ID", "test")
	os.Setenv("GITHUB_CLIENT_SECRET", "test")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("GITHUB_CLIENT_ID")
		os.Unsetenv("GITHUB_CLIENT_SECRET")
	}()

	_, err := Load()
	if err == nil {
		t.Error("expected error for missing JWT_SECRET")
	}
}

func TestWarnDeploymentConfig(t *testing.T) {
	publicURL := "https://todo.qpmstudio.dpdns.org"
	configured := Config{
		GitHubClientID:     "client-id",
		GitHubClientSecret: "client-secret",
		FrontendURL:        publicURL,
		GitHubRedirectURL:  publicURL + "/auth/github/callback",
	}

	tests := []struct {
		name       string
		cfg        Config
		wantLogs   []string
		wantSilent bool
	}{
		{
			name:       "public config with oauth set is silent",
			cfg:        configured,
			wantSilent: true,
		},
		{
			name: "missing oauth credentials warn",
			cfg: Config{
				FrontendURL:       publicURL,
				GitHubRedirectURL: publicURL + "/auth/github/callback",
			},
			wantLogs: []string{"OAuth app not configured", "503"},
		},
		{
			name: "localhost urls warn",
			cfg: Config{
				GitHubClientID:     "client-id",
				GitHubClientSecret: "client-secret",
				FrontendURL:        "http://localhost:5173",
				GitHubRedirectURL:  "http://127.0.0.1:8080/auth/github/callback",
			},
			wantLogs: []string{"localhost", "FRONTEND_URL", "GITHUB_REDIRECT_URL"},
		},
		{
			name: "ipv6 loopback warns",
			cfg: Config{
				GitHubClientID:     "client-id",
				GitHubClientSecret: "client-secret",
				FrontendURL:        publicURL,
				GitHubRedirectURL:  "http://[::1]:8080/auth/github/callback",
			},
			wantLogs: []string{"localhost", "GITHUB_REDIRECT_URL"},
		},
		{
			name: "invalid url warns",
			cfg: Config{
				GitHubClientID:     "client-id",
				GitHubClientSecret: "client-secret",
				FrontendURL:        "://not-a-url",
				GitHubRedirectURL:  publicURL + "/auth/github/callback",
			},
			wantLogs: []string{"not a valid URL", "FRONTEND_URL"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn}))

			tt.cfg.WarnDeploymentConfig(logger)

			got := logs.String()
			if tt.wantSilent {
				if got != "" {
					t.Errorf("expected no warnings, got %q", got)
				}
				return
			}
			for _, want := range tt.wantLogs {
				if !strings.Contains(got, want) {
					t.Errorf("expected log to contain %q, got %q", want, got)
				}
			}
		})
	}
}

func TestRegisterURLDefaultAndOverride(t *testing.T) {
	cfg := &Config{ZitadelIssuer: "https://auth.example.com/"}
	if got := cfg.RegisterURL(); got != "https://auth.example.com/ui/v2/login/register" {
		t.Errorf("default register URL = %q", got)
	}
	cfg.ZitadelRegisterURL = "https://id.example.com/signup"
	if got := cfg.RegisterURL(); got != "https://id.example.com/signup" {
		t.Errorf("override register URL = %q", got)
	}
	if got := (&Config{}).RegisterURL(); got != "" {
		t.Errorf("register URL without issuer = %q, want empty", got)
	}
}

func TestVerifyURLDefaultAndOverride(t *testing.T) {
	cfg := &Config{ZitadelIssuer: "https://auth.example.com/"}
	if got := cfg.VerifyURL(); got != "https://auth.example.com/ui/v2/login" {
		t.Errorf("default verify URL = %q", got)
	}
	cfg.ZitadelVerifyURL = "https://id.example.com/verify"
	if got := cfg.VerifyURL(); got != "https://id.example.com/verify" {
		t.Errorf("override verify URL = %q", got)
	}
	if got := (&Config{}).VerifyURL(); got != "" {
		t.Errorf("verify URL without issuer = %q, want empty", got)
	}
}
