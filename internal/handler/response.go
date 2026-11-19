package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/DevenWen/TodoDemo/internal/middleware"
	"github.com/DevenWen/TodoDemo/internal/model"
)

// writeJSON writes a standard API success response.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(model.APIResponse{Data: data})
}

// writeError writes a standard API error response.
//
// The client only receives code and message. The full error context (cause,
// request metadata and, for 5xx, a stack trace) goes to the server log so
// issues can be diagnosed from logs alone.
func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string, cause error) {
	logErrorResponse(r, status, code, message, cause)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(model.APIResponse{
		Error: &model.APIError{Code: code, Message: message},
	})
}

// logErrorResponse logs the context of an error response.
func logErrorResponse(r *http.Request, status int, code, message string, cause error) {
	logger := middleware.LoggerFromContext(r.Context())

	attrs := []any{
		"status", status,
		"code", code,
		"message", message,
		"method", r.Method,
		"path", r.URL.Path,
		"remote_addr", r.RemoteAddr,
	}
	if userID, ok := middleware.GetUserID(r.Context()); ok && userID != "" {
		attrs = append(attrs, "user_id", userID)
	}
	if cause != nil {
		attrs = append(attrs, "error", cause.Error())
	}

	switch {
	case status >= http.StatusInternalServerError:
		if logger.Enabled(r.Context(), slog.LevelError) {
			attrs = append(attrs, "stack", string(debug.Stack()))
		}
		logger.ErrorContext(r.Context(), "error response", attrs...)
	case status >= http.StatusBadRequest:
		logger.WarnContext(r.Context(), "error response", attrs...)
	default:
		logger.InfoContext(r.Context(), "error response", attrs...)
	}
}
