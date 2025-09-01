package router

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"proxyrouter/internal/acl"
)

func TestHostMatchesGlob(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		glob     string
		expected bool
	}{
		{"exact match", "example.com", "example.com", true},
		{"wildcard suffix", "sub.example.com", "*.example.com", true},
		{"wildcard prefix", "example.org", "example.*", true},
		{"no match", "other.com", "*.example.com", false},
		{"universal wildcard", "any.host.com", "*", true},
		{"empty glob", "example.com", "", false},
	}

	router := &Router{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := router.hostMatchesGlob(tt.host, tt.glob)
			if result != tt.expected {
				t.Errorf("hostMatchesGlob(%q, %q) = %v, want %v", tt.host, tt.glob, result, tt.expected)
			}
		})
	}
}

func TestIPInCIDR(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		cidr     string
		expected bool
	}{
		{"in range", "192.168.10.5", "192.168.10.0/24", true},
		{"in range", "192.168.10.255", "192.168.10.0/24", true},
		{"not in range", "192.168.11.5", "192.168.10.0/24", false},
		{"invalid IP", "invalid", "192.168.10.0/24", false},
		{"invalid CIDR", "192.168.10.5", "invalid", false},
	}

	router := &Router{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := router.ipInCIDR(tt.ip, tt.cidr)
			if result != tt.expected {
				t.Errorf("ipInCIDR(%q, %q) = %v, want %v", tt.ip, tt.cidr, result, tt.expected)
			}
		})
	}
}

func TestMatchesRoute(t *testing.T) {
	tests := []struct {
		name       string
		route      *Route
		clientIP   string
		targetHost string
		expected   bool
	}{
		{
			name: "match client CIDR and host glob",
			route: &Route{
				ClientCIDR: stringPtr("192.168.10.0/24"),
				HostGlob:   stringPtr("*.example.com"),
			},
			clientIP:   "192.168.10.5",
			targetHost: "sub.example.com",
			expected:   true,
		},
		{
			name: "no client CIDR constraint",
			route: &Route{
				HostGlob: stringPtr("*.example.com"),
			},
			clientIP:   "192.168.11.5",
			targetHost: "sub.example.com",
			expected:   true,
		},
		{
			name: "no host glob constraint",
			route: &Route{
				ClientCIDR: stringPtr("192.168.10.0/24"),
			},
			clientIP:   "192.168.10.5",
			targetHost: "any.host.com",
			expected:   true,
		},
		{
			name: "client IP not in CIDR",
			route: &Route{
				ClientCIDR: stringPtr("192.168.10.0/24"),
				HostGlob:   stringPtr("*.example.com"),
			},
			clientIP:   "192.168.11.5",
			targetHost: "sub.example.com",
			expected:   false,
		},
		{
			name: "host doesn't match glob",
			route: &Route{
				ClientCIDR: stringPtr("192.168.10.0/24"),
				HostGlob:   stringPtr("*.example.com"),
			},
			clientIP:   "192.168.10.5",
			targetHost: "other.com",
			expected:   false,
		},
	}

	router := &Router{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := router.matchesRoute(tt.route, tt.clientIP, tt.targetHost)
			if result != tt.expected {
				t.Errorf("matchesRoute() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFindRoute(t *testing.T) {
	// Create in-memory database for testing
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE routes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			client_cidr TEXT,
			host_glob TEXT,
			"group" TEXT NOT NULL,
			proxy_id INTEGER,
			precedence INTEGER NOT NULL DEFAULT 0,
			enabled BOOLEAN NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	require.NoError(t, err)

	router := New(db)

	tests := []struct {
		name      string
		routes    []Route
		clientIP  string
		targetHost string
		expected  *Route
		expectErr bool
	}{
		{
			name: "exact match with highest precedence",
			routes: []Route{
				{Group: RouteGroupLocal, Precedence: 1, Enabled: true},
				{Group: RouteGroupGeneral, Precedence: 2, Enabled: true},
			},
			clientIP:   "192.168.1.100",
			targetHost: "example.com",
			expected:   &Route{Group: RouteGroupLocal, Precedence: 1, Enabled: true},
		},
		{
			name: "client CIDR match",
			routes: []Route{
				{Group: RouteGroupTor, Precedence: 1, ClientCIDR: stringPtr("192.168.1.0/24"), Enabled: true},
				{Group: RouteGroupGeneral, Precedence: 2, Enabled: true},
			},
			clientIP:   "192.168.1.100",
			targetHost: "example.com",
			expected:   &Route{Group: RouteGroupTor, Precedence: 1, ClientCIDR: stringPtr("192.168.1.0/24"), Enabled: true},
		},
		{
			name: "host glob match",
			routes: []Route{
				{Group: RouteGroupUpstream, Precedence: 1, HostGlob: stringPtr("*.example.com"), Enabled: true},
				{Group: RouteGroupGeneral, Precedence: 2, Enabled: true},
			},
			clientIP:   "192.168.1.100",
			targetHost: "api.example.com",
			expected:   &Route{Group: RouteGroupUpstream, Precedence: 1, HostGlob: stringPtr("*.example.com"), Enabled: true},
		},
		{
			name: "client CIDR and host glob match",
			routes: []Route{
				{Group: RouteGroupTor, Precedence: 1, ClientCIDR: stringPtr("192.168.1.0/24"), HostGlob: stringPtr("*.example.com"), Enabled: true},
				{Group: RouteGroupGeneral, Precedence: 2, Enabled: true},
			},
			clientIP:   "192.168.1.100",
			targetHost: "api.example.com",
			expected:   &Route{Group: RouteGroupTor, Precedence: 1, ClientCIDR: stringPtr("192.168.1.0/24"), HostGlob: stringPtr("*.example.com"), Enabled: true},
		},
		{
			name: "no match falls back to general",
			routes: []Route{
				{Group: RouteGroupTor, Precedence: 1, ClientCIDR: stringPtr("10.0.0.0/8"), Enabled: true},
				{Group: RouteGroupGeneral, Precedence: 2, Enabled: true},
			},
			clientIP:   "192.168.1.100",
			targetHost: "example.com",
			expected:   &Route{Group: RouteGroupGeneral, Precedence: 2, Enabled: true},
		},
		{
			name: "disabled route ignored",
			routes: []Route{
				{Group: RouteGroupLocal, Precedence: 1, Enabled: false},
				{Group: RouteGroupGeneral, Precedence: 2, Enabled: true},
			},
			clientIP:   "192.168.1.100",
			targetHost: "example.com",
			expected:   &Route{Group: RouteGroupGeneral, Precedence: 2, Enabled: true},
		},
		{
			name: "no routes returns nil",
			routes: []Route{},
			clientIP:   "192.168.1.100",
			targetHost: "example.com",
			expected:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear existing routes
			_, err := db.Exec("DELETE FROM routes")
			require.NoError(t, err)

			// Insert test routes
			for _, route := range tt.routes {
				_, err := db.Exec(`
					INSERT INTO routes (client_cidr, host_glob, "group", proxy_id, precedence, enabled)
					VALUES (?, ?, ?, ?, ?, ?)
				`, route.ClientCIDR, route.HostGlob, route.Group, route.ProxyID, route.Precedence, route.Enabled)
				require.NoError(t, err)
			}

			// Test FindRoute
			result, err := router.FindRoute(context.Background(), tt.clientIP, tt.targetHost)

			if tt.expectErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected.Group, result.Group)
				assert.Equal(t, tt.expected.Precedence, result.Precedence)
				if tt.expected.ClientCIDR != nil {
					assert.Equal(t, *tt.expected.ClientCIDR, *result.ClientCIDR)
				}
				if tt.expected.HostGlob != nil {
					assert.Equal(t, *tt.expected.HostGlob, *result.HostGlob)
				}
			}
		})
	}
}

