package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// maxLoggedBody caps how much of a request or response body is kept for logs.
const maxLoggedBody = 4096

// Logger returns middleware that logs one structured line per request.
//
// At debug level it additionally captures the request body, the response body
// and the query string. Values of sensitive keys (passwords, tokens, secrets)
// are redacted before being logged.
func Logger(logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			reqLogger := logger
			if id := middleware.GetReqID(r.Context()); id != "" {
				reqLogger = logger.With("request_id", id)
			}
			ctx := WithLogger(r.Context(), reqLogger)
			r = r.WithContext(ctx)

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			debug := reqLogger.Enabled(ctx, slog.LevelDebug)

			var reqBody *cappedBuffer
			if debug && r.Body != nil {
				reqBody = &cappedBuffer{limit: maxLoggedBody}
				r.Body = &teeReadCloser{Reader: io.TeeReader(r.Body, reqBody), Closer: r.Body}
			}

			var respBody *cappedBuffer
			if debug {
				respBody = &cappedBuffer{limit: maxLoggedBody}
				ww.Tee(respBody)
			}

			next.ServeHTTP(ww, r)

			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}

			attrs := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", status,
				"duration", time.Since(start).String(),
				"bytes", ww.BytesWritten(),
			}

			if debug {
				if query := redactQuery(r.URL.Query()); query != "" {
					attrs = append(attrs, "query", query)
				}
				if reqBody != nil && reqBody.Len() > 0 {
					attrs = append(attrs, "request_body", redactBody(reqBody.Bytes()))
					if reqBody.Truncated() {
						attrs = append(attrs, "request_body_truncated", true)
					}
				}
				if respBody != nil && respBody.Len() > 0 {
					attrs = append(attrs, "response_body", redactBody(respBody.Bytes()))
					if respBody.Truncated() {
						attrs = append(attrs, "response_body_truncated", true)
					}
				}
			}

			switch {
			case status >= http.StatusInternalServerError:
				reqLogger.ErrorContext(ctx, "request", attrs...)
			case status >= http.StatusBadRequest:
				reqLogger.WarnContext(ctx, "request", attrs...)
			default:
				reqLogger.InfoContext(ctx, "request", attrs...)
			}
		})
	}
}

// contextKey is a type for context keys to avoid collisions.
type contextKey string

const UserIDKey contextKey = "user_id"
const UserLoginKey contextKey = "user_login"
const loggerKey contextKey = "logger"

// WithLogger stores a request-scoped logger in ctx.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// LoggerFromContext returns the request-scoped logger, falling back to
// slog.Default when the request did not pass through the Logger middleware.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey).(*slog.Logger); ok && logger != nil {
		return logger
	}
	return slog.Default()
}

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

// cappedBuffer keeps at most limit bytes and remembers whether it overflowed.
type cappedBuffer struct {
	buf      bytes.Buffer
	limit    int
	overflow bool
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	if remaining := c.limit - c.buf.Len(); remaining > 0 {
		if len(p) <= remaining {
			c.buf.Write(p)
		} else {
			c.buf.Write(p[:remaining])
			c.overflow = true
		}
	} else if len(p) > 0 {
		c.overflow = true
	}
	return len(p), nil
}

func (c *cappedBuffer) Len() int        { return c.buf.Len() }
func (c *cappedBuffer) Bytes() []byte   { return c.buf.Bytes() }
func (c *cappedBuffer) Truncated() bool { return c.overflow }
func (c *cappedBuffer) String() string  { return c.buf.String() }

// teeReadCloser mirrors reads to a recorder while keeping the original closer.
type teeReadCloser struct {
	io.Reader
	io.Closer
}

// sensitiveKeys are JSON keys whose values must never reach the logs.
var sensitiveKeys = map[string]bool{
	"password":      true,
	"password_hash": true,
	"token":         true,
	"access_token":  true,
	"refresh_token": true,
	"authorization": true,
	"secret":        true,
	"client_secret": true,
	"jwt":           true,
	"jwt_secret":    true,
	"github_secret": true,
	"oauth_state":   true,
	"code":          true,
	"state":         true,
}

func isSensitiveKey(key string) bool {
	return sensitiveKeys[strings.ToLower(key)]
}

// redactBody masks sensitive JSON fields. Non-JSON bodies are returned as-is.
func redactBody(body []byte) string {
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return truncateUTF8(string(body), maxLoggedBody)
	}
	redactValue(value)
	out, err := json.Marshal(value)
	if err != nil {
		return truncateUTF8(string(body), maxLoggedBody)
	}
	return truncateUTF8(string(out), maxLoggedBody)
}

func redactValue(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			if isSensitiveKey(key) {
				typed[key] = "[REDACTED]"
				continue
			}
			redactValue(nested)
		}
	case []any:
		for _, nested := range typed {
			redactValue(nested)
		}
	}
}

// redactQuery re-encodes query parameters with sensitive values masked.
func redactQuery(query url.Values) string {
	if len(query) == 0 {
		return ""
	}
	cleaned := make(url.Values, len(query))
	for key, values := range query {
		if isSensitiveKey(key) {
			cleaned[key] = []string{"[REDACTED]"}
			continue
		}
		cleaned[key] = values
	}
	return cleaned.Encode()
}

// truncateUTF8 cuts s to at most max bytes without splitting a rune.
func truncateUTF8(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && (s[cut]&0xC0) == 0x80 {
		cut--
	}
	return s[:cut] + "...(truncated)"
}
