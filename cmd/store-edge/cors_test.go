package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/example/autoparts-core/internal/storeedge"
)

func TestLocalCORSAllowsAgentSameOrigin(t *testing.T) {
	store, err := storeedge.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h := localCORS(store, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))

	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:17624/v1/pair", nil)
	req.Host = "127.0.0.1:17624"
	req.Header.Set("Origin", "http://127.0.0.1:17624")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestLocalCORSAllowsConfiguredWebOrigin(t *testing.T) {
	store, err := storeedge.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h := localCORS(store, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))

	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:17624/v1/status", nil)
	req.Host = "127.0.0.1:17624"
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestLocalCORSRejectsDifferentLoopbackPort(t *testing.T) {
	store, err := storeedge.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h := localCORS(store, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))

	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:17624/v1/pair", nil)
	req.Host = "127.0.0.1:17624"
	req.Header.Set("Origin", "http://127.0.0.1:9999")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestLocalCORSRejectsUntrustedOrigin(t *testing.T) {
	store, err := storeedge.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h := localCORS(store, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))

	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:17624/v1/pair", nil)
	req.Host = "127.0.0.1:17624"
	req.Header.Set("Origin", "https://evil.example")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}