func TestRoutePrecedence(t *testing.T) {
	// Create in-memory database for testing
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE routes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			client_cidr TEXT,
			host_glob TEXT,
			"group" TEXT NOT NULL,
			proxy_id INTEGER,
			precedence INTEGER NOT NULL DEFAULT 0,
			enabled BOOLEAN NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	require.NoError(t, err)

	router := New(db)

	// Insert routes with different precedence
	routes := []struct {
		group      RouteGroup
		precedence int
	}{
		{RouteGroupGeneral, 100},
		{RouteGroupLocal, 10},
		{RouteGroupTor, 50},
		{RouteGroupUpstream, 25},
	}

	for _, route := range routes {
		_, err := db.Exec(`
			INSERT INTO routes ("group", precedence, enabled)
			VALUES (?, ?, 1)
		`, route.group, route.precedence)
		require.NoError(t, err)
	}

	// Test that lowest precedence (highest priority) is selected
	result, err := router.FindRoute(context.Background(), "192.168.1.100", "example.com")
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, RouteGroupLocal, result.Group) // Should select precedence 10
	assert.Equal(t, 10, result.Precedence)
}

func TestACLIntegration(t *testing.T) {
	// Create in-memory database for testing
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create ACL table
	_, err = db.Exec(`
		CREATE TABLE acl_subnets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			cidr TEXT NOT NULL UNIQUE,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	require.NoError(t, err)

	// Create routes table
	_, err = db.Exec(`
		CREATE TABLE routes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			client_cidr TEXT,
			host_glob TEXT,
			"group" TEXT NOT NULL,
			proxy_id INTEGER,
			precedence INTEGER NOT NULL DEFAULT 0,
			enabled BOOLEAN NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	require.NoError(t, err)

	// Insert test ACL entries
	_, err = db.Exec("INSERT INTO acl_subnets (cidr) VALUES (?)", "192.168.1.0/24")
	require.NoError(t, err)

	// Insert test route
	_, err = db.Exec(`
		INSERT INTO routes ("group", precedence, enabled)
		VALUES (?, ?, ?)
	`, RouteGroupLocal, 1, true)
	require.NoError(t, err)

	acl := acl.New(db)
	router := New(db)

	tests := []struct {
		name      string
		clientIP  string
		expectAllowed bool
	}{
		{
			name: "allowed IP",
			clientIP: "192.168.1.100",
			expectAllowed: true,
		},
		{
			name: "denied IP",
			clientIP: "10.0.0.1",
			expectAllowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test ACL
			allowed, err := acl.IsAllowed(context.Background(), tt.clientIP)
			require.NoError(t, err)
			assert.Equal(t, tt.expectAllowed, allowed)

			// Test routing (should work regardless of ACL for this test)
			route, err := router.FindRoute(context.Background(), tt.clientIP, "example.com")
			require.NoError(t, err)
			assert.NotNil(t, route)
		})
	}
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}
