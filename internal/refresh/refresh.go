package refresh

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"proxyrouter/internal/config"
)

// Refresher handles proxy refresh operations
type Refresher struct {
	db     *sql.DB
	config *config.RefreshConfig
	client *http.Client
}

// New creates a new refresher instance
func New(db *sql.DB, refreshConfig *config.RefreshConfig) *Refresher {
	return &Refresher{
		db:     db,
		config: refreshConfig,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// RefreshAll refreshes proxies from all configured sources
func (r *Refresher) RefreshAll(ctx context.Context) error {
	if !r.config.EnableGeneralSources {
		return nil
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(r.config.Sources))

	for _, source := range r.config.Sources {
		wg.Add(1)
		go func(s config.SourceConfig) {
			defer wg.Done()
			if err := r.refreshFromSource(ctx, s); err != nil {
				errChan <- fmt.Errorf("failed to refresh from %s: %w", s.Name, err)
			}
		}(source)
	}

	wg.Wait()
	close(errChan)

	// Collect any errors
	var errors []string
	for err := range errChan {
		errors = append(errors, err.Error())
	}

	if len(errors) > 0 {
		return fmt.Errorf("refresh errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// refreshFromSource refreshes proxies from a single source
func (r *Refresher) refreshFromSource(ctx context.Context, source config.SourceConfig) error {
	// Download content from source
	content, err := r.downloadSource(ctx, source.URL)
	if err != nil {
		return fmt.Errorf("failed to download from %s: %w", source.URL, err)
	}

	// Parse proxies based on source type
	var proxies []Proxy
	switch source.Type {
	case "html":
		proxies, err = r.parseHTMLSource(content, source.Name)
	case "raw":
		proxies, err = r.parseRawSource(content, source.Name)
	default:
		return fmt.Errorf("unknown source type: %s", source.Type)
	}

	if err != nil {
		return fmt.Errorf("failed to parse %s source: %w", source.Type, err)
	}

	// Import proxies to database
	if err := r.importProxies(ctx, proxies); err != nil {
		return fmt.Errorf("failed to import proxies: %w", err)
	}

	return nil
}

// downloadSource downloads content from a URL
func (r *Refresher) downloadSource(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	// Set user agent to avoid being blocked
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := r.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// parseHTMLSource parses HTML content for proxy information
func (r *Refresher) parseHTMLSource(content, sourceName string) ([]Proxy, error) {
	var proxies []Proxy

	// IP:Port regex pattern
	ipPortPattern := regexp.MustCompile(`(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}):(\d{2,5})`)
	matches := ipPortPattern.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) == 3 {
			ip := match[1]
			portStr := match[2]
			port, err := strconv.Atoi(portStr)
			if err != nil {
				continue
			}

			// Validate IP and port
			if !r.isValidIP(ip) || !r.isValidPort(port) {
				continue
			}

			proxy := Proxy{
				Scheme: "socks5", // Default to SOCKS5
				Host:   ip,
				Port:   port,
				Source: sourceName,
			}

			proxies = append(proxies, proxy)
		}
	}

	return proxies, nil
}

// parseRawSource parses raw text content for proxy information
func (r *Refresher) parseRawSource(content, sourceName string) ([]Proxy, error) {
	var proxies []Proxy

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Try to parse different formats
		proxy, err := r.parseProxyLine(line, sourceName)
		if err != nil {
			continue
		}

		proxies = append(proxies, proxy)
	}

	return proxies, nil
}

// ParseProxyLine parses a single proxy line (exported for API handlers)
func (r *Refresher) ParseProxyLine(line, sourceName string) (Proxy, error) {
	return r.parseProxyLine(line, sourceName)
}

// parseProxyLine parses a single proxy line
func (r *Refresher) parseProxyLine(line, sourceName string) (Proxy, error) {
	// Try different patterns
	patterns := []*regexp.Regexp{
		// socks5://ip:port
		regexp.MustCompile(`socks5://(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}):(\d{2,5})`),
		// http://ip:port
		regexp.MustCompile(`http://(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}):(\d{2,5})`),
		// https://ip:port
		regexp.MustCompile(`https://(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}):(\d{2,5})`),
		// ip:port (default to socks5)
		regexp.MustCompile(`^(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}):(\d{2,5})$`),
	}

	schemes := []string{"socks5", "http", "https", "socks5"}

	for i, pattern := range patterns {
		match := pattern.FindStringSubmatch(line)
		if len(match) == 3 {
			ip := match[1]
			portStr := match[2]
			port, err := strconv.Atoi(portStr)
			if err != nil {
				continue
			}

			if !r.isValidIP(ip) || !r.isValidPort(port) {
				continue
			}

			return Proxy{
				Scheme: schemes[i],
				Host:   ip,
				Port:   port,
				Source: sourceName,
			}, nil
		}
	}

	return Proxy{}, fmt.Errorf("unable to parse proxy line: %s", line)
}

// ImportProxies imports proxies into the database (exported for API handlers)
func (r *Refresher) ImportProxies(ctx context.Context, proxies []Proxy) error {
	return r.importProxies(ctx, proxies)
}

// importProxies imports proxies into the database
func (r *Refresher) importProxies(ctx context.Context, proxies []Proxy) error {
	if len(proxies) == 0 {
		return nil
	}

	// Start transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Prepare insert statement
	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR IGNORE INTO proxies (scheme, host, port, source, alive, created_at)
		VALUES (?, ?, ?, ?, 0, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Insert proxies
	for _, proxy := range proxies {
		_, err := stmt.ExecContext(ctx, proxy.Scheme, proxy.Host, proxy.Port, proxy.Source)
		if err != nil {
			return fmt.Errorf("failed to insert proxy %s:%d: %w", proxy.Host, proxy.Port, err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// HealthCheck performs health checks on proxies
func (r *Refresher) HealthCheck(ctx context.Context) error {
	// Get proxies that need health checking
	query := `
		SELECT id, scheme, host, port
		FROM proxies
		WHERE alive = 0
		   OR last_checked_at IS NULL
		   OR last_checked_at < datetime('now', '-1 hour')
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query, r.config.HealthcheckConcurrency)
	if err != nil {
		return fmt.Errorf("failed to query proxies for health check: %w", err)
	}
	defer rows.Close()

	var proxies []Proxy
	for rows.Next() {
		var proxy Proxy
		err := rows.Scan(&proxy.ID, &proxy.Scheme, &proxy.Host, &proxy.Port)
		if err != nil {
			continue
		}
		proxies = append(proxies, proxy)
	}

	if len(proxies) == 0 {
		return nil
	}

	// Perform health checks concurrently
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, r.config.HealthcheckConcurrency)
	results := make(chan HealthCheckResult, len(proxies))

	for _, proxy := range proxies {
		wg.Add(1)
		go func(p Proxy) {
			defer wg.Done()
			semaphore <- struct{}{} // Acquire semaphore
			defer func() { <-semaphore }() // Release semaphore

			result := r.checkProxyHealth(ctx, p)
			results <- result
		}(proxy)
	}

	wg.Wait()
	close(results)

	// Update database with results
	return r.updateHealthCheckResults(ctx, results)
}

// CheckProxyHealth checks the health of a single proxy (exported for API handlers)
func (r *Refresher) CheckProxyHealth(ctx context.Context, proxy Proxy) HealthCheckResult {
	return r.checkProxyHealth(ctx, proxy)
}

// checkProxyHealth checks the health of a single proxy
func (r *Refresher) checkProxyHealth(ctx context.Context, proxy Proxy) HealthCheckResult {
	result := HealthCheckResult{
		ProxyID: proxy.ID,
		Alive:   false,
	}

	// Create test URL
	testURL := "http://httpbin.org/ip"
	if proxy.Scheme == "socks5" {
		testURL = "https://httpbin.org/ip"
	}

	// Create HTTP client with proxy
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				proxyURL := fmt.Sprintf("%s://%s:%d", proxy.Scheme, proxy.Host, proxy.Port)
				return url.Parse(proxyURL)
			},
		},
	}

	// Make test request
	req, err := http.NewRequestWithContext(ctx, "GET", testURL, nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)

	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		result.Alive = true
		result.LatencyMs = int(duration.Milliseconds())
	} else {
		result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	return result
}

// updateHealthCheckResults updates the database with health check results
func (r *Refresher) updateHealthCheckResults(ctx context.Context, results <-chan HealthCheckResult) error {
	// Start transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Prepare update statement
	stmt, err := tx.PrepareContext(ctx, `
		UPDATE proxies
		SET alive = ?, latency_ms = ?, last_checked_at = CURRENT_TIMESTAMP, error_message = ?
		WHERE id = ?
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Update each result
	for result := range results {
		var latency *int
		if result.Alive {
			latency = &result.LatencyMs
		}

		var errorMsg *string
		if result.Error != "" {
			errorMsg = &result.Error
		}

		_, err := stmt.ExecContext(ctx, result.Alive, latency, errorMsg, result.ProxyID)
		if err != nil {
			return fmt.Errorf("failed to update proxy %d: %w", result.ProxyID, err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// isValidIP validates an IP address
func (r *Refresher) isValidIP(ip string) bool {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return false
	}

	for _, part := range parts {
		num, err := strconv.Atoi(part)
		if err != nil || num < 0 || num > 255 {
			return false
		}
	}

	return true
}

// isValidPort validates a port number
func (r *Refresher) isValidPort(port int) bool {
	return port > 0 && port <= 65535
}

// Proxy represents a proxy entry
type Proxy struct {
	ID     int    `json:"id,omitempty"`
	Scheme string `json:"scheme"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
	Source string `json:"source"`
}

// HealthCheckResult represents the result of a health check
type HealthCheckResult struct {
	ProxyID   int    `json:"proxy_id"`
	Alive     bool   `json:"alive"`
	LatencyMs int    `json:"latency_ms,omitempty"`
	Error     string `json:"error,omitempty"`
}
