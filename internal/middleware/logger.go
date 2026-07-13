package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// Logger is a custom structured logger middleware.
func Logger(logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			logger.InfoContext(r.Context(), "request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"duration", time.Since(start).String(),
				"bytes", ww.BytesWritten(),
			)
		})
	}
}

// contextKey is a type for context keys to avoid collisions.
type contextKey string

const UserIDKey contextKey = "user_id"
const UserLoginKey contextKey = "user_login"

// GetUserID extracts the user ID from the request context.
func GetUserID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(UserIDKey).(string)
	return id, ok
}

// GetUserLogin extracts the user login from the request context.
func GetUserLogin(ctx context.Context) (string, bool) {
	login, ok := ctx.Value(UserLoginKey).(string)
	return login, ok
}
