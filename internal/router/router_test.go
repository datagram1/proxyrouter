package router

import (
	"testing"
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

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}
