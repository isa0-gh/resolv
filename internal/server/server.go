package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"time"

	"github.com/isa0-gh/resolv/internal/service"
)

type Server struct {
	repo *service.ServiceRepo
}

func New(repo *service.ServiceRepo) *Server {
	return &Server{repo: repo}
}

func (s *Server) HandleConn(data []byte, addr *net.UDPAddr, conn *net.UDPConn) {
	if localResp, ok := s.repo.Local.Match(data); ok {
		_, err := conn.WriteToUDP(localResp, addr)
		if err != nil {
			slog.Error("ERROR writing local resp", "error", err)
		}
		return
	}

	var resp []byte
	cached, ok, err := s.repo.Cache.Get(data)
	if err == nil && ok {
		resp = cached
	} else {
		resp, err = s.repo.Resolver.Resolve(data)
		if err != nil {
			slog.Error("ERROR resolving dns message", "error", err)
			return
		}
		if err := s.repo.Cache.Add(data, resp); err != nil {
			slog.Error("CACHE ERROR", "error", err)
		}
	}

	_, err = conn.WriteToUDP(resp, addr)
	if err != nil {
		slog.Error("ERROR writing to udp", "error", err)
		return
	}
}

func (s *Server) Run() error {
	return s.RunContext(context.Background())
}

func (s *Server) RunContext(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	addr, err := net.ResolveUDPAddr("udp", s.repo.Config.BindAddress)
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}

	return s.serveUDP(ctx, conn)
}

func (s *Server) serveUDP(ctx context.Context, conn *net.UDPConn) error {
	defer conn.Close()
	stopShutdownWatch := closeUDPConnOnContext(ctx, conn)
	defer stopShutdownWatch()

	slog.Info("resolv started.", "resolver", s.repo.Config.Resolver, "listen", s.repo.Config.BindAddress, "config", s.repo.Config)

	buf := make([]byte, 4096)
	stopFlusher := s.repo.Cache.StartFlusher(time.Duration(s.repo.Config.TTL) * time.Second)
	defer stopFlusher()

	for {
		size, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				slog.Info("resolv stopped.", "listen", s.repo.Config.BindAddress)
				return nil
			}
			slog.Error("ERROR reading from UDP", "error", err)
			continue
		}

		request := make([]byte, size)
		copy(request, buf[:size])
		go s.HandleConn(request, clientAddr, conn)
	}
}

func closeUDPConnOnContext(ctx context.Context, conn *net.UDPConn) func() {
	shutdownCtx, stop := context.WithCancel(ctx)
	done := make(chan struct{})

	go func() {
		defer close(done)
		<-shutdownCtx.Done()

		if ctx.Err() == nil {
			return
		}

		if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			slog.Error("ERROR closing UDP listener", "error", err)
		}
	}()

	return func() {
		stop()
		<-done
	}
}
