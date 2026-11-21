package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DevenWen/TodoDemo/internal/logging"
)

func TestLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	mw := Logger(logger)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	// Verify log output contains expected fields
	logOutput := buf.String()
	if !bytes.Contains([]byte(logOutput), []byte("GET")) {
		t.Error("log should contain method GET")
	}
	if !bytes.Contains([]byte(logOutput), []byte("/test")) {
		t.Error("log should contain path /test")
	}
	if !bytes.Contains([]byte(logOutput), []byte(`"status":200`)) {
		t.Error("log should contain status 200")
	}
	if !bytes.Contains([]byte(logOutput), []byte(`"duration"`)) {
		t.Error("log should contain duration")
	}
}

func TestLoggerDebugCapturesBodiesAndRedactsSecrets(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	const requestBody = `{"email":"user@example.com","password":"hunter2"}`

	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The middleware must not consume the body before the handler reads it.
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("handler failed to read body: %v", err)
		}
		if string(body) != requestBody {
			t.Errorf("handler got body %q, want %q", body, requestBody)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"data":{"id":"todo-1"}}`))
	}))

	req := httptest.NewRequest("POST", "/auth/login?token=s3cret&page=2", strings.NewReader(requestBody))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := buf.String()
	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rec.Code)
	}
	if strings.Contains(logOutput, "hunter2") {
		t.Errorf("password leaked into logs: %s", logOutput)
	}
	if strings.Contains(logOutput, "s3cret") {
		t.Errorf("query token leaked into logs: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"request_body"`) {
		t.Errorf("expected request_body in debug log: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"response_body"`) {
		t.Errorf("expected response_body in debug log: %s", logOutput)
	}
	if !strings.Contains(logOutput, "user@example.com") {
		t.Errorf("expected non-sensitive body content to be logged: %s", logOutput)
	}
	if !strings.Contains(logOutput, "todo-1") {
		t.Errorf("expected response content to be logged: %s", logOutput)
	}
}

func TestLoggerInfoLevelDoesNotLogBodies(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
	}))

	req := httptest.NewRequest("POST", "/todos", strings.NewReader(`{"title":"x"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	logOutput := buf.String()
	if strings.Contains(logOutput, "request_body") || strings.Contains(logOutput, "response_body") {
		t.Errorf("bodies should only be logged at debug level: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"status":200`) {
		t.Errorf("expected request log at info level: %s", logOutput)
	}
}

func TestLoggerTruncatesLargeBodies(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	large := strings.Repeat("x", maxLoggedBody+100)
	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.Write([]byte(large))
	}))

	req := httptest.NewRequest("POST", "/todos", strings.NewReader(large))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, `"response_body_truncated":true`) {
		t.Errorf("expected response truncation flag: %s", logOutput)
	}
	if strings.Contains(logOutput, large) {
		t.Error("full large response should not be logged")
	}
}

func TestLoggerEscalatesServerErrors(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequest("GET", "/boom", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if !strings.Contains(buf.String(), `"level":"ERROR"`) {
		t.Errorf("expected ERROR level log for 5xx response: %s", buf.String())
	}
}

func TestLoggerFromContextFallsBackToDefault(t *testing.T) {
	if logging.FromContext(t.Context()) != slog.Default() {
		t.Error("expected slog.Default() when no logger is in context")
	}

	custom := slog.New(slog.NewJSONHandler(io.Discard, nil))
	ctx := logging.WithLogger(t.Context(), custom)
	if logging.FromContext(ctx) != custom {
		t.Error("expected the logger stored in context")
	}
	if got, ok := logging.Lookup(ctx); !ok || got != custom {
		t.Error("expected Lookup to report the stored logger")
	}
	if _, ok := logging.Lookup(t.Context()); ok {
		t.Error("expected Lookup to report no logger for a bare context")
	}
}

func TestRedactBodyKeepsErrorCodes(t *testing.T) {
	body := []byte(`{"error":{"code":"VALIDATION_ERROR","message":"bad"},"password":"hunter2"}`)
	got := redactBody(body)

	if !strings.Contains(got, "VALIDATION_ERROR") {
		t.Errorf("API error code must not be redacted: %s", got)
	}
	if strings.Contains(got, "hunter2") {
		t.Errorf("password must be redacted: %s", got)
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Errorf("expected redaction marker: %s", got)
	}
}

func TestRedactQuery(t *testing.T) {
	query := redactQuery(map[string][]string{
		"code":  {"oauth-code"},
		"state": {"oauth-state"},
		"page":  {"2"},
	})
	if strings.Contains(query, "oauth-code") || strings.Contains(query, "oauth-state") {
		t.Errorf("sensitive query values not redacted: %s", query)
	}
	if !strings.Contains(query, "page=2") {
		t.Errorf("expected non-sensitive query value to survive: %s", query)
	}
}
