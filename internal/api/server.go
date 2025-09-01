package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"proxyrouter/internal/acl"
	"proxyrouter/internal/router"
)

// Server represents the API server
type Server struct {
	listenAddr string
	acl        *acl.ACL
	router     *router.Router
	chiRouter  *chi.Mux
}

// New creates a new API server
func New(listenAddr string, acl *acl.ACL, router *router.Router) *Server {
	s := &Server{
		listenAddr: listenAddr,
		acl:        acl,
		router:     router,
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

	// API v1 routes
	s.chiRouter.Route("/v1", func(r chi.Router) {
		// Health check
		r.Get("/healthz", s.handleHealthz)

		// Metrics
		r.Get("/metrics", s.handleMetrics)

		// ACL routes
		r.Route("/acl", func(r chi.Router) {
			r.Get("/", s.handleGetACL)
			r.Post("/", s.handleCreateACL)
			r.Delete("/{id}", s.handleDeleteACL)
		})

		// Routes
		r.Route("/routes", func(r chi.Router) {
			r.Get("/", s.handleGetRoutes)
			r.Post("/", s.handleCreateRoute)
			r.Patch("/{id}", s.handleUpdateRoute)
			r.Delete("/{id}", s.handleDeleteRoute)
		})

		// Proxies
		r.Route("/proxies", func(r chi.Router) {
			r.Get("/", s.handleGetProxies)
			r.Post("/import", s.handleImportProxies)
			r.Post("/refresh", s.handleRefreshProxies)
			r.Post("/{id}/check", s.handleCheckProxy)
			r.Delete("/{id}", s.handleDeleteProxy)
		})

		// Settings
		r.Route("/settings", func(r chi.Router) {
			r.Get("/", s.handleGetSettings)
			r.Patch("/", s.handleUpdateSettings)
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

// handleHealthz handles health check requests
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleMetrics handles Prometheus metrics requests
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	promhttp.Handler().ServeHTTP(w, r)
}

// handleGetACL handles GET /v1/acl
func (s *Server) handleGetACL(w http.ResponseWriter, r *http.Request) {
	subnets, err := s.acl.GetSubnets(r.Context())
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "Failed to get ACL subnets", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(subnets)
}

// handleCreateACL handles POST /v1/acl
func (s *Server) handleCreateACL(w http.ResponseWriter, r *http.Request) {
	var request struct {
		CIDR string `json:"cidr"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	if request.CIDR == "" {
		s.sendError(w, http.StatusBadRequest, "CIDR is required", nil)
		return
	}

	if err := s.acl.AddSubnet(r.Context(), request.CIDR); err != nil {
		s.sendError(w, http.StatusBadRequest, "Failed to add subnet", err)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// handleDeleteACL handles DELETE /v1/acl/{id}
func (s *Server) handleDeleteACL(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	if err := s.acl.RemoveSubnet(r.Context(), id); err != nil {
		s.sendError(w, http.StatusNotFound, "Failed to remove subnet", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleGetRoutes handles GET /v1/routes
func (s *Server) handleGetRoutes(w http.ResponseWriter, r *http.Request) {
	routes, err := s.router.GetRoutes(r.Context())
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "Failed to get routes", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(routes)
}

// handleCreateRoute handles POST /v1/routes
func (s *Server) handleCreateRoute(w http.ResponseWriter, r *http.Request) {
	var route router.Route
	if err := json.NewDecoder(r.Body).Decode(&route); err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	if route.Group == "" {
		s.sendError(w, http.StatusBadRequest, "Group is required", nil)
		return
	}

	if err := s.router.CreateRoute(r.Context(), &route); err != nil {
		s.sendError(w, http.StatusBadRequest, "Failed to create route", err)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// handleUpdateRoute handles PATCH /v1/routes/{id}
func (s *Server) handleUpdateRoute(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	if err := s.router.UpdateRoute(r.Context(), id, updates); err != nil {
		s.sendError(w, http.StatusNotFound, "Failed to update route", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleDeleteRoute handles DELETE /v1/routes/{id}
func (s *Server) handleDeleteRoute(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	if err := s.router.DeleteRoute(r.Context(), id); err != nil {
		s.sendError(w, http.StatusNotFound, "Failed to delete route", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleGetProxies handles GET /v1/proxies
func (s *Server) handleGetProxies(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement proxy listing
	s.sendError(w, http.StatusNotImplemented, "Not implemented", nil)
}

// handleImportProxies handles POST /v1/proxies/import
func (s *Server) handleImportProxies(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement proxy import
	s.sendError(w, http.StatusNotImplemented, "Not implemented", nil)
}

// handleRefreshProxies handles POST /v1/proxies/refresh
func (s *Server) handleRefreshProxies(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement proxy refresh
	s.sendError(w, http.StatusNotImplemented, "Not implemented", nil)
}

// handleCheckProxy handles POST /v1/proxies/{id}/check
func (s *Server) handleCheckProxy(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement proxy health check
	s.sendError(w, http.StatusNotImplemented, "Not implemented", nil)
}

// handleDeleteProxy handles DELETE /v1/proxies/{id}
func (s *Server) handleDeleteProxy(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement proxy deletion
	s.sendError(w, http.StatusNotImplemented, "Not implemented", nil)
}

// handleGetSettings handles GET /v1/settings
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement settings retrieval
	s.sendError(w, http.StatusNotImplemented, "Not implemented", nil)
}

// handleUpdateSettings handles PATCH /v1/settings
func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement settings update
	s.sendError(w, http.StatusNotImplemented, "Not implemented", nil)
}

// sendError sends an error response
func (s *Server) sendError(w http.ResponseWriter, statusCode int, message string, err error) {
	response := map[string]interface{}{
		"error":   message,
		"status":  statusCode,
		"time":    time.Now().UTC().Format(time.RFC3339),
	}

	if err != nil {
		response["details"] = err.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}
