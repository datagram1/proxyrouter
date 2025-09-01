package proxyhttp

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"proxyrouter/internal/acl"
	"proxyrouter/internal/router"
)

// Server represents the HTTP proxy server
type Server struct {
	listenAddr string
	acl        *acl.ACL
	router     *router.Router
	dialerFactory *router.DialerFactory
	timeout    time.Duration
}

// New creates a new HTTP proxy server
func New(listenAddr string, acl *acl.ACL, router *router.Router, dialerFactory *router.DialerFactory, timeout time.Duration) *Server {
	return &Server{
		listenAddr:    listenAddr,
		acl:           acl,
		router:        router,
		dialerFactory: dialerFactory,
		timeout:       timeout,
	}
}

// Start starts the HTTP proxy server
func (s *Server) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.listenAddr, err)
	}
	defer listener.Close()

	fmt.Printf("HTTP proxy server listening on %s\n", s.listenAddr)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			conn, err := listener.Accept()
			if err != nil {
				fmt.Printf("Failed to accept connection: %v\n", err)
				continue
			}

			go s.handleConnection(ctx, conn)
		}
	}
}

// handleConnection handles a single client connection
func (s *Server) handleConnection(ctx context.Context, clientConn net.Conn) {
	defer clientConn.Close()

	// Set connection deadline
	clientConn.SetDeadline(time.Now().Add(s.timeout))

	// Extract client IP
	clientIP := acl.ExtractClientIP(clientConn.RemoteAddr().String(), nil)

	// Check ACL
	allowed, err := s.acl.IsAllowed(ctx, clientIP)
	if err != nil {
		fmt.Printf("ACL check failed for %s: %v\n", clientIP, err)
		return
	}

	if !allowed {
		fmt.Printf("Access denied for %s\n", clientIP)
		s.sendForbiddenResponse(clientConn)
		return
	}

	// Read the first line to determine the request type
	reader := bufio.NewReader(clientConn)
	firstLine, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Failed to read request: %v\n", err)
		return
	}

	// Parse the request line
	method, target, version, err := s.parseRequestLine(firstLine)
	if err != nil {
		fmt.Printf("Failed to parse request line: %v\n", err)
		return
	}

	// Handle CONNECT method for HTTPS tunneling
	if method == "CONNECT" {
		s.handleCONNECT(ctx, clientConn, target, version, reader)
		return
	}

	// Handle regular HTTP requests
	s.handleHTTPRequest(ctx, clientConn, method, target, version, reader)
}

// handleCONNECT handles HTTPS CONNECT tunneling
func (s *Server) handleCONNECT(ctx context.Context, clientConn net.Conn, target, version string, reader *bufio.Reader) {
	// Extract host and port from target
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		// Default to port 443 if not specified
		host = target
		port = "443"
		target = net.JoinHostPort(host, port)
	}

	// Find route for this target
	route, err := s.router.FindRoute(ctx, acl.ExtractClientIP(clientConn.RemoteAddr().String(), nil), host)
	if err != nil {
		fmt.Printf("Failed to find route for %s: %v\n", host, err)
		s.sendErrorResponse(clientConn, "502 Bad Gateway")
		return
	}

	// Create dialer for the route
	dialer, err := s.dialerFactory.CreateDialer(ctx, route)
	if err != nil {
		fmt.Printf("Failed to create dialer for route %s: %v\n", route.Group, err)
		s.sendErrorResponse(clientConn, "502 Bad Gateway")
		return
	}

	// Connect to target
	targetConn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		fmt.Printf("Failed to connect to %s: %v\n", target, err)
		s.sendErrorResponse(clientConn, "502 Bad Gateway")
		return
	}
	defer targetConn.Close()

	// Send success response to client
	response := fmt.Sprintf("%s 200 Connection established\r\n\r\n", version)
	if _, err := clientConn.Write([]byte(response)); err != nil {
		fmt.Printf("Failed to send CONNECT response: %v\n", err)
		return
	}

	// Tunnel data between client and target
	s.tunnelData(clientConn, targetConn)
}

