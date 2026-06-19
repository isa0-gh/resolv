package resolver

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/miekg/dns"
)

func TestResolverResolveSuccess(t *testing.T) {
	responseBytes := packDNSResponse(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/dns-message" {
			t.Errorf("expected Content-Type application/dns-message, got %s", r.Header.Get("Content-Type"))
		}
		w.Header().Set("Content-Type", "application/dns-message")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(responseBytes); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer ts.Close()

	r := NewResolver(ts.URL, http.DefaultClient)
	resp, err := r.Resolve(packDNSQuery(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(resp, responseBytes) {
		t.Fatalf("expected mock-dns-response, got %s", string(resp))
	}
}

func TestResolverResolveNon200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	r := NewResolver(ts.URL, http.DefaultClient)
	_, err := r.Resolve(packDNSQuery(t))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestResolverResolveInvalidDNSResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/dns-message")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("not a dns message")); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer ts.Close()

	r := NewResolver(ts.URL, http.DefaultClient)
	_, err := r.Resolve(packDNSQuery(t))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func packDNSQuery(t *testing.T) []byte {
	t.Helper()

	msg := new(dns.Msg)
	msg.SetQuestion("example.com.", dns.TypeA)
	msg.Id = 1234
	queryBytes, err := msg.Pack()
	if err != nil {
		t.Fatalf("failed to pack dns query: %v", err)
	}
	return queryBytes
}

func packDNSResponse(t *testing.T) []byte {
	t.Helper()

	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	req.Id = 1234

	resp := new(dns.Msg)
	resp.SetReply(req)
	responseBytes, err := resp.Pack()
	if err != nil {
		t.Fatalf("failed to pack dns response: %v", err)
	}
	return responseBytes
}
