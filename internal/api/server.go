package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"proxyrouter/internal/acl"
	"proxyrouter/internal/config"
	"proxyrouter/internal/db"
	"proxyrouter/internal/refresh"
	"proxyrouter/internal/router"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server represents the API server
type Server struct {
	listenAddr string
	handler    *Handler
	chiRouter  *chi.Mux
}

// New creates a new API server
func New(listenAddr string, db *db.Database, acl *acl.ACL, router *router.Router, refresher *refresh.Refresher, config *config.Config) *Server {
	handler := NewHandler(db, acl, router, refresher, config)
	s := &Server{
		listenAddr: listenAddr,
		handler:    handler,
		chiRouter:  chi.NewRouter(),
	}

	s.setupRoutes()
	return s
}

// setupRoutes sets up all API routes
func (s *Server) setupRoutes() {
	// Middleware
	s.chiRouter.Use(middleware.Logger)
	s.chiRouter.Use(middleware.Recoverer)
	s.chiRouter.Use(middleware.Timeout(60 * time.Second))

	// Health check and metrics
	s.chiRouter.Get("/healthz", s.handler.HealthCheck)
	s.chiRouter.Get("/metrics", s.metrics)

	// API v1 routes
	s.chiRouter.Route("/api/v1", func(r chi.Router) {
		// ACL routes
		r.Route("/acl", func(r chi.Router) {
			r.Get("/", s.handler.GetACL)
			r.Post("/", s.handler.AddACL)
			r.Delete("/{id}", s.handler.DeleteACL)
		})

		// Routes
		r.Route("/routes", func(r chi.Router) {
			r.Get("/", s.handler.GetRoutes)
			r.Post("/", s.handler.CreateRoute)
			r.Put("/{id}", s.handler.UpdateRoute)
			r.Delete("/{id}", s.handler.DeleteRoute)
		})

		// Proxies
		r.Route("/proxies", func(r chi.Router) {
			r.Get("/", s.handler.GetProxies)
			r.Post("/import", s.handler.ImportProxies)
			r.Post("/refresh", s.handler.RefreshProxies)
			r.Post("/{id}/check", s.handler.CheckProxy)
			r.Delete("/{id}", s.handler.DeleteProxy)
		})

		// Settings
		r.Route("/settings", func(r chi.Router) {
			r.Get("/", s.handler.GetSettings)
			r.Patch("/", s.handler.UpdateSettings)
		})
	})
}

// Start starts the API server
func (s *Server) Start(ctx context.Context) error {
	fmt.Printf("API server listening on %s\n", s.listenAddr)
	
	server := &http.Server{
		Addr:    s.listenAddr,
		Handler: s.chiRouter,
	}

	// Start server in a goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("API server error: %v\n", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Shutdown server gracefully
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return server.Shutdown(shutdownCtx)
}

// metrics handles Prometheus metrics requests
func (s *Server) metrics(w http.ResponseWriter, r *http.Request) {
	promhttp.Handler().ServeHTTP(w, r)
}
