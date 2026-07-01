package meshsync

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHealthServerReadiness verifies that /healthz always reports OK and
// that /readyz flips from 503 to 200 once markReady is called.
func TestHealthServerReadiness(t *testing.T) {
	h := newHealthServer()
	srv := httptest.NewServer(h.handler())
	defer srv.Close()

	assertStatus := func(path string, expected int) {
		t.Helper()
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		defer func() {
			_ = resp.Body.Close()
		}()
		if resp.StatusCode != expected {
			t.Errorf("GET %s: expected status %d, got %d", path, expected, resp.StatusCode)
		}
	}

	assertStatus("/healthz", http.StatusOK)
	assertStatus("/readyz", http.StatusServiceUnavailable)

	h.markReady()

	assertStatus("/healthz", http.StatusOK)
	assertStatus("/readyz", http.StatusOK)
}
