package router

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strings"
	"time"
)

// RouteGroup represents the routing group
type RouteGroup string

const (
	RouteGroupLocal     RouteGroup = "LOCAL"
	RouteGroupGeneral   RouteGroup = "GENERAL"
	RouteGroupTor       RouteGroup = "TOR"
	RouteGroupUpstream  RouteGroup = "UPSTREAM"
)

// Route represents a routing rule
type Route struct {
	ID          int         `json:"id"`
	ClientCIDR  *string     `json:"client_cidr,omitempty"`
	HostGlob    *string     `json:"host_glob,omitempty"`
	Group       RouteGroup  `json:"group"`
	ProxyID     *int        `json:"proxy_id,omitempty"`
	Precedence  int         `json:"precedence"`
	Enabled     bool        `json:"enabled"`
	CreatedAt   time.Time   `json:"created_at"`
}

// Router represents the routing engine
type Router struct {
	db *sql.DB
}

// New creates a new router instance
func New(db *sql.DB) *Router {
	return &Router{db: db}
}

// FindRoute finds the best matching route for a request
func (r *Router) FindRoute(ctx context.Context, clientIP, targetHost string) (*Route, error) {
	query := `
		SELECT id, client_cidr, host_glob, "group", proxy_id, precedence, enabled, created_at
		FROM routes
		WHERE enabled = 1
		ORDER BY precedence ASC, id ASC
	`
	
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query routes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var route Route
		var clientCIDR, hostGlob sql.NullString
		var proxyID sql.NullInt64
		
		err := rows.Scan(
			&route.ID,
			&clientCIDR,
			&hostGlob,
			&route.Group,
			&proxyID,
			&route.Precedence,
			&route.Enabled,
			&route.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan route: %w", err)
		}

		// Set nullable fields
		if clientCIDR.Valid {
			route.ClientCIDR = &clientCIDR.String
		}
		if hostGlob.Valid {
			route.HostGlob = &hostGlob.String
		}
		if proxyID.Valid {
			id := int(proxyID.Int64)
			route.ProxyID = &id
		}

		// Check if route matches
		if r.matchesRoute(&route, clientIP, targetHost) {
			return &route, nil
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over routes: %w", err)
	}

	// No matching route found
	return nil, nil
}

// matchesRoute checks if a route matches the given client IP and target host
func (r *Router) matchesRoute(route *Route, clientIP, targetHost string) bool {
	// Check client CIDR if specified
	if route.ClientCIDR != nil {
		if !r.ipInCIDR(clientIP, *route.ClientCIDR) {
			return false
		}
	}

	// Check host glob if specified
	if route.HostGlob != nil {
		if !r.hostMatchesGlob(targetHost, *route.HostGlob) {
			return false
		}
	}

	return true
}

// ipInCIDR checks if an IP is within a CIDR range
func (r *Router) ipInCIDR(ipStr, cidr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}

	return network.Contains(ip)
}

// hostMatchesGlob checks if a host matches a glob pattern
func (r *Router) hostMatchesGlob(host, glob string) bool {
	// Simple glob matching - can be enhanced for more complex patterns
	if glob == "*" {
		return true
	}

	if strings.HasPrefix(glob, "*.") {
		// Pattern like "*.example.com"
		suffix := glob[1:] // Remove the "*"
		return strings.HasSuffix(host, suffix)
	}

	if strings.HasSuffix(glob, ".*") {
		// Pattern like "example.*"
		prefix := glob[:len(glob)-1] // Remove the ".*"
		return strings.HasPrefix(host, prefix)
	}

	// Exact match
	return host == glob
}

// GetRoutes returns all routes (without context for backward compatibility)
func (r *Router) GetRoutes() ([]Route, error) {
	return r.GetRoutesWithContext(context.Background())
}

// GetRoutesWithContext returns all routes with context
func (r *Router) GetRoutesWithContext(ctx context.Context) ([]Route, error) {
	query := `
		SELECT id, client_cidr, host_glob, "group", proxy_id, precedence, enabled, created_at
		FROM routes
		ORDER BY precedence ASC, id ASC
	`
	
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query routes: %w", err)
	}
	defer rows.Close()

	var routes []Route
	for rows.Next() {
		var route Route
		var clientCIDR, hostGlob sql.NullString
		var proxyID sql.NullInt64
		
		err := rows.Scan(
			&route.ID,
			&clientCIDR,
			&hostGlob,
			&route.Group,
			&proxyID,
			&route.Precedence,
			&route.Enabled,
			&route.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan route: %w", err)
		}

		// Set nullable fields
		if clientCIDR.Valid {
			route.ClientCIDR = &clientCIDR.String
		}
		if hostGlob.Valid {
			route.HostGlob = &hostGlob.String
		}
		if proxyID.Valid {
			id := int(proxyID.Int64)
			route.ProxyID = &id
		}

		routes = append(routes, route)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over routes: %w", err)
	}

	return routes, nil
}

// CreateRouteWithContext creates a new route with context
func (r *Router) CreateRouteWithContext(ctx context.Context, route *Route) error {
	query := `
		INSERT INTO routes (client_cidr, host_glob, "group", proxy_id, precedence, enabled)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	
	_, err := r.db.ExecContext(ctx, query,
		route.ClientCIDR,
		route.HostGlob,
		route.Group,
		route.ProxyID,
		route.Precedence,
		route.Enabled,
	)
	
	if err != nil {
		return fmt.Errorf("failed to create route: %w", err)
	}

	return nil
}

// UpdateRoute updates an existing route
func (r *Router) UpdateRoute(ctx context.Context, id int, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	// Build dynamic query
	var setClauses []string
	var args []interface{}
	
	for field, value := range updates {
		setClauses = append(setClauses, field+" = ?")
		args = append(args, value)
	}
	
	args = append(args, id)
	
	query := fmt.Sprintf("UPDATE routes SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	
	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update route: %w", err)
	}

	return nil
}

// DeleteRouteWithContext deletes a route with context
func (r *Router) DeleteRouteWithContext(ctx context.Context, id int) error {
	query := "DELETE FROM routes WHERE id = ?"
	
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete route: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("route with id %d not found", id)
	}

	return nil
}

// CreateRoute creates a new route (without context for backward compatibility)
func (r *Router) CreateRoute(route *Route) error {
	return r.CreateRouteWithContext(context.Background(), route)
}



// DeleteRoute deletes a route (without context for backward compatibility)
func (r *Router) DeleteRoute(id int) error {
	return r.DeleteRouteWithContext(context.Background(), id)
}
