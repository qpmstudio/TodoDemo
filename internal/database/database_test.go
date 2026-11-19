package database

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestQueryTracerLogsQueryAtDebug(t *testing.T) {
	var buf bytes.Buffer
	SetLogger(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { SetLogger(nil) })

	tracer := queryTracer{}
	ctx := tracer.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{
		SQL: "SELECT id,\n  title\nFROM todos",
	})
	tracer.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log is not valid JSON: %v (%q)", err, buf.String())
	}
	if entry["msg"] != "db query" {
		t.Errorf("expected msg %q, got %v", "db query", entry["msg"])
	}
	if entry["sql"] != "SELECT id, title FROM todos" {
		t.Errorf("unexpected sql: %v", entry["sql"])
	}
	if _, ok := entry["duration"]; !ok {
		t.Error("expected duration field")
	}
}

func TestQueryTracerSilentBelowDebug(t *testing.T) {
	var buf bytes.Buffer
	SetLogger(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { SetLogger(nil) })

	tracer := queryTracer{}
	ctx := tracer.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{SQL: "SELECT 1"})
	tracer.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})

	if buf.Len() != 0 {
		t.Errorf("expected no log output at info level, got %q", buf.String())
	}
}

func TestCompactSQLLimitsLength(t *testing.T) {
	got := compactSQL("SELECT  " + string(bytes.Repeat([]byte("x"), maxSQLInLog+100)))
	if len(got) > maxSQLInLog+len("...(truncated)") {
		t.Errorf("compactSQL not truncated: len=%d", len(got))
	}
	if got[len(got)-len("...(truncated)"):] != "...(truncated)" {
		t.Errorf("expected truncation marker, got %q", got)
	}
}
