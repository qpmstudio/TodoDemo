// Package logging builds the application's structured logger from configuration.
package logging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// DefaultLevel is used when LOG_LEVEL is empty.
const DefaultLevel = "debug"

// ParseLevel converts a LOG_LEVEL value into a slog.Level.
// Accepted values (case-insensitive): debug, info, warn, warning, error.
func ParseLevel(level string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "", DefaultLevel:
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid LOG_LEVEL %q: want debug, info, warn or error", level)
	}
}

// New creates a JSON logger writing to stdout at the given level.
// Source locations are added at debug level, where they help local troubleshooting.
func New(level string) (*slog.Logger, error) {
	lvl, err := ParseLevel(level)
	if err != nil {
		return nil, err
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     lvl,
		AddSource: lvl <= slog.LevelDebug,
	})), nil
}

// loggerKey is the context key holding a request-scoped logger.
type loggerKey struct{}

// WithLogger stores a logger in ctx.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// Lookup returns the logger stored in ctx and reports whether one was set.
func Lookup(ctx context.Context) (*slog.Logger, bool) {
	logger, ok := ctx.Value(loggerKey{}).(*slog.Logger)
	return logger, ok && logger != nil
}

// FromContext returns the logger stored in ctx, falling back to slog.Default
// when the request did not pass through the logging middleware.
func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := Lookup(ctx); ok {
		return logger
	}
	return slog.Default()
}
