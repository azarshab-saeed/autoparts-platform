package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestManagerCORSAllowsFrontendAndRejectsUnknownOrigin(t *testing.T) {
	h := managerCORS(t.TempDir(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	for _, tc := range []struct {
		origin string
		want   int
	}{
		{"http://localhost:3000", http.StatusNoContent},
		{"http://127.0.0.1:3000", http.StatusNoContent},
		{"http://evil.local:3000", http.StatusForbidden},
	} {
		req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:17623/v1/lifecycle/start", nil)
		req.Header.Set("Origin", tc.origin)
		req.Header.Set("X-AutoParts-Edge", "1")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != tc.want {
			t.Fatalf("origin %s => %d want %d", tc.origin, rr.Code, tc.want)
		}
	}
}

func TestManagerCORSAllowsPairedCloudOrigin(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"allowed_origins":["https://app.example.com"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	h := managerCORS(dir, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:17623/v1/lifecycle/start", nil)
	req.Header.Set("Origin", "https://app.example.com")
	req.Header.Set("X-AutoParts-Edge", "1")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("paired origin => %d", rr.Code)
	}
}
