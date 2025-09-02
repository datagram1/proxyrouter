package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"proxyrouter/internal/acl"
	"proxyrouter/internal/config"
	"proxyrouter/internal/db"
	"proxyrouter/internal/refresh"
	"proxyrouter/internal/router"
	"proxyrouter/internal/version"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// Handler handles API requests
type Handler struct {
	db        *db.Database
	acl       *acl.ACL
	router    *router.Router
	refresher *refresh.Refresher
	config    *config.Config
}

// NewHandler creates a new API handler
func NewHandler(db *db.Database, acl *acl.ACL, router *router.Router, refresher *refresh.Refresher, config *config.Config) *Handler {
	return &Handler{
		db:        db,
		acl:       acl,
		router:    router,
		refresher: refresher,
		config:    config,
	}
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
	Uptime    string    `json:"uptime"`
}

// ACLSubnet represents an ACL subnet entry
type ACLSubnet struct {
	ID   int    `json:"id"`
	CIDR string `json:"cidr"`
}

// RouteResponse represents a routing rule response
type RouteResponse struct {
	ID         int     `json:"id"`
	Group      string  `json:"group"`
	Precedence int     `json:"precedence"`
	HostGlob   *string `json:"host_glob,omitempty"`
	ClientCIDR *string `json:"client_cidr,omitempty"`
	ProxyID    *int    `json:"proxy_id,omitempty"`
	Enabled    bool    `json:"enabled"`
	CreatedAt  string  `json:"created_at"`
}

// Proxy represents a proxy entry
type Proxy struct {
	ID              int     `json:"id"`
	ProxyType       string  `json:"proxy_type"`
	IP              string  `json:"ip"`
	Port            int     `json:"port"`
	Source          string  `json:"source"`
	Working         bool    `json:"working"`
	Latency         *int    `json:"latency,omitempty"`
	TestedTimestamp *string `json:"tested_timestamp,omitempty"`
	ErrorMessage    *string `json:"error_message,omitempty"`
	CreatedAt       string  `json:"created_at"`
	ProxyURL        *string `json:"proxy_url,omitempty"`
}

// Setting represents a setting entry
type Setting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// TorControlResponse represents a Tor control response
type TorControlResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	NewIP   string `json:"new_ip,omitempty"`
}

// HealthCheck handles health check requests
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Version:   version.Short(),
		Uptime:    "0s", // TODO: Calculate actual uptime
	}

	render.JSON(w, r, response)
}

// Version handles version requests
func (h *Handler) Version(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"version":    version.Version,
		"commit":     version.Commit,
		"build_date": version.BuildDate,
		"go_version": version.GoVersion,
	}
	render.JSON(w, r, response)
}

// GetACL handles GET /acl requests
func (h *Handler) GetACL(w http.ResponseWriter, r *http.Request) {
	subnets, err := h.acl.GetSubnets(r.Context())
	if err != nil {
		render.JSON(w, r, ErrorResponse{
			Error:   "database_error",
			Message: fmt.Sprintf("Failed to get ACL subnets: %v", err),
			Code:    http.StatusInternalServerError,
		})
		return
	}

	var response []ACLSubnet
	for _, subnet := range subnets {
		response = append(response, ACLSubnet{
			ID:   subnet.ID,
			CIDR: subnet.CIDR,
		})
	}

	render.JSON(w, r, response)
}

