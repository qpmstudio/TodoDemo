package config

import (
	"os"
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

func TestLoadMissingGitHubClientID(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost/tododemo")
	os.Setenv("GITHUB_CLIENT_SECRET", "test")
	os.Setenv("JWT_SECRET", "test")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("GITHUB_CLIENT_SECRET")
		os.Unsetenv("JWT_SECRET")
	}()

	_, err := Load()
	if err == nil {
		t.Error("expected error for missing GITHUB_CLIENT_ID")
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
