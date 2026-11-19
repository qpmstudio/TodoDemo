package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DevenWen/TodoDemo/internal/logging"
	"github.com/DevenWen/TodoDemo/internal/middleware"
	"github.com/DevenWen/TodoDemo/internal/model"
)

func requestWithLogger(t *testing.T, method, target string) (*http.Request, *bytes.Buffer) {
	t.Helper()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	req := httptest.NewRequest(method, target, nil)
	ctx := logging.WithLogger(req.Context(), logger)
	ctx = context.WithValue(ctx, middleware.UserIDKey, "user-1")
	return req.WithContext(ctx), &buf
}

func TestWriteErrorLogsCauseAndStackFor5xx(t *testing.T) {
	req, buf := requestWithLogger(t, http.MethodGet, "/api/v1/todos?page=2")
	rec := httptest.NewRecorder()

	writeError(rec, req, http.StatusInternalServerError, model.ErrCodeInternalError,
		"Failed to fetch todos", errors.New("connection refused"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}

	// The client response must not leak the internal error.
	body := rec.Body.String()
	if strings.Contains(body, "connection refused") {
		t.Errorf("internal error leaked to client: %s", body)
	}

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log is not valid JSON: %v (%q)", err, buf.String())
	}
	if entry["level"] != "ERROR" {
		t.Errorf("expected ERROR level, got %v", entry["level"])
	}
	if entry["error"] != "connection refused" {
		t.Errorf("expected cause in log, got %v", entry["error"])
	}
	if entry["status"] != float64(500) {
		t.Errorf("expected status 500 in log, got %v", entry["status"])
	}
	if entry["user_id"] != "user-1" {
		t.Errorf("expected user_id in log, got %v", entry["user_id"])
	}
	stack, _ := entry["stack"].(string)
	if !strings.Contains(stack, "TestWriteErrorLogsCauseAndStackFor5xx") {
		t.Errorf("expected a stack trace in log, got %q", stack)
	}
}

func TestWriteErrorLogsWarnFor4xxWithoutStack(t *testing.T) {
	req, buf := requestWithLogger(t, http.MethodPost, "/api/v1/todos")
	rec := httptest.NewRecorder()

	writeError(rec, req, http.StatusBadRequest, model.ErrCodeValidationError, "Title is required", nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log is not valid JSON: %v (%q)", err, buf.String())
	}
	if entry["level"] != "WARN" {
		t.Errorf("expected WARN level, got %v", entry["level"])
	}
	if _, ok := entry["stack"]; ok {
		t.Error("4xx responses should not log a stack trace")
	}
	if entry["code"] != model.ErrCodeValidationError {
		t.Errorf("expected error code in log, got %v", entry["code"])
	}
}
