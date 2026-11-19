package logging

import (
	"log/slog"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		in      string
		want    slog.Level
		wantErr bool
	}{
		{in: "", want: slog.LevelDebug},
		{in: "debug", want: slog.LevelDebug},
		{in: "DEBUG", want: slog.LevelDebug},
		{in: " info ", want: slog.LevelInfo},
		{in: "warn", want: slog.LevelWarn},
		{in: "warning", want: slog.LevelWarn},
		{in: "error", want: slog.LevelError},
		{in: "verbose", wantErr: true},
		{in: "3", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseLevel(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseLevel(%q): expected error, got %v", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseLevel(%q): unexpected error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("ParseLevel(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestNewInvalidLevel(t *testing.T) {
	if _, err := New("nope"); err == nil {
		t.Fatal("expected error for invalid level")
	}
}

func TestNewHonoursLevel(t *testing.T) {
	logger, err := New("error")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if logger.Enabled(t.Context(), slog.LevelWarn) {
		t.Error("warn should be disabled at error level")
	}
	if !logger.Enabled(t.Context(), slog.LevelError) {
		t.Error("error should be enabled at error level")
	}
}
