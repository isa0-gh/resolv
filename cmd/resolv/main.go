package main

import (
	"flag"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/isa0-gh/resolv/internal/cache"
	"github.com/isa0-gh/resolv/internal/config"
	"github.com/isa0-gh/resolv/internal/local"
	"github.com/isa0-gh/resolv/internal/resolve-dns"
	"github.com/isa0-gh/resolv/internal/resolver"
	"github.com/isa0-gh/resolv/internal/service"
)

type Server struct {
	repo *service.ServiceRepo
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

func main() {
	configPath := flag.String("config", config.DefaultConfigPath, "path to config file")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	conf, err := config.Load(*configPath)
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	client, err := resolvedns.ResolveServer(conf.Resolver)
	if err != nil {
		slog.Error("Failed to initialize http client", "error", err)
		os.Exit(1)
	}
	conf.Client = client

	cdb := cache.New(time.Duration(conf.TTL) * time.Second)
	matcher := local.NewMatcher(conf.Hosts, conf.TTL)
	res := resolver.NewResolver(conf.Resolver, conf.Client)
	repo := service.NewServiceRepo(conf, cdb, matcher, res)

	srv := &Server{repo: repo}

	addr, err := net.ResolveUDPAddr("udp", conf.BindAddress)
	if err != nil {
		panic(err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	slog.Info("resolv started.", "resolver", conf.Resolver, "listen", conf.BindAddress, "config", *configPath)

	buf := make([]byte, 4096)
	for {
		size, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			slog.Error("ERROR reading from UDP", "error", err)
			continue
		}

		request := make([]byte, size)
		copy(request, buf[:size])
		go srv.HandleConn(request, clientAddr, conn)
	}
}
