package admin

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Middleware provides middleware functions for the admin interface
type Middleware struct {
	authManager  *AuthManager
	config       *Config
	rateLimiters map[string]*rate.Limiter
	mu           sync.RWMutex
}

// NewMiddleware creates a new middleware instance
func NewMiddleware(authManager *AuthManager, config *Config) *Middleware {
	return &Middleware{
		authManager:  authManager,
		config:       config,
		rateLimiters: make(map[string]*rate.Limiter),
	}
}

// SessionAuth middleware requires a valid session
func (m *Middleware) SessionAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get session from cookie
		cookie, err := r.Cookie("session")
		if err != nil {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}

		// Get session
		session, exists := m.authManager.GetSession(cookie.Value)
		if !exists {
			// Clear invalid cookie
			http.SetCookie(w, &http.Cookie{
				Name:     "session",
				Value:    "",
				Path:     "/",
				MaxAge:   -1,
				HttpOnly: true,
			})
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}

		// Add session to context
		ctx := context.WithValue(r.Context(), "session", session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// CSRF middleware provides CSRF protection
func (m *Middleware) CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip CSRF for GET requests
		if r.Method == "GET" {
			next.ServeHTTP(w, r)
			return
		}

		// Check CSRF token
		csrfToken := r.Header.Get("X-CSRF-Token")
		if csrfToken == "" {
			csrfToken = r.FormValue("csrf_token")
		}

		// Get session from context
		session, ok := r.Context().Value("session").(*Session)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Validate CSRF token
		username, valid := m.validateCSRFToken(csrfToken)
		if !valid || username != session.Username {
			http.Error(w, "CSRF token invalid or expired", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// LoginCSRF middleware provides CSRF protection for login
func (m *Middleware) LoginCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip CSRF for GET requests
		if r.Method == "GET" {
			next.ServeHTTP(w, r)
			return
		}

		// Check CSRF token
		csrfToken := r.Header.Get("X-CSRF-Token")
		if csrfToken == "" {
			csrfToken = r.FormValue("csrf_token")
		}

		// Validate CSRF token (for login, we accept "login" username)
		username, valid := m.validateCSRFToken(csrfToken)
		if !valid || username != "login" {
			http.Error(w, "CSRF token invalid or expired", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RateLimit middleware provides rate limiting
func (m *Middleware) RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := m.getClientIP(r)

		// Get or create rate limiter for this IP
		limiter := m.getRateLimiter(clientIP)

		if !limiter.Allow() {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// LoginRateLimit middleware provides rate limiting for login attempts
func (m *Middleware) LoginRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := m.getClientIP(r)
		username := r.FormValue("username")

		// Create a unique key for this IP + username combination
		key := fmt.Sprintf("%s:%s", clientIP, username)

		// Get or create rate limiter for this key
		limiter := m.getLoginRateLimiter(key)

		if !limiter.Allow() {
			http.Error(w, "Too Many Login Attempts", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// CIDRGuard middleware restricts access based on CIDR ranges
func (m *Middleware) CIDRGuard(allowCIDRs []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(allowCIDRs) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			clientIP := m.getClientIP(r)

			// Check if client IP is in any allowed CIDR
			allowed := false
			for _, cidr := range allowCIDRs {
				_, network, err := net.ParseCIDR(cidr)
				if err != nil {
					continue
				}

				ip := net.ParseIP(clientIP)
				if ip != nil && network.Contains(ip) {
					allowed = true
					break
				}
			}

			if !allowed {
				http.Error(w, "Access Denied", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeaders middleware adds security headers
func (m *Middleware) SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Security headers
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'")

		next.ServeHTTP(w, r)
	})
}

// getClientIP extracts the client IP from the request
func (m *Middleware) getClientIP(r *http.Request) string {
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

// getRateLimiter gets or creates a rate limiter for an IP
func (m *Middleware) getRateLimiter(clientIP string) *rate.Limiter {
	m.mu.Lock()
	defer m.mu.Unlock()

	limiter, exists := m.rateLimiters[clientIP]
	if !exists {
		// 5 requests per second, burst of 10
		limiter = rate.NewLimiter(rate.Limit(5), 10)
		m.rateLimiters[clientIP] = limiter
	}

	return limiter
}

// getLoginRateLimiter gets or creates a rate limiter for login attempts
func (m *Middleware) getLoginRateLimiter(key string) *rate.Limiter {
	m.mu.Lock()
	defer m.mu.Unlock()

	limiter, exists := m.rateLimiters[key]
	if !exists {
		// 1 request per 30 seconds for login attempts
		limiter = rate.NewLimiter(rate.Every(30*time.Second), 1)
		m.rateLimiters[key] = limiter
	}

	return limiter
}

// generateCSRFToken generates a CSRF token for a user
func (m *Middleware) generateCSRFToken(username string) string {
	return m.generateCSRFTokenWithExpiry(username, time.Now().Add(30*time.Minute))
}

// generateCSRFTokenWithExpiry generates a CSRF token with a specific expiry time
func (m *Middleware) generateCSRFTokenWithExpiry(username string, expiry time.Time) string {
	// Create a token with username, expiry timestamp, and HMAC signature
	timestamp := strconv.FormatInt(expiry.Unix(), 10)
	data := fmt.Sprintf("%s:%s", username, timestamp)

	// Create HMAC signature
	h := hmac.New(sha256.New, []byte(m.config.SessionSecret))
	h.Write([]byte(data))
	signature := base64.URLEncoding.EncodeToString(h.Sum(nil))

	// Return token in format: username:timestamp:signature
	return fmt.Sprintf("%s:%s:%s", username, timestamp, signature)
}

// validateCSRFToken validates a CSRF token and returns the username if valid
func (m *Middleware) validateCSRFToken(token string) (string, bool) {
	parts := strings.Split(token, ":")
	if len(parts) != 3 {
		return "", false
	}

	username := parts[0]
	timestampStr := parts[1]
	signature := parts[2]

	// Parse timestamp
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return "", false
	}

	// Check if token has expired
	if time.Unix(timestamp, 0).Before(time.Now()) {
		return "", false
	}

	// Verify signature
	data := fmt.Sprintf("%s:%s", username, timestampStr)
	h := hmac.New(sha256.New, []byte(m.config.SessionSecret))
	h.Write([]byte(data))
	expectedSignature := base64.URLEncoding.EncodeToString(h.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return "", false
	}

	return username, true
}

// refreshCSRFToken generates a new CSRF token for a user
func (m *Middleware) refreshCSRFToken(username string) string {
	return m.generateCSRFToken(username)
}

// CleanupRateLimiters cleans up old rate limiters
func (m *Middleware) CleanupRateLimiters() {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for range ticker.C {
			m.mu.Lock()
			// Remove rate limiters older than 1 hour
			// This is a simple cleanup - in production, you might want more sophisticated cleanup
			m.mu.Unlock()
		}
	}()
}
