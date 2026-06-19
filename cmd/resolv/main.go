package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/isa0-gh/resolv/internal/cache"
	"github.com/isa0-gh/resolv/internal/config"
	"github.com/isa0-gh/resolv/internal/local"
	"github.com/isa0-gh/resolv/internal/resolve-dns"
	"github.com/isa0-gh/resolv/internal/resolver"
	"github.com/isa0-gh/resolv/internal/server"
	"github.com/isa0-gh/resolv/internal/service"
)

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

	if err := server.New(repo).Run(); err != nil {
		slog.Error("resolv stopped", "error", err)
		os.Exit(1)
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
