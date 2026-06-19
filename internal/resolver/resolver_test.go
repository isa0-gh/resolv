package resolver

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolverResolveSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/dns-message" {
			t.Errorf("expected Content-Type application/dns-message, got %s", r.Header.Get("Content-Type"))
		}
		w.Header().Set("Content-Type", "application/dns-message")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("mock-dns-response"))
	}))
	defer ts.Close()

	r := NewResolver(ts.URL, http.DefaultClient)
	resp, err := r.Resolve([]byte("mock-dns-query"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(resp, []byte("mock-dns-response")) {
		t.Fatalf("expected mock-dns-response, got %s", string(resp))
	}
}

func TestResolverResolveNon200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	r := NewResolver(ts.URL, http.DefaultClient)
	_, err := r.Resolve([]byte("mock-dns-query"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
