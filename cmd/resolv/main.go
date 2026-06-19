package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

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

	cdb := cache.New()
	matcher := local.NewMatcher(conf.Hosts, conf.TTL)
	res := resolver.NewResolver(conf.Resolver, conf.Client)
	repo := service.NewServiceRepo(conf, cdb, matcher, res)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := server.New(repo).RunContext(ctx); err != nil {
		slog.Error("resolv stopped with error", "error", err)
		os.Exit(1)
	}
}
