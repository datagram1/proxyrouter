package router

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"time"
)

// Dialer represents a network dialer
type Dialer interface {
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

// DialerFactory creates dialers based on route groups
type DialerFactory struct {
	db           *sql.DB
	torAddress   string
	dialTimeout  time.Duration
}

// NewDialerFactory creates a new dialer factory
func NewDialerFactory(db *sql.DB, torAddress string, dialTimeout time.Duration) *DialerFactory {
	return &DialerFactory{
		db:          db,
		torAddress:  torAddress,
		dialTimeout: dialTimeout,
	}
}

// CreateDialer creates a dialer for the given route group
func (f *DialerFactory) CreateDialer(ctx context.Context, route *Route) (Dialer, error) {
	switch route.Group {
	case RouteGroupLocal:
		return f.createLocalDialer()
	case RouteGroupTor:
		return f.createTorDialer()
	case RouteGroupGeneral:
		return f.createGeneralDialer(ctx)
	case RouteGroupUpstream:
		return f.createUpstreamDialer(ctx, route.ProxyID)
	default:
		return nil, fmt.Errorf("unknown route group: %s", route.Group)
	}
}

// createLocalDialer creates a direct connection dialer
func (f *DialerFactory) createLocalDialer() (Dialer, error) {
	return &net.Dialer{
		Timeout: f.dialTimeout,
	}, nil
}

// createTorDialer creates a Tor SOCKS5 dialer
func (f *DialerFactory) createTorDialer() (Dialer, error) {
	// TODO: Implement Tor SOCKS5 dialer when go-socks5 dependency is fixed
	return &net.Dialer{
		Timeout: f.dialTimeout,
	}, nil
}

// createGeneralDialer creates a dialer that selects from the general proxy pool
func (f *DialerFactory) createGeneralDialer(ctx context.Context) (Dialer, error) {
	// Get the best available proxy from the general pool
	proxy, err := f.getBestGeneralProxy(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get general proxy: %w", err)
	}

	if proxy == nil {
		// Fallback to direct connection if no proxy available
		return f.createLocalDialer()
	}

	return f.createProxyDialer(proxy)
}

// createUpstreamDialer creates a dialer for a specific upstream proxy
func (f *DialerFactory) createUpstreamDialer(ctx context.Context, proxyID *int) (Dialer, error) {
	if proxyID == nil {
		return nil, fmt.Errorf("proxy_id is required for UPSTREAM route")
	}

	proxy, err := f.getProxyByID(ctx, *proxyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get upstream proxy: %w", err)
	}

	if proxy == nil {
		return nil, fmt.Errorf("proxy with id %d not found", *proxyID)
	}

	return f.createProxyDialer(proxy)
}

// createProxyDialer creates a dialer for a specific proxy
func (f *DialerFactory) createProxyDialer(proxy *Proxy) (Dialer, error) {
	switch proxy.Scheme {
	case "socks5":
		return f.createSOCKS5Dialer(proxy)
	case "http", "https":
		return f.createHTTPDialer(proxy)
	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s", proxy.Scheme)
	}
}

// createSOCKS5Dialer creates a SOCKS5 dialer
func (f *DialerFactory) createSOCKS5Dialer(proxy *Proxy) (Dialer, error) {
	// TODO: Implement SOCKS5 dialer when go-socks5 dependency is fixed
	return &net.Dialer{
		Timeout: f.dialTimeout,
	}, nil
}

// createHTTPDialer creates an HTTP proxy dialer
func (f *DialerFactory) createHTTPDialer(proxy *Proxy) (Dialer, error) {
	// For HTTP proxies, we'll use a custom dialer that handles CONNECT
	return &HTTPProxyDialer{
		proxyHost: fmt.Sprintf("%s:%d", proxy.Host, proxy.Port),
		timeout:   f.dialTimeout,
	}, nil
}

// getBestGeneralProxy gets the best available proxy from the general pool
func (f *DialerFactory) getBestGeneralProxy(ctx context.Context) (*Proxy, error) {
	query := `
		SELECT id, scheme, host, port, latency_ms, alive, last_checked_at
		FROM proxies
		WHERE alive = 1
		  AND (expires_at IS NULL OR expires_at > datetime('now'))
		  AND (last_checked_at IS NULL OR last_checked_at > datetime('now', '-1 hour'))
		ORDER BY latency_ms ASC NULLS LAST, last_checked_at DESC
		LIMIT 1
	`

	var p Proxy
	err := f.db.QueryRowContext(ctx, query).Scan(
		&p.ID,
		&p.Scheme,
		&p.Host,
		&p.Port,
		&p.LatencyMs,
		&p.Alive,
		&p.LastCheckedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query general proxy: %w", err)
	}

	return &p, nil
}

// getProxyByID gets a proxy by its ID
func (f *DialerFactory) getProxyByID(ctx context.Context, id int) (*Proxy, error) {
	query := `
		SELECT id, scheme, host, port, latency_ms, alive, last_checked_at
		FROM proxies
		WHERE id = ?
	`

	var p Proxy
	err := f.db.QueryRowContext(ctx, query, id).Scan(
		&p.ID,
		&p.Scheme,
		&p.Host,
		&p.Port,
		&p.LatencyMs,
		&p.Alive,
		&p.LastCheckedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query proxy: %w", err)
	}

	return &p, nil
}

// Proxy represents a proxy entry
type Proxy struct {
	ID            int        `json:"id"`
	Scheme        string     `json:"scheme"`
	Host          string     `json:"host"`
	Port          int        `json:"port"`
	LatencyMs     *int       `json:"latency_ms,omitempty"`
	Alive         bool       `json:"alive"`
	LastCheckedAt *time.Time `json:"last_checked_at,omitempty"`
}

// HTTPProxyDialer implements Dialer for HTTP proxies
type HTTPProxyDialer struct {
	proxyHost string
	timeout   time.Duration
}

// DialContext implements Dialer for HTTP proxies
func (h *HTTPProxyDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	// For HTTP proxies, we need to establish a CONNECT tunnel
	// This is a simplified implementation - in practice, you'd want to handle
	// the full HTTP CONNECT protocol
	
	// For now, we'll use a direct connection as a fallback
	dialer := &net.Dialer{
		Timeout: h.timeout,
	}
	return dialer.DialContext(ctx, network, addr)
}
