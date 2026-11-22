package handler

import "net/http"

// probeResponse is the fixed body served by GET /api/v1/probe.
//
// It is written verbatim instead of going through writeJSON: the probe is a
// deploy/QA contract and callers compare the exact top-level JSON, so it must
// not be wrapped in the standard {"data": ...} envelope.
const probeResponse = `{"status":"ok","service":"tododemo"}`

// Probe reports that the service is up and running the expected build.
//
// It is intentionally unauthenticated: the deploy pipeline and QA use it to
// confirm that a newly deployed revision is actually serving traffic.
func Probe(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(probeResponse))
}
