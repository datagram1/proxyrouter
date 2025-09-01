package acl

import (
	"net"
	"testing"
)

func TestExtractClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		expected   string
	}{
		{
			name:       "X-Forwarded-For header",
			remoteAddr: "192.168.1.1:1234",
			headers: map[string]string{
				"X-Forwarded-For": "10.0.0.1, 192.168.1.2",
			},
			expected: "10.0.0.1",
		},
		{
			name:       "X-Real-IP header",
			remoteAddr: "192.168.1.1:1234",
			headers: map[string]string{
				"X-Real-IP": "10.0.0.2",
			},
			expected: "10.0.0.2",
		},
		{
			name:       "remote address with port",
			remoteAddr: "192.168.1.1:1234",
			headers:    map[string]string{},
			expected:   "192.168.1.1",
		},
		{
			name:       "remote address without port",
			remoteAddr: "192.168.1.1",
			headers:    map[string]string{},
			expected:   "192.168.1.1",
		},
		{
			name:       "empty remote address",
			remoteAddr: "",
			headers:    map[string]string{},
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractClientIP(tt.remoteAddr, tt.headers)
			if result != tt.expected {
				t.Errorf("ExtractClientIP() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestValidateCIDR(t *testing.T) {
	tests := []struct {
		name    string
		cidr    string
		isValid bool
	}{
		{"valid CIDR", "192.168.10.0/24", true},
		{"valid CIDR", "10.0.0.0/8", true},
		{"valid CIDR", "172.16.0.0/12", true},
		{"invalid CIDR", "192.168.10.0", false},
		{"invalid CIDR", "192.168.10.0/33", false},
		{"invalid CIDR", "invalid", false},
		{"empty CIDR", "", false},
	}

	acl := &ACL{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := acl.validateCIDR(tt.cidr)
			if tt.isValid && err != nil {
				t.Errorf("validateCIDR(%q) returned error: %v", tt.cidr, err)
			}
			if !tt.isValid && err == nil {
				t.Errorf("validateCIDR(%q) should have returned error", tt.cidr)
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

	acl := &ACL{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			result := acl.ipInCIDR(ip, tt.cidr)
			if result != tt.expected {
				t.Errorf("ipInCIDR(%q, %q) = %v, want %v", tt.ip, tt.cidr, result, tt.expected)
			}
		})
	}
}
