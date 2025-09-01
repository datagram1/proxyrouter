package refresh

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"proxyrouter/internal/config"
)

// JobManager manages refresh jobs
type JobManager struct {
	refresher *Refresher
	config    *config.Config
	logger    *slog.Logger
	stopChan  chan struct{}
	wg        sync.WaitGroup
}

// NewJobManager creates a new job manager
func NewJobManager(refresher *Refresher, config *config.Config, logger *slog.Logger) *JobManager {
	return &JobManager{
		refresher: refresher,
		config:    config,
		logger:    logger,
		stopChan:  make(chan struct{}),
	}
}

// Start starts the job manager
func (jm *JobManager) Start(ctx context.Context) error {
	if !jm.config.Refresh.EnableGeneralSources {
		jm.logger.Info("Refresh jobs disabled - general sources not enabled")
		return nil
	}

	jm.logger.Info("Starting refresh job manager", 
		"interval_sec", jm.config.Refresh.IntervalSec,
		"healthcheck_concurrency", jm.config.Refresh.HealthcheckConcurrency)

	// Start ingest job
	jm.wg.Add(1)
	go jm.runIngestJob(ctx)

	// Start health check job
	jm.wg.Add(1)
	go jm.runHealthJob(ctx)

	return nil
}

// Stop stops the job manager
func (jm *JobManager) Stop() {
	close(jm.stopChan)
	jm.wg.Wait()
	jm.logger.Info("Refresh job manager stopped")
}

// runIngestJob runs the ingest job periodically
func (jm *JobManager) runIngestJob(ctx context.Context) {
	defer jm.wg.Done()

	ticker := time.NewTicker(jm.config.GetRefreshInterval())
	defer ticker.Stop()

	// Run immediately on start
	if err := jm.ingestProxies(ctx); err != nil {
		jm.logger.Error("Initial ingest job failed", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-jm.stopChan:
			return
		case <-ticker.C:
			if err := jm.ingestProxies(ctx); err != nil {
				jm.logger.Error("Ingest job failed", "error", err)
			}
		}
	}
}

// runHealthJob runs the health check job periodically
func (jm *JobManager) runHealthJob(ctx context.Context) {
	defer jm.wg.Done()

	ticker := time.NewTicker(jm.config.GetRefreshInterval())
	defer ticker.Stop()

	// Run immediately on start
	if err := jm.healthCheckProxies(ctx); err != nil {
		jm.logger.Error("Initial health check job failed", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-jm.stopChan:
			return
		case <-ticker.C:
			if err := jm.healthCheckProxies(ctx); err != nil {
				jm.logger.Error("Health check job failed", "error", err)
			}
		}
	}
}

// ingestProxies runs the ingest job
func (jm *JobManager) ingestProxies(ctx context.Context) error {
	jm.logger.Info("Starting proxy ingest job")
	start := time.Now()

	if err := jm.refresher.RefreshAll(ctx); err != nil {
		return fmt.Errorf("failed to refresh proxies: %w", err)
	}

	duration := time.Since(start)
	jm.logger.Info("Proxy ingest job completed", "duration", duration)
	return nil
}

// healthCheckProxies runs the health check job
func (jm *JobManager) healthCheckProxies(ctx context.Context) error {
	jm.logger.Info("Starting proxy health check job")
	start := time.Now()

	if err := jm.refresher.HealthCheck(ctx); err != nil {
		return fmt.Errorf("failed to health check proxies: %w", err)
	}

	duration := time.Since(start)
	jm.logger.Info("Proxy health check job completed", "duration", duration)
	return nil
}

// ManualRefresh triggers a manual refresh
func (jm *JobManager) ManualRefresh(ctx context.Context) error {
	jm.logger.Info("Manual refresh triggered")
	return jm.ingestProxies(ctx)
}

// ManualHealthCheck triggers a manual health check
func (jm *JobManager) ManualHealthCheck(ctx context.Context) error {
	jm.logger.Info("Manual health check triggered")
	return jm.healthCheckProxies(ctx)
}
