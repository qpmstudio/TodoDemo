// Package logging builds the application's structured logger from configuration.
package logging

import (
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
