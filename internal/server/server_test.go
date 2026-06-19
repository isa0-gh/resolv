package server

import (
	"encoding/binary"
	"errors"
	"io"
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func packDNSQuery(t *testing.T, id uint16) []byte {
	t.Helper()
	msg := new(dns.Msg)
	msg.SetQuestion("example.com.", dns.TypeA)
	msg.Id = id
	queryBytes, err := msg.Pack()
	if err != nil {
		t.Fatalf("failed to pack query: %v", err)
	}
	return queryBytes
}

func packDNSResponse(t *testing.T, id uint16) []byte {
	t.Helper()
	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	req.Id = id

	resp := new(dns.Msg)
	resp.SetReply(req)
	responseBytes, err := resp.Pack()
	if err != nil {
		t.Fatalf("failed to pack response: %v", err)
	}
	return responseBytes
}

func TestServerTCPAndUDPRealListeners(t *testing.T) {
	// Find a free TCP port to bind both UDP and TCP
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	addrStr := l.Addr().String()
	l.Close() // Close so the server can bind to it

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/dns-message")
		w.WriteHeader(http.StatusOK)
		resp := packDNSResponse(t, 7777)
		w.Write(resp)
	}))
	defer ts.Close()

	conf := &config.Config{
		Resolver:    ts.URL,
		TTL:         300,
		BindAddress: addrStr,
	}
	c := cache.New()
	matcher := local.NewMatcher(nil, conf.TTL)
	res := resolver.NewResolver(conf.Resolver, http.DefaultClient)
	repo := service.NewServiceRepo(conf, c, matcher, res)

	srv := New(repo)
	go func() {
		if err := srv.Run(); err != nil && !errors.Is(err, net.ErrClosed) {
			// ignore closed listener error on test exit
		}
	}()

	time.Sleep(100 * time.Millisecond) // Wait for listeners to start

	// 1. Test TCP
	conn, err := net.Dial("tcp", addrStr)
	if err != nil {
		t.Fatalf("failed to dial tcp: %v", err)
	}
	defer conn.Close()

	query := packDNSQuery(t, 7777)
	binary.Write(conn, binary.BigEndian, uint16(len(query)))
	conn.Write(query)

	var respLen uint16
	binary.Read(conn, binary.BigEndian, &respLen)
	respData := make([]byte, respLen)
	io.ReadFull(conn, respData)

	respMsg := new(dns.Msg)
	respMsg.Unpack(respData)
	if respMsg.Rcode != dns.RcodeSuccess || respMsg.Id != 7777 {
		t.Errorf("TCP check failed: rcode=%s id=%d", dns.RcodeToString[respMsg.Rcode], respMsg.Id)
	}

	// 2. Test UDP
	udpAddr, _ := net.ResolveUDPAddr("udp", addrStr)
	clientUDP, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		t.Fatalf("failed to dial udp: %v", err)
	}
	defer clientUDP.Close()

	clientUDP.Write(query)
	respBuf := make([]byte, 512)
	n, _ := clientUDP.Read(respBuf)

	udpRespMsg := new(dns.Msg)
	udpRespMsg.Unpack(respBuf[:n])
	if udpRespMsg.Rcode != dns.RcodeSuccess || udpRespMsg.Id != 7777 {
		t.Errorf("UDP check failed: rcode=%s id=%d", dns.RcodeToString[udpRespMsg.Rcode], udpRespMsg.Id)
	}
}
