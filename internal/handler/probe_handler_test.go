package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProbe(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/probe", nil)
	rec := httptest.NewRecorder()

	Probe(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	if got, want := resp.Header.Get("Content-Type"), "application/json"; got != want {
		t.Errorf("expected Content-Type %q, got %q", want, got)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	// The probe body is a fixed contract used by the deploy/QA pipeline, so it
	// must be byte-for-byte stable - no trailing newline, no data wrapper.
	const wantBody = `{"status":"ok","service":"tododemo"}`
	if string(body) != wantBody {
		t.Errorf("expected body %s, got %s", wantBody, body)
	}

	var payload struct {
		Status  string `json:"status"`
		Service string `json:"service"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if payload.Status != "ok" {
		t.Errorf("expected status \"ok\", got %q", payload.Status)
	}
	if payload.Service != "tododemo" {
		t.Errorf("expected service \"tododemo\", got %q", payload.Service)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if _, wrapped := raw["data"]; wrapped {
		t.Error("expected a bare JSON body, got a data wrapper")
	}
}
