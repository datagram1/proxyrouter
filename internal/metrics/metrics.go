package metrics

import (
	"context"
	"database/sql"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics provides Prometheus metrics collection
type Metrics struct {
	// Proxy metrics
	proxyTotal    prometheus.Gauge
	proxyAlive    prometheus.Gauge
	proxyLatency  prometheus.Histogram

	// Route metrics
	routeHitsTotal *prometheus.CounterVec

	// Dial metrics
	dialErrorsTotal *prometheus.CounterVec
	dialDuration    prometheus.Histogram

	// Request metrics
	requestsTotal   *prometheus.CounterVec
	requestDuration prometheus.Histogram

	// ACL metrics
	aclDeniedTotal prometheus.Counter

	// Database for metrics collection
	db *sql.DB
}

// New creates a new metrics instance
func New(db *sql.DB) *Metrics {
	m := &Metrics{
		db: db,
		proxyTotal: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "proxyrouter_proxies_total",
			Help: "Total number of proxies",
		}),
		proxyAlive: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "proxyrouter_proxies_alive",
			Help: "Number of alive proxies",
		}),
		proxyLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "proxyrouter_proxy_latency_ms",
			Help:    "Proxy latency in milliseconds",
			Buckets: prometheus.DefBuckets,
		}),
		routeHitsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "proxyrouter_route_hits_total",
			Help: "Total number of route hits",
		}, []string{"route_group"}),
		dialErrorsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "proxyrouter_dial_errors_total",
			Help: "Total number of dial errors",
		}, []string{"route_group", "error_type"}),
		dialDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "proxyrouter_dial_duration_ms",
			Help:    "Dial duration in milliseconds",
			Buckets: prometheus.DefBuckets,
		}),
		requestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "proxyrouter_requests_total",
			Help: "Total number of requests",
		}, []string{"method", "status_code"}),
		requestDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "proxyrouter_request_duration_ms",
			Help:    "Request duration in milliseconds",
			Buckets: prometheus.DefBuckets,
		}),
		aclDeniedTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "proxyrouter_acl_denied_total",
			Help: "Total number of ACL denials",
		}),
	}

	// Start metrics collection
	go m.collectMetrics()

	return m
}

// collectMetrics periodically collects metrics from the database
func (m *Metrics) collectMetrics() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.updateProxyMetrics()
		}
	}
}

// updateProxyMetrics updates proxy-related metrics
func (m *Metrics) updateProxyMetrics() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get total proxies
	var total int
	err := m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM proxies").Scan(&total)
	if err == nil {
		m.proxyTotal.Set(float64(total))
	}

	// Get alive proxies
	var alive int
	err = m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM proxies WHERE alive = 1").Scan(&alive)
	if err == nil {
		m.proxyAlive.Set(float64(alive))
	}

	// Get latency metrics
	rows, err := m.db.QueryContext(ctx, "SELECT latency_ms FROM proxies WHERE alive = 1 AND latency_ms IS NOT NULL")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var latency int
			if err := rows.Scan(&latency); err == nil {
				m.proxyLatency.Observe(float64(latency))
			}
		}
	}
}

// RecordRouteHit records a route hit
func (m *Metrics) RecordRouteHit(routeGroup string) {
	m.routeHitsTotal.WithLabelValues(routeGroup).Inc()
}

// RecordDialError records a dial error
func (m *Metrics) RecordDialError(routeGroup, errorType string) {
	m.dialErrorsTotal.WithLabelValues(routeGroup, errorType).Inc()
}

// RecordDialDuration records dial duration
func (m *Metrics) RecordDialDuration(duration time.Duration) {
	m.dialDuration.Observe(float64(duration.Milliseconds()))
}

// RecordRequest records a request
func (m *Metrics) RecordRequest(method string, statusCode int, duration time.Duration) {
	m.requestsTotal.WithLabelValues(method, string(rune(statusCode))).Inc()
	m.requestDuration.Observe(float64(duration.Milliseconds()))
}

// RecordACLDenial records an ACL denial
func (m *Metrics) RecordACLDenial() {
	m.aclDeniedTotal.Inc()
}

// GetP95Latency returns the 95th percentile latency
func (m *Metrics) GetP95Latency() float64 {
	// This would require implementing a custom histogram or using a different approach
	// For now, return 0 as placeholder
	return 0
}
