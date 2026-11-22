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
	Port        string
	DatabaseURL string
	JWTSecret   string
	FrontendURL string
	LogLevel    string

	// ZITADEL is the OIDC provider for the hosted login. It is optional: when
	// ZITADEL_CLIENT_ID is empty the login page hides the ZITADEL button and
	// /auth/zitadel/login returns 503.
	ZitadelIssuer        string
	ZitadelClientID      string
	ZitadelClientSecret  string
	ZitadelRedirectURL   string
	ZitadelPostLogoutURL string
	ZitadelRegisterURL   string
	ZitadelVerifyURL     string
}

// ZitadelEnabled reports whether ZITADEL login has enough configuration to
// start an authorization request.
func (c *Config) ZitadelEnabled() bool {
	return c.ZitadelIssuer != "" && c.ZitadelClientID != "" && c.ZitadelRedirectURL != ""
}

// RegisterURL is where new users create their account. Registration and email
// verification live in ZITADEL; TodoDemo no longer accepts self-service
// sign-ups. It defaults to the hosted login's registration page.
func (c *Config) RegisterURL() string {
	if c.ZitadelRegisterURL != "" {
		return c.ZitadelRegisterURL
	}
	if c.ZitadelIssuer == "" {
		return ""
	}
	return strings.TrimRight(c.ZitadelIssuer, "/") + "/ui/v2/login/register"
}

// VerifyURL is where a signed-in user finishes email verification in ZITADEL.
// It defaults to the hosted login entry, which is safe to open without the
// one-time code from the verification mail.
func (c *Config) VerifyURL() string {
	if c.ZitadelVerifyURL != "" {
		return c.ZitadelVerifyURL
	}
	if c.ZitadelIssuer == "" {
		return ""
	}
	return strings.TrimRight(c.ZitadelIssuer, "/") + "/ui/v2/login"
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		FrontendURL: getEnv("FRONTEND_URL", "http://localhost:5173"),
		LogLevel:    getEnv("LOG_LEVEL", "debug"),

		ZitadelIssuer:        getEnv("ZITADEL_ISSUER", "https://auth.qpmstudio.dpdns.org"),
		ZitadelClientID:      os.Getenv("ZITADEL_CLIENT_ID"),
		ZitadelClientSecret:  os.Getenv("ZITADEL_CLIENT_SECRET"),
		ZitadelRedirectURL:   getEnv("ZITADEL_REDIRECT_URL", "http://localhost:8080/auth/zitadel/callback"),
		ZitadelPostLogoutURL: os.Getenv("ZITADEL_POST_LOGOUT_URL"),
		ZitadelRegisterURL:   os.Getenv("ZITADEL_REGISTER_URL"),
		ZitadelVerifyURL:     os.Getenv("ZITADEL_VERIFY_URL"),
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
	warnLocalhostURL(logger, "FRONTEND_URL", c.FrontendURL)

	if c.ZitadelClientID == "" {
		logger.Info("ZITADEL login not configured; /auth/zitadel/login will return 503",
			"hint", "set ZITADEL_CLIENT_ID (and optionally ZITADEL_CLIENT_SECRET)")
		return
	}
	if c.ZitadelRedirectURL == "" {
		logger.Warn("ZITADEL_CLIENT_ID is set but ZITADEL_REDIRECT_URL is empty; ZITADEL login stays disabled")
	}
	if c.ZitadelClientSecret == "" {
		logger.Warn("ZITADEL_CLIENT_SECRET is empty; using the public-client PKCE flow")
	}
	warnLocalhostURL(logger, "ZITADEL_REDIRECT_URL", c.ZitadelRedirectURL)
	if c.ZitadelPostLogoutURL != "" {
		warnLocalhostURL(logger, "ZITADEL_POST_LOGOUT_URL", c.ZitadelPostLogoutURL)
	}
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
