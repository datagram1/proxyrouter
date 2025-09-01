package proxysocks

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/armon/go-socks5"

	"proxyrouter/internal/acl"
	"proxyrouter/internal/router"
)

// Server represents the SOCKS5 proxy server
type Server struct {
	listenAddr   string
	acl          *acl.ACL
	router       *router.Router
	dialerFactory *router.DialerFactory
	timeout      time.Duration
}

// New creates a new SOCKS5 server
func New(listenAddr string, acl *acl.ACL, router *router.Router, dialerFactory *router.DialerFactory, timeout time.Duration) *Server {
	return &Server{
		listenAddr:    listenAddr,
		acl:           acl,
		router:        router,
		dialerFactory: dialerFactory,
		timeout:       timeout,
	}
}

// Start starts the SOCKS5 server
func (s *Server) Start(ctx context.Context) error {
	// Create custom dialer that uses our routing engine
	dialer := &RouterDialer{
		acl:           s.acl,
		router:        s.router,
		dialerFactory: s.dialerFactory,
		timeout:       s.timeout,
	}

	// Create SOCKS5 server configuration
	conf := &socks5.Config{
		Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
		AuthMethods: []socks5.Authenticator{
			&socks5.NoAuthAuthenticator{}, // Auth off by default
		},
	}

	// Create SOCKS5 server
	server, err := socks5.New(conf)
	if err != nil {
		return fmt.Errorf("failed to create SOCKS5 server: %w", err)
	}

	fmt.Printf("SOCKS5 server listening on %s\n", s.listenAddr)

	// Start server in a goroutine
	go func() {
		if err := server.ListenAndServe("tcp", s.listenAddr); err != nil {
			fmt.Printf("SOCKS5 server error: %v\n", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	fmt.Println("SOCKS5 server shutting down...")
	return nil
}

// RouterDialer implements the dialer interface for SOCKS5
type RouterDialer struct {
	acl           *acl.ACL
	router        *router.Router
	dialerFactory *router.DialerFactory
	timeout       time.Duration
}

// Dial implements the dialer interface
func (d *RouterDialer) Dial(network, addr string) (net.Conn, error) {
	// Extract client IP from context (SOCKS5 doesn't provide this directly)
	// For now, we'll use a placeholder - in a real implementation, you'd need to
	// pass this through the SOCKS5 context or use connection tracking
	clientIP := "127.0.0.1" // Placeholder - would need connection tracking

	// Check ACL
	allowed, err := d.acl.IsAllowed(context.Background(), clientIP)
	if err != nil {
		return nil, fmt.Errorf("ACL check failed: %w", err)
	}
	if !allowed {
		return nil, fmt.Errorf("access denied for client %s", clientIP)
	}

	// Parse target address
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid address format: %w", err)
	}

	// Find route using routing engine
	route, err := d.router.FindRoute(context.Background(), clientIP, host)
	if err != nil {
		return nil, fmt.Errorf("failed to find route: %w", err)
	}

	// Create dialer based on route
	dialer, err := d.dialerFactory.CreateDialer(context.Background(), route)
	if err != nil {
		return nil, fmt.Errorf("failed to create dialer: %w", err)
	}

	// Dial with timeout
	dialCtx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()

	return dialer.DialContext(dialCtx, network, addr)
}
