package server

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/isa0-gh/resolv/internal/cache"
	"github.com/isa0-gh/resolv/internal/config"
	"github.com/isa0-gh/resolv/internal/local"
	"github.com/isa0-gh/resolv/internal/resolver"
	"github.com/isa0-gh/resolv/internal/service"
	"github.com/miekg/dns"
)

func TestServerHandleConnServfailOnNon200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	respMsg := exchangeDNSQuery(t, newTestServer(ts.URL, http.DefaultClient))
	assertServfail(t, respMsg)
}

func TestServerHandleConnServfailOnInvalidDNSResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/dns-message")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("not a dns message")); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer ts.Close()

	respMsg := exchangeDNSQuery(t, newTestServer(ts.URL, http.DefaultClient))
	assertServfail(t, respMsg)
}

func TestServerHandleConnServfailOnTransportError(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return nil, errors.New("upstream unavailable")
		}),
	}

	respMsg := exchangeDNSQuery(t, newTestServer("https://example.test/dns-query", client))
	assertServfail(t, respMsg)
}

func newTestServer(url string, client *http.Client) *Server {
	conf := &config.Config{
		Resolver: url,
		TTL:      300,
	}
	c := cache.New()
	matcher := local.NewMatcher(nil, conf.TTL)
	res := resolver.NewResolver(conf.Resolver, client)
	repo := service.NewServiceRepo(conf, c, matcher, res)

	return New(repo)
}

func exchangeDNSQuery(t *testing.T, srv *Server) *dns.Msg {
	t.Helper()

	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to resolve udp addr: %v", err)
	}
	serverConn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Fatalf("failed to listen on udp: %v", err)
	}
	defer serverConn.Close()

	clientConn, err := net.DialUDP("udp", nil, serverConn.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatalf("failed to dial server: %v", err)
	}
	defer clientConn.Close()

	msg := new(dns.Msg)
	msg.SetQuestion("example.com.", dns.TypeA)
	msg.Id = 1234
	queryBytes, err := msg.Pack()
	if err != nil {
		t.Fatalf("failed to pack dns msg: %v", err)
	}

	readErr := make(chan error, 1)
	go func() {
		buf := make([]byte, 512)
		n, clientAddr, err := serverConn.ReadFromUDP(buf)
		if err != nil {
			readErr <- err
			return
		}
		srv.HandleConn(buf[:n], clientAddr, serverConn)
		readErr <- nil
	}()

	_, err = clientConn.Write(queryBytes)
	if err != nil {
		t.Fatalf("failed to write query: %v", err)
	}

	clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	respBuf := make([]byte, 512)
	n, err := clientConn.Read(respBuf)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	respMsg := new(dns.Msg)
	if err := respMsg.Unpack(respBuf[:n]); err != nil {
		t.Fatalf("failed to unpack dns response: %v", err)
	}

	if err := <-readErr; err != nil {
		t.Fatalf("failed to read query on server conn: %v", err)
	}

	return respMsg
}

func assertServfail(t *testing.T, respMsg *dns.Msg) {
	t.Helper()

	if respMsg.Rcode != dns.RcodeServerFailure {
		t.Errorf("expected RcodeServerFailure (SERVFAIL), got %s", dns.RcodeToString[respMsg.Rcode])
	}
	if respMsg.Id != 1234 {
		t.Errorf("expected ID 1234, got %d", respMsg.Id)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
