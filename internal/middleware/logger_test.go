package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
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
}
