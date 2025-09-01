package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"proxyrouter/internal/acl"
	"proxyrouter/internal/admin"
	"proxyrouter/internal/api"
	"proxyrouter/internal/config"
	"proxyrouter/internal/db"
	"proxyrouter/internal/proxyhttp"
	"proxyrouter/internal/proxysocks"
	"proxyrouter/internal/refresh"
	"proxyrouter/internal/router"
	"proxyrouter/internal/version"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "configs/config.yaml", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	// Handle version flag
	if *showVersion {
		fmt.Printf("ProxyRouter %s\n", version.Info())
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database
	database, err := db.New(cfg.Database.Path)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Run database migrations
	migrationsDir := filepath.Join(filepath.Dir(*configPath), "..", "migrations")
	if err := database.RunMigrations(migrationsDir); err != nil {
		log.Fatalf("Failed to run database migrations: %v", err)
	}

	// Initialize components
	aclManager := acl.New(database.GetDB())
	routerEngine := router.New(database.GetDB())
	dialerFactory := router.NewDialerFactory(
		database.GetDB(),
		cfg.Tor.SocksAddress,
		cfg.GetDialTimeout(),
	)
	refresher := refresh.New(database.GetDB(), &cfg.Refresh)

	// Initialize job manager
	refreshJobManager := refresh.NewJobManager(refresher, cfg, slog.Default())

	// Initialize servers
	httpProxy := proxyhttp.New(
		cfg.Listen.HTTPProxy,
		aclManager,
		routerEngine,
		dialerFactory,
		cfg.GetReadTimeout(),
	)

	socks5Proxy := proxysocks.New(
		cfg.Listen.Socks5Proxy,
		aclManager,
		routerEngine,
		dialerFactory,
		cfg.GetDialTimeout(),
	)

	apiServer := api.New(
		cfg.Listen.API,
		database,
		aclManager,
		routerEngine,
		refresher,
		cfg,
	)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		fmt.Printf("Received signal %v, shutting down...\n", sig)
		cancel()
	}()

	// Start job manager
	if err := refreshJobManager.Start(ctx); err != nil {
		log.Fatalf("Failed to start refresh job manager: %v", err)
	}
	defer refreshJobManager.Stop()

	// Start servers
	errChan := make(chan error, 4)

	// Start HTTP proxy
	go func() {
		if err := httpProxy.Start(ctx); err != nil {
			errChan <- fmt.Errorf("HTTP proxy error: %w", err)
		}
	}()

	// Start SOCKS5 proxy
	go func() {
		if err := socks5Proxy.Start(ctx); err != nil {
			errChan <- fmt.Errorf("SOCKS5 proxy error: %w", err)
		}
	}()

	// Start API server
	go func() {
		if err := apiServer.Start(ctx); err != nil {
			errChan <- fmt.Errorf("API server error: %w", err)
		}
	}()

	// Start admin server if enabled
	if cfg.Admin.Enabled {
		adminServer := admin.NewServer(cfg, database)
		go func() {
			if err := adminServer.Start(ctx); err != nil {
				errChan <- fmt.Errorf("Admin server error: %w", err)
			}
		}()
	}

	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		fmt.Println("Shutdown complete")
	case err := <-errChan:
		log.Fatalf("Server error: %v", err)
	}
}
