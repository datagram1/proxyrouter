package proxysocks

import (
	"context"
	"fmt"
	"net"

	"proxyrouter/internal/acl"
	"proxyrouter/internal/router"
)

// Server represents the SOCKS5 proxy server
type Server struct {
	listenAddr    string
	acl           *acl.ACL
	router        *router.Router
	dialerFactory *router.DialerFactory
}

// New creates a new SOCKS5 proxy server
func New(listenAddr string, acl *acl.ACL, router *router.Router, dialerFactory *router.DialerFactory) *Server {
	return &Server{
		listenAddr:    listenAddr,
		acl:           acl,
		router:        router,
		dialerFactory: dialerFactory,
	}
}

// Start starts the SOCKS5 proxy server
func (s *Server) Start(ctx context.Context) error {
	// TODO: Implement SOCKS5 server when go-socks5 dependency is fixed
	fmt.Printf("SOCKS5 proxy server listening on %s (placeholder - not implemented)\n", s.listenAddr)
	
	// Wait for context cancellation
	<-ctx.Done()
	return ctx.Err()
}

// RouterDialer implements the dialer interface for SOCKS5
type RouterDialer struct {
	acl           *acl.ACL
	router        *router.Router
	dialerFactory *router.DialerFactory
}

// Dial implements the dialer interface
func (d *RouterDialer) Dial(network, addr string) (net.Conn, error) {
	// TODO: Implement when SOCKS5 server is implemented
	return nil, fmt.Errorf("SOCKS5 dialer not implemented")
}
