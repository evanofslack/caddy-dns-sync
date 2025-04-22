package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/evanofslack/caddy-dns-sync/config"
	"github.com/evanofslack/caddy-dns-sync/metrics"
	"github.com/evanofslack/caddy-dns-sync/provider/cloudflare"
	"github.com/evanofslack/caddy-dns-sync/reconcile"
	"github.com/evanofslack/caddy-dns-sync/source/caddy"
	"github.com/evanofslack/caddy-dns-sync/state"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Initialize metrics
	metrics := metrics.New(true)

	// Set up HTTP server for metrics and health checks
	mux := http.NewServeMux()
	mux.Handle("/metrics", metrics.Handler())

	server := &http.Server{
		Addr:    ":9090",
		Handler: mux,
	}

	// Start http server in background
	go func() {
		slog.Info("Starting metrics server", "address", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Metrics server failed", "error", err)
		}
	}()

	// Graceful shutdown handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.Load("config.yaml")
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	stateManager, err := state.New(cfg.StatePath, metrics)
	if err != nil {
		slog.Error("Failed to initialize state manager", "error", err)
		os.Exit(1)
	}
	defer stateManager.Close()

	caddyClient := caddy.New(cfg.Caddy.AdminURL, metrics)

	cf, err := cloudflare.New(cfg.DNS, metrics)
	if err != nil {
		slog.Error("Failed to initialize DNS provider", "error", err)
		os.Exit(1)
	}

	engine := reconcile.NewEngine(stateManager, cf, cfg)

	slog.Info("Starting caddy-dns-sync service")

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go runSyncLoop(ctx, wg, caddyClient, engine, metrics, cfg.SyncInterval)

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	slog.Info("Shutdown signal received")
	cancel()

	// Shutdown server with same context
	serverShutdownCtx, cancelServer := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelServer()
	if err := server.Shutdown(serverShutdownCtx); err != nil {
		slog.Error("Metrics server shutdown error", "error", err)
	}

	// Wait for sync loop to finish
	wg.Wait()
	slog.Info("Service shutdown complete")
}

func runSyncLoop(ctx context.Context, wg *sync.WaitGroup, client caddy.Client, engine reconcile.Engine, metrics *metrics.Metrics, interval time.Duration) {
	defer wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if err := performSync(ctx, client, engine, metrics); err != nil {
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

func performSync(ctx context.Context, client caddy.Client, engine reconcile.Engine, metrics *metrics.Metrics) error {
	slog.Info("Starting sync operation")
	start := time.Now()
	defer func() {
		metrics.SetSyncDuration(time.Since(start))
	}()

	domains, err := client.Domains(ctx)
	if err != nil {
		metrics.IncSyncRun(false)
		return err
	}

	slog.Info("Reconciling domains", "count", len(domains))
	results, err := engine.Reconcile(ctx, domains)
	if err != nil {
		metrics.IncSyncRun(false)
		return err
	}

	slog.Info("Sync completed",
		"created", len(results.Created),
		"updated", len(results.Updated),
		"deleted", len(results.Deleted))
	metrics.IncSyncRun(true)

	return nil
}
