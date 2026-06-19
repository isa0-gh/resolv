package server

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/isa0-gh/resolv/internal/cache"
	"github.com/isa0-gh/resolv/internal/config"
	"github.com/isa0-gh/resolv/internal/local"
	"github.com/isa0-gh/resolv/internal/service"
)

func TestCloseUDPConnOnContextClosesOnCancel(t *testing.T) {
	conn := listenLocalUDP(t)
	ctx, cancel := context.WithCancel(context.Background())
	stop := closeUDPConnOnContext(ctx, conn)

	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	cancel()
	t.Cleanup(stop)

	_, _, err := conn.ReadFromUDP(make([]byte, 1))
	if err == nil {
		t.Fatal("expected closed UDP listener to unblock reads")
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		t.Fatalf("listener timed out instead of closing: %v", err)
	}
}

func TestServeUDPReturnsWhenContextCancelled(t *testing.T) {
	conn := listenLocalUDP(t)
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)

	srv := testServer(conn.LocalAddr().String())
	go func() {
		errCh <- srv.serveUDP(ctx, conn)
	}()

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("serveUDP returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serveUDP did not stop after context cancellation")
	}
}

func listenLocalUDP(t *testing.T) *net.UDPConn {
	t.Helper()

	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatalf("listen UDP: %v", err)
	}

	return conn
}

func testServer(bindAddress string) *Server {
	conf := &config.Config{
		Resolver:    "test",
		TTL:         3600,
		BindAddress: bindAddress,
		Hosts:       map[string]string{},
	}

	return New(service.NewServiceRepo(
		conf,
		cache.New(),
		local.NewMatcher(conf.Hosts, conf.TTL),
		nil,
	))
}
