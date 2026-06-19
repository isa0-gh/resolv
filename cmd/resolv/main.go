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
	"github.com/pelletier/go-toml/v2"
)

func main() {
	configPath := flag.String("config", config.DefaultConfigPath, "path to config file")
	checkConfig := flag.Bool("check-config", false, "validate config and exit")
	printConfig := flag.Bool("print-config", false, "print config and exit")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	conf, err := config.Load(*configPath)
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	if *checkConfig {
		slog.Info("Config validation successful", "config", *configPath)
		os.Exit(0)
	}

	if *printConfig {
		slog.Info("Config", "config", conf)
		os.Exit(0)
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

	slog.Info("Starting resolv...", "resolver", conf.Resolver, "listen", conf.BindAddress, "config", *configPath)
	if err := server.New(repo).Run(); err != nil {
		slog.Error("resolv stopped", "error", err)
		os.Exit(1)
	}
}
