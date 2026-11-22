package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
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
	LogLevel           string
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
		LogLevel:           getEnv("LOG_LEVEL", "debug"),
	}

	// Validate required config
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
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

// WarnDeploymentConfig logs the settings that work for local development but
// break a public deployment. It never fails the process: those defaults are
// intentional for local development.
func (c *Config) WarnDeploymentConfig(logger *slog.Logger) {
	if c.GitHubClientID == "" || c.GitHubClientSecret == "" {
		logger.Warn("GitHub OAuth app not configured; /auth/github/login will return 503",
			"hint", "set GITHUB_CLIENT_ID and GITHUB_CLIENT_SECRET",
			"client_id_set", c.GitHubClientID != "",
			"client_secret_set", c.GitHubClientSecret != "",
		)
	}

	warnLocalhostURL(logger, "FRONTEND_URL", c.FrontendURL)
	warnLocalhostURL(logger, "GITHUB_REDIRECT_URL", c.GitHubRedirectURL)
}

// warnLocalhostURL warns when a configured URL is loopback or not absolute.
func warnLocalhostURL(logger *slog.Logger, name, raw string) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		logger.Warn("config value is not a valid URL; a public deployment needs an absolute URL",
			"env", name,
			"value", raw,
		)
		return
	}

	switch strings.ToLower(u.Hostname()) {
	case "localhost", "127.0.0.1", "::1":
		logger.Warn("config still points at localhost; a public deployment needs a public URL",
			"env", name,
			"value", raw,
		)
	}
}
