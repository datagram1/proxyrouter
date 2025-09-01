package acl

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strings"
)

// ACL represents the access control list
type ACL struct {
	db *sql.DB
}

// New creates a new ACL instance
func New(db *sql.DB) *ACL {
	return &ACL{db: db}
}

// IsAllowed checks if the given IP address is allowed
func (a *ACL) IsAllowed(ctx context.Context, clientIP string) (bool, error) {
	// Parse the client IP
	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false, fmt.Errorf("invalid IP address: %s", clientIP)
	}

	// Get all allowed subnets
	subnets, err := a.GetAllowedSubnets(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get allowed subnets: %w", err)
	}

	// Check if IP is in any allowed subnet
	for _, cidr := range subnets {
		if a.ipInCIDR(ip, cidr) {
			return true, nil
		}
	}

	return false, nil
}

// GetAllowedSubnets returns all allowed CIDR subnets
func (a *ACL) GetAllowedSubnets(ctx context.Context) ([]string, error) {
	query := "SELECT cidr FROM acl_subnets ORDER BY cidr"
	rows, err := a.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query allowed subnets: %w", err)
	}
	defer rows.Close()

	var subnets []string
	for rows.Next() {
		var cidr string
		if err := rows.Scan(&cidr); err != nil {
			return nil, fmt.Errorf("failed to scan CIDR: %w", err)
		}
		subnets = append(subnets, cidr)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over subnets: %w", err)
	}

	return subnets, nil
}

// AddSubnet adds a new allowed subnet
func (a *ACL) AddSubnet(ctx context.Context, cidr string) error {
	// Validate CIDR format
	if err := a.validateCIDR(cidr); err != nil {
		return fmt.Errorf("invalid CIDR format: %w", err)
	}

	query := "INSERT OR IGNORE INTO acl_subnets (cidr) VALUES (?)"
	_, err := a.db.ExecContext(ctx, query, cidr)
	if err != nil {
		return fmt.Errorf("failed to add subnet: %w", err)
	}

	return nil
}

// RemoveSubnet removes an allowed subnet
func (a *ACL) RemoveSubnet(ctx context.Context, id int) error {
	query := "DELETE FROM acl_subnets WHERE id = ?"
	result, err := a.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to remove subnet: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("subnet with id %d not found", id)
	}

	return nil
}

// GetSubnets returns all subnets with their IDs
func (a *ACL) GetSubnets(ctx context.Context) ([]Subnet, error) {
	query := "SELECT id, cidr FROM acl_subnets ORDER BY cidr"
	rows, err := a.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query subnets: %w", err)
	}
	defer rows.Close()

	var subnets []Subnet
	for rows.Next() {
		var subnet Subnet
		if err := rows.Scan(&subnet.ID, &subnet.CIDR); err != nil {
			return nil, fmt.Errorf("failed to scan subnet: %w", err)
		}
		subnets = append(subnets, subnet)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over subnets: %w", err)
	}

	return subnets, nil
}

// Subnet represents an ACL subnet entry
type Subnet struct {
	ID   int    `json:"id"`
	CIDR string `json:"cidr"`
}

// validateCIDR validates that the CIDR format is correct
func (a *ACL) validateCIDR(cidr string) error {
	_, _, err := net.ParseCIDR(cidr)
	return err
}

// ipInCIDR checks if an IP is within a CIDR range
func (a *ACL) ipInCIDR(ip net.IP, cidr string) bool {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}
	return network.Contains(ip)
}

// ExtractClientIP extracts the client IP from various sources
func ExtractClientIP(remoteAddr string, headers map[string]string) string {
	// Check for X-Forwarded-For header
	if forwardedFor := headers["X-Forwarded-For"]; forwardedFor != "" {
		// Take the first IP in the chain
		ips := strings.Split(forwardedFor, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	// Check for X-Real-IP header
	if realIP := headers["X-Real-IP"]; realIP != "" {
		if net.ParseIP(realIP) != nil {
			return realIP
		}
	}

	// Use remote address
	if remoteAddr != "" {
		// Remove port if present
		if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
			return host
		}
		return remoteAddr
	}

	return ""
}