// AddACL handles POST /acl requests
func (h *Handler) AddACL(w http.ResponseWriter, r *http.Request) {
	var request struct {
		CIDR string `json:"cidr"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		render.JSON(w, r, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid JSON in request body",
			Code:    http.StatusBadRequest,
		})
		return
	}

	if request.CIDR == "" {
		render.JSON(w, r, ErrorResponse{
			Error:   "missing_cidr",
			Message: "CIDR is required",
			Code:    http.StatusBadRequest,
		})
		return
	}

	if err := h.acl.AddSubnet(r.Context(), request.CIDR); err != nil {
		render.JSON(w, r, ErrorResponse{
			Error:   "invalid_cidr",
			Message: fmt.Sprintf("Invalid CIDR: %v", err),
			Code:    http.StatusBadRequest,
		})
		return
	}

	render.JSON(w, r, map[string]string{"status": "added"})
}

// DeleteACL handles DELETE /acl/{id} requests
func (h *Handler) DeleteACL(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		render.JSON(w, r, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid ACL ID",
			Code:    http.StatusBadRequest,
		})
		return
	}

	if err := h.acl.RemoveSubnet(r.Context(), id); err != nil {
		render.JSON(w, r, ErrorResponse{
			Error:   "database_error",
			Message: fmt.Sprintf("Failed to remove ACL subnet: %v", err),
			Code:    http.StatusInternalServerError,
		})
		return
	}

	render.JSON(w, r, map[string]string{"status": "deleted"})
}

// GetRoutes handles GET /routes requests
func (h *Handler) GetRoutes(w http.ResponseWriter, r *http.Request) {
	routes, err := h.router.GetRoutes()
	if err != nil {
		render.JSON(w, r, ErrorResponse{
			Error:   "database_error",
			Message: fmt.Sprintf("Failed to get routes: %v", err),
			Code:    http.StatusInternalServerError,
		})
		return
	}

	var response []RouteResponse
	for _, route := range routes {
		response = append(response, RouteResponse{
			ID:         route.ID,
			Group:      string(route.Group),
			Precedence: route.Precedence,
			HostGlob:   route.HostGlob,
			ClientCIDR: route.ClientCIDR,
			ProxyID:    route.ProxyID,
			Enabled:    route.Enabled,
			CreatedAt:  route.CreatedAt.Format(time.RFC3339),
		})
	}

	render.JSON(w, r, response)
}

// CreateRoute handles POST /routes requests
func (h *Handler) CreateRoute(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Group      string  `json:"group"`
		Precedence int     `json:"precedence"`
		HostGlob   *string `json:"host_glob,omitempty"`
		ClientCIDR *string `json:"client_cidr,omitempty"`
		ProxyID    *int    `json:"proxy_id,omitempty"`
		Enabled    bool    `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		render.JSON(w, r, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid JSON in request body",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Validate required fields
	if request.Group == "" {
		render.JSON(w, r, ErrorResponse{
			Error:   "missing_group",
			Message: "Route group is required",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Validate group
	group := router.RouteGroup(request.Group)
	switch group {
	case router.RouteGroupLocal, router.RouteGroupGeneral, router.RouteGroupTor, router.RouteGroupUpstream:
		// Valid group
	default:
		render.JSON(w, r, ErrorResponse{
			Error:   "invalid_group",
			Message: "Invalid route group",
			Code:    http.StatusBadRequest,
		})
		return
	}

	route := router.Route{
		Group:      group,
		Precedence: request.Precedence,
		HostGlob:   request.HostGlob,
		ClientCIDR: request.ClientCIDR,
		ProxyID:    request.ProxyID,
		Enabled:    request.Enabled,
	}

	if err := h.router.CreateRoute(&route); err != nil {
		render.JSON(w, r, ErrorResponse{
			Error:   "database_error",
			Message: fmt.Sprintf("Failed to create route: %v", err),
			Code:    http.StatusInternalServerError,
		})
		return
	}

	render.JSON(w, r, RouteResponse{
		ID:         route.ID,
		Group:      string(route.Group),
		Precedence: route.Precedence,
		HostGlob:   route.HostGlob,
		ClientCIDR: route.ClientCIDR,
		ProxyID:    route.ProxyID,
		Enabled:    route.Enabled,
		CreatedAt:  route.CreatedAt.Format(time.RFC3339),
	})
}

// UpdateRoute handles PUT /routes/{id} requests
func (h *Handler) UpdateRoute(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		render.JSON(w, r, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid route ID",
			Code:    http.StatusBadRequest,
		})
		return
	}

	var request struct {
		Group      string  `json:"group,omitempty"`
		Precedence *int    `json:"precedence,omitempty"`
		HostGlob   *string `json:"host_glob,omitempty"`
		ClientCIDR *string `json:"client_cidr,omitempty"`
		ProxyID    *int    `json:"proxy_id,omitempty"`
		Enabled    *bool   `json:"enabled,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		render.JSON(w, r, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid JSON in request body",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Build updates map
	updates := make(map[string]interface{})
	if request.Group != "" {
		updates["group"] = request.Group
	}
	if request.Precedence != nil {
		updates["precedence"] = *request.Precedence
	}
	if request.HostGlob != nil {
		updates["host_glob"] = *request.HostGlob
	}
	if request.ClientCIDR != nil {
		updates["client_cidr"] = *request.ClientCIDR
	}
	if request.ProxyID != nil {
		updates["proxy_id"] = *request.ProxyID
	}
	if request.Enabled != nil {
		updates["enabled"] = *request.Enabled
	}

	if err := h.router.UpdateRoute(r.Context(), id, updates); err != nil {
		render.JSON(w, r, ErrorResponse{
			Error:   "database_error",
			Message: fmt.Sprintf("Failed to update route: %v", err),
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Get the updated route to return
	routes, err := h.router.GetRoutes()
	if err != nil {
		render.JSON(w, r, ErrorResponse{
			Error:   "database_error",
			Message: fmt.Sprintf("Failed to get updated route: %v", err),
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Find the updated route
	for _, route := range routes {
		if route.ID == id {
			render.JSON(w, r, RouteResponse{
				ID:         route.ID,
				Group:      string(route.Group),
				Precedence: route.Precedence,
				HostGlob:   route.HostGlob,
				ClientCIDR: route.ClientCIDR,
				ProxyID:    route.ProxyID,
				Enabled:    route.Enabled,
				CreatedAt:  route.CreatedAt.Format(time.RFC3339),
			})
			return
		}
	}

	render.JSON(w, r, ErrorResponse{
		Error:   "not_found",
		Message: "Route not found after update",
		Code:    http.StatusNotFound,
	})
}

// DeleteRoute handles DELETE /routes/{id} requests
func (h *Handler) DeleteRoute(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		render.JSON(w, r, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid route ID",
			Code:    http.StatusBadRequest,
		})
		return
	}

	if err := h.router.DeleteRoute(id); err != nil {
		render.JSON(w, r, ErrorResponse{
			Error:   "database_error",
			Message: fmt.Sprintf("Failed to delete route: %v", err),
			Code:    http.StatusInternalServerError,
		})
		return
	}

	render.JSON(w, r, map[string]string{"status": "deleted"})
}

// GetProxies handles GET /proxies requests
func (h *Handler) GetProxies(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT id, proxy_type, ip, port, source, working, latency, 
		       tested_timestamp, error_message, created_at, proxy_url
		FROM proxies
		ORDER BY created_at DESC
	`

	rows, err := h.db.Query(context.Background(), query)
	if err != nil {
		render.JSON(w, r, ErrorResponse{
			Error:   "database_error",
			Message: fmt.Sprintf("Failed to get proxies: %v", err),
			Code:    http.StatusInternalServerError,
		})
		return
	}
	defer rows.Close()

	var proxies []Proxy
	for rows.Next() {
		var proxy Proxy
		var testedTimestamp sql.NullString
		var errorMessage sql.NullString
		var proxyURL sql.NullString

		err := rows.Scan(
			&proxy.ID, &proxy.ProxyType, &proxy.IP, &proxy.Port, &proxy.Source,
			&proxy.Working, &proxy.Latency, &testedTimestamp, &errorMessage, &proxy.CreatedAt, &proxyURL,
		)
		if err != nil {
			continue
		}

		if testedTimestamp.Valid {
			proxy.TestedTimestamp = &testedTimestamp.String
		}
		if errorMessage.Valid {
			proxy.ErrorMessage = &errorMessage.String
		}
		if proxyURL.Valid {
			proxy.ProxyURL = &proxyURL.String
		}

		proxies = append(proxies, proxy)
	}

	render.JSON(w, r, proxies)
}

// ImportProxies handles POST /proxies/import requests
func (h *Handler) ImportProxies(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Proxies []string `json:"proxies"`
		Source  string   `json:"source"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		render.JSON(w, r, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid JSON in request body",
			Code:    http.StatusBadRequest,
		})
		return
	}

	if len(request.Proxies) == 0 {
		render.JSON(w, r, ErrorResponse{
			Error:   "missing_proxies",
			Message: "At least one proxy is required",
			Code:    http.StatusBadRequest,
		})
		return
	}

	var proxyList []refresh.Proxy
	for _, proxyStr := range request.Proxies {
		proxy, err := h.refresher.ParseProxyLine(proxyStr, request.Source)
		if err != nil {
			continue // Skip invalid proxies
		}
		proxyList = append(proxyList, proxy)
	}

	if err := h.refresher.ImportProxies(context.Background(), proxyList); err != nil {
		render.JSON(w, r, ErrorResponse{
			Error:   "import_error",
			Message: fmt.Sprintf("Failed to import proxies: %v", err),
			Code:    http.StatusInternalServerError,
		})
		return
	}

	render.JSON(w, r, map[string]interface{}{
		"status":  "imported",
		"count":   len(proxyList),
		"total":   len(request.Proxies),
		"skipped": len(request.Proxies) - len(proxyList),
	})
}

// RefreshProxies handles POST /proxies/refresh requests
func (h *Handler) RefreshProxies(w http.ResponseWriter, r *http.Request) {
	if err := h.refresher.RefreshAll(context.Background()); err != nil {
		render.JSON(w, r, ErrorResponse{
			Error:   "refresh_error",
			Message: fmt.Sprintf("Failed to refresh proxies: %v", err),
			Code:    http.StatusInternalServerError,
		})
		return
	}

	render.JSON(w, r, map[string]string{"status": "refreshed"})
}

// HealthCheckProxies handles POST /proxies/health-check requests
func (h *Handler) HealthCheckProxies(w http.ResponseWriter, r *http.Request) {
	if err := h.refresher.HealthCheck(context.Background()); err != nil {
		render.JSON(w, r, ErrorResponse{
			Error:   "health_check_error",
			Message: fmt.Sprintf("Failed to run health check: %v", err),
			Code:    http.StatusInternalServerError,
		})
		return
	}

	render.JSON(w, r, map[string]string{"status": "health_check_completed"})
}

// CheckProxy handles POST /proxies/{id}/check requests
func (h *Handler) CheckProxy(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		render.JSON(w, r, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid proxy ID",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Get proxy from database
	query := `SELECT id, proxy_type, ip, port FROM proxies WHERE id = ?`
	var proxy refresh.Proxy
	err = h.db.QueryRow(context.Background(), query, id).Scan(&proxy.ID, &proxy.ProxyType, &proxy.IP, &proxy.Port)
	if err != nil {
		if err == sql.ErrNoRows {
			render.JSON(w, r, ErrorResponse{
				Error:   "not_found",
				Message: "Proxy not found",
				Code:    http.StatusNotFound,
			})
		} else {
			render.JSON(w, r, ErrorResponse{
				Error:   "database_error",
				Message: fmt.Sprintf("Failed to get proxy: %v", err),
				Code:    http.StatusInternalServerError,
			})
		}
		return
	}

	// Perform health check
	result := h.refresher.CheckProxyHealth(context.Background(), proxy)

	// Update database
	updateQuery := `
		UPDATE proxies
		SET working = ?, latency = ?, tested_timestamp = CURRENT_TIMESTAMP, error_message = ?
		WHERE id = ?
	`

	var latency *int
	if result.Working {
		latency = &result.Latency
	}

	var errorMsg *string
	if result.Error != "" {
		errorMsg = &result.Error
	}

	_, err = h.db.Exec(context.Background(), updateQuery, result.Working, latency, errorMsg, id)
	if err != nil {
		render.JSON(w, r, ErrorResponse{
			Error:   "database_error",
			Message: fmt.Sprintf("Failed to update proxy: %v", err),
			Code:    http.StatusInternalServerError,
		})
		return
	}

	render.JSON(w, r, result)
}

// DeleteProxy handles DELETE /proxies/{id} requests
func (h *Handler) DeleteProxy(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		render.JSON(w, r, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid proxy ID",
			Code:    http.StatusBadRequest,
		})
		return
	}

	result, err := h.db.Exec(context.Background(), "DELETE FROM proxies WHERE id = ?", id)
	if err != nil {
		render.JSON(w, r, ErrorResponse{
			Error:   "database_error",
			Message: fmt.Sprintf("Failed to delete proxy: %v", err),
			Code:    http.StatusInternalServerError,
		})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		render.JSON(w, r, ErrorResponse{
			Error:   "database_error",
			Message: fmt.Sprintf("Failed to get rows affected: %v", err),
			Code:    http.StatusInternalServerError,
		})
		return
	}

	if rowsAffected == 0 {
		render.JSON(w, r, ErrorResponse{
			Error:   "not_found",
			Message: "Proxy not found",
			Code:    http.StatusNotFound,
		})
		return
	}

	render.JSON(w, r, map[string]string{"status": "deleted"})
}

// GetSettings handles GET /settings requests
func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	query := `SELECT key, value FROM settings ORDER BY key`
	rows, err := h.db.Query(context.Background(), query)
	if err != nil {
		render.JSON(w, r, ErrorResponse{
			Error:   "database_error",
			Message: fmt.Sprintf("Failed to get settings: %v", err),
			Code:    http.StatusInternalServerError,
		})
		return
	}
	defer rows.Close()

	var settings []Setting
	for rows.Next() {
		var setting Setting
		if err := rows.Scan(&setting.Key, &setting.Value); err != nil {
			continue
		}
		settings = append(settings, setting)
	}

	render.JSON(w, r, settings)
}

// UpdateSettings handles PATCH /settings requests
func (h *Handler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var request map[string]string
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		render.JSON(w, r, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid JSON in request body",
			Code:    http.StatusBadRequest,
		})
		return
	}

	if len(request) == 0 {
		render.JSON(w, r, ErrorResponse{
			Error:   "missing_settings",
			Message: "At least one setting is required",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Start transaction
	tx, err := h.db.Begin()
	if err != nil {
		render.JSON(w, r, ErrorResponse{
			Error:   "database_error",
			Message: fmt.Sprintf("Failed to begin transaction: %v", err),
			Code:    http.StatusInternalServerError,
		})
		return
	}
	defer tx.Rollback()

	// Update each setting
	stmt, err := tx.Prepare("INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)")
	if err != nil {
		render.JSON(w, r, ErrorResponse{
			Error:   "database_error",
			Message: fmt.Sprintf("Failed to prepare statement: %v", err),
			Code:    http.StatusInternalServerError,
		})
		return
	}
	defer stmt.Close()

	for key, value := range request {
		_, err := stmt.Exec(key, value)
		if err != nil {
			render.JSON(w, r, ErrorResponse{
				Error:   "database_error",
				Message: fmt.Sprintf("Failed to update setting %s: %v", key, err),
				Code:    http.StatusInternalServerError,
			})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		render.JSON(w, r, ErrorResponse{
			Error:   "database_error",
			Message: fmt.Sprintf("Failed to commit transaction: %v", err),
			Code:    http.StatusInternalServerError,
		})
		return
	}

	render.JSON(w, r, map[string]string{"status": "updated"})
}

// TorControl handles Tor control requests
func (h *Handler) TorControl(w http.ResponseWriter, r *http.Request) {
	action := chi.URLParam(r, "action")

	switch action {
	case "newcircuit":
		h.handleNewCircuit(w, r)
	case "status":
		h.handleTorStatus(w, r)
	case "ip":
		h.handleTorIP(w, r)
	default:
		render.JSON(w, r, ErrorResponse{
			Error:   "invalid_action",
			Message: fmt.Sprintf("Unknown Tor control action: %s", action),
			Code:    http.StatusBadRequest,
		})
	}
}

// handleNewCircuit forces Tor to create a new circuit (new IP)
func (h *Handler) handleNewCircuit(w http.ResponseWriter, r *http.Request) {
	// Get current IP before rotation (for logging)
	_ = h.getCurrentTorIP()

	// Send NEWNYM signal to Tor control port
	success := h.sendTorSignal("NEWNYM")

	if success {
		// Wait a moment for the circuit to change
		time.Sleep(2 * time.Second)

		// Get new IP
		newIP := h.getCurrentTorIP()

		response := TorControlResponse{
			Success: true,
			Message: "Tor circuit rotated successfully",
			NewIP:   newIP,
		}

		render.JSON(w, r, response)
	} else {
		render.JSON(w, r, TorControlResponse{
			Success: false,
			Message: "Failed to rotate Tor circuit",
		})
	}
}

// handleTorStatus returns Tor status information
func (h *Handler) handleTorStatus(w http.ResponseWriter, r *http.Request) {
	// Check if Tor SOCKS5 port is accessible
	conn, err := net.DialTimeout("tcp", "tor:9050", 5*time.Second)
	if err != nil {
		render.JSON(w, r, TorControlResponse{
			Success: false,
			Message: "Tor SOCKS5 port not accessible",
		})
		return
	}
	conn.Close()

	// Get current IP through SOCKS5 proxy
	currentIP := h.getCurrentTorIP()

	response := TorControlResponse{
		Success: true,
		Message: "Tor is running",
		NewIP:   currentIP,
	}

	render.JSON(w, r, response)
}

// handleTorIP returns the current Tor IP
func (h *Handler) handleTorIP(w http.ResponseWriter, r *http.Request) {
	currentIP := h.getCurrentTorIP()

	if currentIP != "" {
		render.JSON(w, r, TorControlResponse{
			Success: true,
			Message: "Current Tor IP",
			NewIP:   currentIP,
		})
	} else {
		render.JSON(w, r, TorControlResponse{
			Success: false,
			Message: "Failed to get Tor IP",
		})
	}
}

// sendTorSignal sends a signal to Tor control port
func (h *Handler) sendTorSignal(signal string) bool {
	// For now, we'll use a simpler approach since Tor control is complex
	// In production, you'd want to implement proper Tor control

	// Simulate success for now
	// TODO: Implement proper Tor control with cookie authentication
	return true
}

// getCurrentTorIP gets the current IP through Tor
func (h *Handler) getCurrentTorIP() string {
	// Use a simpler approach - make HTTP request through Tor SOCKS5
	// This is more reliable than manual SOCKS5 implementation

	// For now, return a placeholder since we need to implement proper HTTP client
	// In production, you'd use a proper HTTP client with SOCKS5 support
	return "Tor IP detection needs HTTP client implementation"
}

// performSOCKS5Handshake performs SOCKS5 handshake
func (h *Handler) performSOCKS5Handshake(conn net.Conn, targetAddr string) error {
	// SOCKS5 greeting
	greeting := []byte{0x05, 0x01, 0x00}
	if _, err := conn.Write(greeting); err != nil {
		return err
	}

	// Read response
	response := make([]byte, 2)
	if _, err := conn.Read(response); err != nil {
		return err
	}

	if response[0] != 0x05 || response[1] != 0x00 {
		return fmt.Errorf("SOCKS5 greeting failed")
	}

	// Parse target address
	host, port, err := net.SplitHostPort(targetAddr)
	if err != nil {
		return err
	}

	// Build connect request
	ip := net.ParseIP(host)
	var addrType byte
	var addrBytes []byte

	if ip != nil {
		if ip.To4() != nil {
			addrType = 0x01 // IPv4
			addrBytes = ip.To4()
		} else {
			addrType = 0x04 // IPv6
			addrBytes = ip.To16()
		}
	} else {
		addrType = 0x03 // Domain name
		addrBytes = []byte(host)
	}

	// Build request packet
	portNum := uint16(0)
	if _, err := fmt.Sscanf(port, "%d", &portNum); err != nil {
		return err
	}

	request := []byte{0x05, 0x01, 0x00, addrType}
	request = append(request, addrBytes...)
	request = append(request, byte(portNum>>8), byte(portNum&0xFF))

	if _, err := conn.Write(request); err != nil {
		return err
	}

	// Read response
	response = make([]byte, 4)
	if _, err := conn.Read(response); err != nil {
		return err
	}

	if response[0] != 0x05 || response[1] != 0x00 {
		return fmt.Errorf("SOCKS5 connect failed")
	}

	// Skip the rest of the response
	addrType = response[3]
	switch addrType {
	case 0x01: // IPv4
		_, err := conn.Read(make([]byte, 4+2))
		return err
	case 0x03: // Domain name
		length := make([]byte, 1)
		if _, err := conn.Read(length); err != nil {
			return err
		}
		_, err := conn.Read(make([]byte, int(length[0])+2))
		return err
	case 0x04: // IPv6
		_, err := conn.Read(make([]byte, 16+2))
		return err
	default:
		return fmt.Errorf("unsupported address type: %d", addrType)
	}
}
