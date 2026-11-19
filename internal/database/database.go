package database

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool is the global database connection pool.
var Pool *pgxpool.Pool

var (
	loggerMu sync.RWMutex
	logger   *slog.Logger
)

// SetLogger configures the logger used for database query tracing.
// It should be called once during startup, before Connect.
func SetLogger(l *slog.Logger) {
	loggerMu.Lock()
	logger = l
	loggerMu.Unlock()
}

func queryLogger() *slog.Logger {
	loggerMu.RLock()
	l := logger
	loggerMu.RUnlock()
	if l != nil {
		return l
	}
	return slog.Default()
}

// Connect creates a connection pool to the PostgreSQL database.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	config.ConnConfig.Tracer = queryTracer{}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	Pool = pool
	return pool, nil
}

// Close closes the database connection pool.
func Close() {
	if Pool != nil {
		Pool.Close()
	}
}

// queryTracer logs each SQL statement and its duration at debug level.
type queryTracer struct{}

// queryStart holds the information captured when a query starts.
type queryStart struct {
	at  time.Time
	sql string
}

// queryStartKey is the context key holding the query start time and SQL.
type queryStartKey struct{}

func (queryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, queryStartKey{}, queryStart{at: time.Now(), sql: data.SQL})
}

func (queryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	l := queryLogger()
	if !l.Enabled(ctx, slog.LevelDebug) {
		return
	}

	start, _ := ctx.Value(queryStartKey{}).(queryStart)

	attrs := []any{"sql", compactSQL(start.sql)}
	if !start.at.IsZero() {
		attrs = append(attrs, "duration", time.Since(start.at).String())
	}
	if data.Err != nil {
		attrs = append(attrs, "error", data.Err)
	}

	l.DebugContext(ctx, "db query", attrs...)
}

const maxSQLInLog = 1000

// compactSQL collapses whitespace so a query stays on one log line.
func compactSQL(sql string) string {
	compact := strings.Join(strings.Fields(sql), " ")
	if len(compact) > maxSQLInLog {
		compact = compact[:maxSQLInLog] + "...(truncated)"
	}
	return compact
}
