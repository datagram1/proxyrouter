package admin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"proxyrouter/internal/config"
	"proxyrouter/internal/db"

	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server represents the admin HTTP server
type Server struct {
	config      *config.Config
	database    *db.Database
	authManager *AuthManager
	middleware  *Middleware
	handlers    *Handlers
	server      *http.Server
}

// NewServer creates a new admin server
func NewServer(cfg *config.Config, database *db.Database) *Server {
	// Auto-generate session secret if empty
	sessionSecret := cfg.Admin.SessionSecret
	if sessionSecret == "" {
		sessionSecret = generateSessionSecret()
		slog.Info("Auto-generated session secret", "length", len(sessionSecret))
	}

	// Create auth manager
	authConfig := &Config{
		SessionSecret: sessionSecret,
		PasswordHash:  cfg.Security.PasswordHash,
		MaxAttempts:   cfg.Security.Login.MaxAttempts,
		WindowSeconds: cfg.Security.Login.WindowSeconds,
	}

	authManager := NewAuthManager(database.GetDB(), authConfig)

	// Create middleware
	mw := NewMiddleware(authManager, authConfig)

	// Create handlers
	handlers := NewHandlers(cfg, database, authManager, mw)

	// Create server
	s := &Server{
		config:      cfg,
		database:    database,
		authManager: authManager,
		middleware:  mw,
		handlers:    handlers,
	}

	// Setup routes
	s.setupRoutes()

	return s
}

// generateSessionSecret generates a random session secret
func generateSessionSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// setupRoutes sets up all admin routes
func (s *Server) setupRoutes() {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(s.middleware.SecurityHeaders)
	r.Use(s.middleware.CIDRGuard(s.config.Admin.AllowCIDRs))
	r.Use(s.middleware.RateLimit)

	// Root redirect to admin login
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, s.config.Admin.BasePath+"/login", http.StatusSeeOther)
	})

	// Admin routes
	r.Route(s.config.Admin.BasePath, func(admin chi.Router) {
		// Public routes (no auth required)
		admin.Get("/login", s.handlers.ShowLogin)
		admin.Get("/csrf-login", s.handlers.GetLoginCSRFToken)

		// Login route with CSRF protection
		admin.Group(func(login chi.Router) {
			login.Use(s.middleware.LoginCSRF)
			login.Post("/login", s.handlers.DoLogin)
		})

		// Protected routes (auth required)
		admin.Group(func(protected chi.Router) {
			protected.Use(s.middleware.SessionAuth)
			protected.Use(s.middleware.CSRF)

			// Dashboard
			protected.Get("/", s.handlers.Dashboard)
			protected.Get("/health", s.handlers.HealthSummary)

			// Settings
			protected.Get("/settings", s.handlers.GetSettings)
			protected.Post("/settings", s.handlers.PostSettings)

			// Upload
			protected.Get("/upload", s.handlers.UploadForm)
			protected.Post("/upload", s.handlers.UploadProxies)

			// Users
			protected.Get("/users", s.handlers.ListUsers)
			protected.Get("/users/change-password", s.handlers.ChangePassword)
			protected.Post("/users/change-password", s.handlers.ChangePassword)
			protected.Post("/users/create", s.handlers.CreateUser)

			// CSRF token refresh
			protected.Get("/csrf-refresh", s.handlers.RefreshCSRFToken)

			// Logout
			protected.Post("/logout", s.handlers.DoLogout)
		})
	})

	// Create HTTP server
	s.server = &http.Server{
		Addr:         net.JoinHostPort(s.config.Admin.Bind, strconv.Itoa(s.config.Admin.Port)),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// Start starts the admin server
func (s *Server) Start(ctx context.Context) error {
	// Start rate limiter cleanup
	s.middleware.CleanupRateLimiters()

	// Bootstrap admin user if needed
	if err := s.bootstrapAdminUser(ctx); err != nil {
		return fmt.Errorf("failed to bootstrap admin user: %w", err)
	}

	// Start server
	go func() {
		var err error
		if s.config.Admin.TLS.Enabled {
			slog.Info("Admin server starting (TLS)",
				"addr", s.server.Addr,
				"cert", s.config.Admin.TLS.CertFile,
				"key", s.config.Admin.TLS.KeyFile)
			err = s.server.ListenAndServeTLS(s.config.Admin.TLS.CertFile, s.config.Admin.TLS.KeyFile)
		} else {
			slog.Info("Admin server starting", "addr", s.server.Addr)
			err = s.server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			slog.Error("Admin server error", "error", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Shutdown server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("failed to shutdown admin server: %w", err)
	}

	slog.Info("Admin server stopped")
	return nil
}

// bootstrapAdminUser creates the default admin user if no users exist
func (s *Server) bootstrapAdminUser(ctx context.Context) error {
	// Check if any users exist
	var count int
	query := `SELECT COUNT(*) FROM admin_users`
	err := s.database.GetDB().QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check user count: %w", err)
	}

	if count == 0 {
		// Create default admin user
		hash, err := s.authManager.hashPassword("admin")
		if err != nil {
			return fmt.Errorf("failed to hash default password: %w", err)
		}

		query := `INSERT INTO admin_users (username, password_hash, force_change) VALUES (?, ?, 0)`
		_, err = s.database.GetDB().ExecContext(ctx, query, "admin", hash)
		if err != nil {
			return fmt.Errorf("failed to create default admin user: %w", err)
		}

		slog.Info("Created default admin user", "username", "admin", "password", "admin")
	}

	return nil
}
