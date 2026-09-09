package main

import (
	"context"
	"flag"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4/server4"

	"github.com/mcaimi/illithid/internal/api"
	"github.com/mcaimi/illithid/internal/config"
	"github.com/mcaimi/illithid/internal/dhcp"
	"github.com/mcaimi/illithid/internal/intercept"
	"github.com/mcaimi/illithid/internal/lease"
	"github.com/mcaimi/illithid/internal/pool"
)

func main() {
	configPath := flag.String("config", "parameters.yaml", "path to configuration file")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	logger.Info("config loaded", "interfaces", len(cfg.Interfaces), "api", cfg.API.Listen)

	leaseStore := lease.NewStore()
	interceptStore := intercept.NewStore()

	pools := make(map[string]*pool.IPPool, len(cfg.Interfaces))
	var dhcpServers []*server4.Server
	var ifNames []string

	for _, ifCfg := range cfg.Interfaces {
		p, err := pool.New(
			net.ParseIP(ifCfg.DHCP.RangeStart),
			net.ParseIP(ifCfg.DHCP.RangeEnd),
		)
		if err != nil {
			logger.Error("failed to create pool", "interface", ifCfg.Name, "error", err)
			os.Exit(1)
		}
		pools[ifCfg.Name] = p

		handler := dhcp.NewHandler(ifCfg, p, leaseStore, interceptStore, logger)

		srv, err := dhcp.StartServer(ifCfg.Name, handler.ServeDHCP, logger)
		if err != nil {
			logger.Error("failed to create DHCP server", "interface", ifCfg.Name, "error", err)
			os.Exit(1)
		}
		dhcpServers = append(dhcpServers, srv)
		ifNames = append(ifNames, ifCfg.Name)

		total, free, _ := p.Stats()
		logger.Info("DHCP server created", "interface", ifCfg.Name, "pool_size", total, "free", free)
	}

	apiServer := api.NewServer(leaseStore, interceptStore, pools, ifNames)
	httpServer := &http.Server{
		Addr:    cfg.API.Listen,
		Handler: apiServer.Handler(),
	}

	var wg sync.WaitGroup

	for i, srv := range dhcpServers {
		wg.Add(1)
		go func(s *server4.Server, name string) {
			defer wg.Done()
			logger.Info("starting DHCP server", "interface", name)
			if err := s.Serve(); err != nil {
				logger.Error("DHCP server error", "interface", name, "error", err)
			}
		}(srv, ifNames[i])
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("starting REST API", "addr", cfg.API.Listen)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	logger.Info("received signal, shutting down", "signal", sig)

	for i, srv := range dhcpServers {
		srv.Close()
		logger.Info("DHCP server stopped", "interface", ifNames[i])
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	httpServer.Shutdown(ctx)

	wg.Wait()
	logger.Info("shutdown complete")
}
