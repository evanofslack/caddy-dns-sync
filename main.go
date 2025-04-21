package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/evanofslack/caddy-dns-sync/source/caddy"
	"github.com/evanofslack/caddy-dns-sync/config"
	"github.com/evanofslack/caddy-dns-sync/provider/cloudflare"
	"github.com/evanofslack/caddy-dns-sync/reconcile"
	"github.com/evanofslack/caddy-dns-sync/state"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load("config.yaml")
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stateManager, err := state.NewBadgerManager(cfg.StatePath)
	if err != nil {
		slog.Error("Failed to initialize state manager", "error", err)
		os.Exit(1)
	}
	defer stateManager.Close()

	caddyClient := caddy.New(cfg.CaddyConfig.AdminURL)

	cf, err := cloudflare.New(cfg.DNSConfig)
	if err != nil {
		slog.Error("Failed to initialize DNS provider", "error", err)
		os.Exit(1)
	}

	engine := reconcile.NewEngine(stateManager, cf, cfg.ReconcileConfig)

	slog.Info("Starting caddy-dns-sync service")

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go runSyncLoop(ctx, wg, caddyClient, engine, cfg.SyncInterval)

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	slog.Info("Shutdown signal received")
	cancel()
	wg.Wait()
	slog.Info("Service shutdown complete")
}

func runSyncLoop(ctx context.Context, wg *sync.WaitGroup, client caddy.Client, engine reconcile.Engine, interval time.Duration) {
	defer wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if err := performSync(client, engine); err != nil {
			slog.Error("Sync operation failed", "error", err)
		}

		select {
		case <-ticker.C:
			continue
		case <-ctx.Done():
			slog.Info("Stopping sync loop")
			return
		}
	}
}

func performSync(client caddy.Client, engine reconcile.Engine) error {
	slog.Info("Starting sync operation")

	domains, err := client.Domains()
	if err != nil {
		return err
	}

	slog.Info("Reconciling domains", "count", len(domains))
	results, err := engine.Reconcile(domains)
	if err != nil {
		return err
	}

	slog.Info("Sync completed",
		"created", len(results.Created),
		"updated", len(results.Updated),
		"deleted", len(results.Deleted))

	return nil
}
