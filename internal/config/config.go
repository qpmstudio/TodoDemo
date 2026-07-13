package config

import (
	"fmt"
	"os"
)

// Config holds all configuration for the application.
type Config struct {
	Port               string
	DatabaseURL        string
	GitHubClientID     string
	GitHubClientSecret string
	JWTSecret          string
	FrontendURL        string
	GitHubRedirectURL  string
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		Port:               getEnv("PORT", "8080"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		GitHubClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		JWTSecret:          os.Getenv("JWT_SECRET"),
		FrontendURL:        getEnv("FRONTEND_URL", "http://localhost:5173"),
		GitHubRedirectURL:  getEnv("GITHUB_REDIRECT_URL", "http://localhost:8080/auth/github/callback"),
	}

	// Validate required config
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.GitHubClientID == "" {
		return nil, fmt.Errorf("GITHUB_CLIENT_ID is required")
	}
	if cfg.GitHubClientSecret == "" {
		return nil, fmt.Errorf("GITHUB_CLIENT_SECRET is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