// handleHTTPRequest handles regular HTTP requests
func (s *Server) handleHTTPRequest(ctx context.Context, clientConn net.Conn, method, target, version string, reader *bufio.Reader) {
	// Parse the target URL
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		// Default to port 80 if not specified
		host = target
		port = "80"
		target = net.JoinHostPort(host, port)
	}

	// Find route for this target
	route, err := s.router.FindRoute(ctx, acl.ExtractClientIP(clientConn.RemoteAddr().String(), nil), host)
	if err != nil {
		fmt.Printf("Failed to find route for %s: %v\n", host, err)
		s.sendErrorResponse(clientConn, "502 Bad Gateway")
		return
	}

	// Create dialer for the route
	dialer, err := s.dialerFactory.CreateDialer(ctx, route)
	if err != nil {
		fmt.Printf("Failed to create dialer for route %s: %v\n", route.Group, err)
		s.sendErrorResponse(clientConn, "502 Bad Gateway")
		return
	}

	// Connect to target
	targetConn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		fmt.Printf("Failed to connect to %s: %v\n", target, err)
		s.sendErrorResponse(clientConn, "502 Bad Gateway")
		return
	}
	defer targetConn.Close()

	// Forward the request
	if err := s.forwardHTTPRequest(targetConn, method, target, version, reader); err != nil {
		fmt.Printf("Failed to forward HTTP request: %v\n", err)
		return
	}

	// Forward the response
	if err := s.forwardHTTPResponse(clientConn, targetConn); err != nil {
		fmt.Printf("Failed to forward HTTP response: %v\n", err)
		return
	}
}

// parseRequestLine parses the HTTP request line
func (s *Server) parseRequestLine(line string) (method, target, version string, err error) {
	parts := strings.Fields(strings.TrimSpace(line))
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid request line: %s", line)
	}
	return parts[0], parts[1], parts[2], nil
}

// forwardHTTPRequest forwards an HTTP request to the target
func (s *Server) forwardHTTPRequest(targetConn net.Conn, method, target, version string, reader *bufio.Reader) error {
	// Write the request line
	requestLine := fmt.Sprintf("%s %s %s\r\n", method, target, version)
	if _, err := targetConn.Write([]byte(requestLine)); err != nil {
		return err
	}

	// Forward headers
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}

		if _, err := targetConn.Write([]byte(line)); err != nil {
			return err
		}

		// Empty line indicates end of headers
		if line == "\r\n" || line == "\n" {
			break
		}
	}

	// Forward body if present
	if _, err := io.Copy(targetConn, reader); err != nil {
		return err
	}

	return nil
}

// forwardHTTPResponse forwards an HTTP response to the client
func (s *Server) forwardHTTPResponse(clientConn net.Conn, targetConn net.Conn) error {
	_, err := io.Copy(clientConn, targetConn)
	return err
}

// tunnelData tunnels data between two connections
func (s *Server) tunnelData(conn1, conn2 net.Conn) {
	// Create channels for coordination
	done := make(chan bool, 2)

	// Copy from conn1 to conn2
	go func() {
		io.Copy(conn2, conn1)
		done <- true
	}()

	// Copy from conn2 to conn1
	go func() {
		io.Copy(conn1, conn2)
		done <- true
	}()

	// Wait for either direction to finish
	<-done
}

// sendForbiddenResponse sends a 403 Forbidden response
func (s *Server) sendForbiddenResponse(conn net.Conn) {
	response := "HTTP/1.1 403 Forbidden\r\nContent-Length: 0\r\n\r\n"
	conn.Write([]byte(response))
}

// sendErrorResponse sends an error response
func (s *Server) sendErrorResponse(conn net.Conn, status string) {
	response := fmt.Sprintf("HTTP/1.1 %s\r\nContent-Length: 0\r\n\r\n", status)
	conn.Write([]byte(response))
}
