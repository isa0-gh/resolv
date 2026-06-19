package server

import (
	"log/slog"
	"net"

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
	addr, err := net.ResolveUDPAddr("udp", s.repo.Config.BindAddress)
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	slog.Info("resolv started.", "resolver", s.repo.Config.Resolver, "listen", s.repo.Config.BindAddress, "config", s.repo.Config)

	buf := make([]byte, 4096)
	for {
		size, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			slog.Error("ERROR reading from UDP", "error", err)
			continue
		}

		request := make([]byte, size)
		copy(request, buf[:size])
		go s.HandleConn(request, clientAddr, conn)
	}
}
