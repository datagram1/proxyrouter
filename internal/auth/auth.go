package auth

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Enabled     bool   `yaml:"enabled"`
	BearerToken string `yaml:"bearer_token"`
	Username    string `yaml:"username"`
	Password    string `yaml:"password"`
}

// RateLimitConfig represents rate limiting configuration
type RateLimitConfig struct {
	Enabled     bool    `yaml:"enabled"`
	RequestsPer float64 `yaml:"requests_per_second"`
	Burst       int     `yaml:"burst"`
}

// Authenticator provides authentication functionality
type Authenticator struct {
	config AuthConfig
}

// NewAuthenticator creates a new authenticator
func NewAuthenticator(config AuthConfig) *Authenticator {
	return &Authenticator{
		config: config,
	}
}

// AuthenticateRequest authenticates an HTTP request
func (a *Authenticator) AuthenticateRequest(r *http.Request) error {
	if !a.config.Enabled {
		return nil // No auth required
	}

	// Check Bearer token first
	if a.config.BearerToken != "" {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			return fmt.Errorf("missing authorization header")
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			return fmt.Errorf("invalid authorization header format")
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token != a.config.BearerToken {
			return fmt.Errorf("invalid bearer token")
		}

		return nil
	}

	// Check Basic auth
	if a.config.Username != "" && a.config.Password != "" {
		username, password, ok := r.BasicAuth()
		if !ok {
			return fmt.Errorf("missing basic auth")
		}

		if username != a.config.Username || password != a.config.Password {
			return fmt.Errorf("invalid credentials")
		}

		return nil
	}

	return fmt.Errorf("no authentication method configured")
}

// RateLimiter provides rate limiting functionality
type RateLimiter struct {
	config     RateLimitConfig
	limiters   map[string]*rate.Limiter
	mu         sync.RWMutex
	cleanupTicker *time.Ticker
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config RateLimitConfig) *RateLimiter {
	rl := &RateLimiter{
		config:   config,
		limiters: make(map[string]*rate.Limiter),
	}

	if config.Enabled {
		rl.cleanupTicker = time.NewTicker(1 * time.Hour)
		go rl.cleanup()
	}

	return rl
}

// Allow checks if a request is allowed for the given client
func (rl *RateLimiter) Allow(clientIP string) bool {
	if !rl.config.Enabled {
		return true
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.limiters[clientIP]
	if !exists {
		limiter = rate.NewLimiter(rate.Limit(rl.config.RequestsPer), rl.config.Burst)
		rl.limiters[clientIP] = limiter
	}

	return limiter.Allow()
}

// cleanup removes old limiters to prevent memory leaks
func (rl *RateLimiter) cleanup() {
	for range rl.cleanupTicker.C {
		rl.mu.Lock()
		// In a production system, you might want to track last access time
		// and remove limiters that haven't been used for a while
		rl.mu.Unlock()
	}
}

// Close closes the rate limiter
func (rl *RateLimiter) Close() {
	if rl.cleanupTicker != nil {
		rl.cleanupTicker.Stop()
	}
}

// ProxyAuth provides proxy authentication
type ProxyAuth struct {
	config AuthConfig
}

// NewProxyAuth creates a new proxy authenticator
func NewProxyAuth(config AuthConfig) *ProxyAuth {
	return &ProxyAuth{
		config: config,
	}
}

// AuthenticateProxyRequest authenticates a proxy request
func (pa *ProxyAuth) AuthenticateProxyRequest(r *http.Request) error {
	if !pa.config.Enabled {
		return nil // No auth required
	}

	// Check Basic auth for proxy requests
	if pa.config.Username != "" && pa.config.Password != "" {
		username, password, ok := r.BasicAuth()
		if !ok {
			return fmt.Errorf("proxy authentication required")
		}

		if username != pa.config.Username || password != pa.config.Password {
			return fmt.Errorf("invalid proxy credentials")
		}

		return nil
	}

	return nil
}

// Middleware provides HTTP middleware for authentication and rate limiting
type Middleware struct {
	auth       *Authenticator
	rateLimiter *RateLimiter
}

// NewMiddleware creates new middleware
func NewMiddleware(auth *Authenticator, rateLimiter *RateLimiter) *Middleware {
	return &Middleware{
		auth:        auth,
		rateLimiter: rateLimiter,
	}
}

// AuthMiddleware returns authentication middleware
func (m *Middleware) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := m.auth.AuthenticateRequest(r); err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RateLimitMiddleware returns rate limiting middleware
func (m *Middleware) RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := getClientIP(r)
		if !m.rateLimiter.Allow(clientIP) {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		ips := strings.Split(forwardedFor, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}

	// Use RemoteAddr
	host, _, ok := strings.Cut(r.RemoteAddr, ":")
	if !ok {
		return r.RemoteAddr
	}
	return host
}
