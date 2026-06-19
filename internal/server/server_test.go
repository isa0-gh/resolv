package server

import (
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

func TestServerHandleConnServfailOnResolverError(t *testing.T) {
	// 1. Set up a failing upstream DoH server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	// 2. Set up UDP listener to act as fake client and fake server conn
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

	// 3. Construct Server dependencies
	conf := &config.Config{
		Resolver: ts.URL,
		TTL:      300,
	}
	c := cache.New()
	matcher := local.NewMatcher(nil, conf.TTL)
	res := resolver.NewResolver(conf.Resolver, http.DefaultClient)
	repo := service.NewServiceRepo(conf, c, matcher, res)

	srv := New(repo)

	// 4. Send a DNS request from client to server
	msg := new(dns.Msg)
	msg.SetQuestion("example.com.", dns.TypeA)
	msg.Id = 1234
	queryBytes, err := msg.Pack()
	if err != nil {
		t.Fatalf("failed to pack dns msg: %v", err)
	}

	// Read and process the query in HandleConn (run in goroutine or synchronously)
	// We need clientAddr to pass to HandleConn
	clientAddrChan := make(chan *net.UDPAddr, 1)
	go func() {
		buf := make([]byte, 512)
		n, clientAddr, err := serverConn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		clientAddrChan <- clientAddr
		srv.HandleConn(buf[:n], clientAddr, serverConn)
	}()

	_, err = clientConn.Write(queryBytes)
	if err != nil {
		t.Fatalf("failed to write query: %v", err)
	}

	// 5. Read response back on client side
	_ = <-clientAddrChan
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

	if respMsg.Rcode != dns.RcodeServerFailure {
		t.Errorf("expected RcodeServerFailure (SERVFAIL), got %s", dns.RcodeToString[respMsg.Rcode])
	}
	if respMsg.Id != 1234 {
		t.Errorf("expected ID 1234, got %d", respMsg.Id)
	}
}
