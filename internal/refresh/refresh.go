package refresh

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net"
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

			proxyURL := fmt.Sprintf("socks5://%s:%d", ip, port)
			proxy := Proxy{
				ProxyType: "socks5", // Default to SOCKS5
				IP:        ip,
				Port:      port,
				Source:    sourceName,
				ProxyURL:  &proxyURL,
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

			proxyURL := fmt.Sprintf("%s://%s:%d", schemes[i], ip, port)
			return Proxy{
				ProxyType: schemes[i],
				IP:        ip,
				Port:      port,
				Source:    sourceName,
				ProxyURL:  &proxyURL,
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
		INSERT OR IGNORE INTO proxies (proxy_type, ip, port, source, working, created_at, proxy_url)
		VALUES (?, ?, ?, ?, 0, CURRENT_TIMESTAMP, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Insert proxies
	for _, proxy := range proxies {
		_, err := stmt.ExecContext(ctx, proxy.ProxyType, proxy.IP, proxy.Port, proxy.Source, proxy.ProxyURL)
		if err != nil {
			return fmt.Errorf("failed to insert proxy %s:%d: %w", proxy.IP, proxy.Port, err)
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
		SELECT id, proxy_type, ip, port
		FROM proxies
		WHERE working = 0
		   OR tested_timestamp IS NULL
		   OR tested_timestamp < datetime('now', '-1 hour')
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
		err := rows.Scan(&proxy.ID, &proxy.ProxyType, &proxy.IP, &proxy.Port)
		if err != nil {
			continue
		}
		proxies = append(proxies, proxy)
	}

	if len(proxies) == 0 {
		fmt.Println("No proxies need health checking")
		return nil
	}

	fmt.Printf("Starting health check for %d proxies with %d concurrent workers...\n", len(proxies), r.config.HealthcheckConcurrency)

	// Perform health checks concurrently
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, r.config.HealthcheckConcurrency)
	results := make(chan HealthCheckResult, len(proxies))

	// Start a goroutine to print progress like tqdm
	workingCount := 0
	totalChecked := 0
	progressTicker := time.NewTicker(2 * time.Second)
	defer progressTicker.Stop()

	go func() {
		for range progressTicker.C {
			percentage := float64(totalChecked) / float64(len(proxies)) * 100
			barLength := 30
			filledLength := int(float64(barLength) * percentage / 100)
			bar := strings.Repeat("█", filledLength) + strings.Repeat("░", barLength-filledLength)
			fmt.Printf("\r[%s] %d/%d (%d working) %.1f%%", bar, totalChecked, len(proxies), workingCount, percentage)
		}
	}()

	for _, proxy := range proxies {
		wg.Add(1)
		go func(p Proxy) {
			defer wg.Done()
			semaphore <- struct{}{} // Acquire semaphore
			defer func() {
				<-semaphore // Release semaphore
				totalChecked++
			}()

			result := r.checkProxyHealth(ctx, p)
			if result.Working {
				workingCount++
			}
			results <- result
		}(proxy)
	}

	wg.Wait()
	close(results)

	fmt.Printf("\nHealth check completed: %d/%d proxies checked, %d working found\n", totalChecked, len(proxies), workingCount)

	// Update database with results
	return r.updateHealthCheckResults(ctx, results)
}

// CheckProxyHealth checks the health of a single proxy (exported for API handlers)
func (r *Refresher) CheckProxyHealth(ctx context.Context, proxy Proxy) HealthCheckResult {
	return r.checkProxyHealth(ctx, proxy)
}

// checkProxyHealth checks the health of a single proxy with intelligent type detection
func (r *Refresher) checkProxyHealth(ctx context.Context, proxy Proxy) HealthCheckResult {
	// Try the current scheme first
	testResult := r.testProxyWithProtocol(ctx, proxy, proxy.ProxyType)
	if testResult.Working {
		// Current scheme works, no need to test other protocols
		return testResult
	}

	// Current scheme failed, try the alternative protocol
	var alternativeProtocol string
	if proxy.ProxyType == "socks5" {
		alternativeProtocol = "http"
	} else {
		alternativeProtocol = "socks5"
	}

	alternativeResult := r.testProxyWithProtocol(ctx, proxy, alternativeProtocol)
	if alternativeResult.Working {
		// Alternative protocol works, update the proxy type in database
		if err := r.updateProxyType(ctx, proxy.ID, alternativeProtocol); err != nil {
			slog.Error("Failed to update proxy type", "proxy_id", proxy.ID, "proxy_type", alternativeProtocol, "error", err)
		}
		return alternativeResult
	}

	// Neither protocol works, return the current scheme result (which has the original error)
	return testResult
}

// testProxyWithProtocol tests a proxy with a specific protocol
func (r *Refresher) testProxyWithProtocol(ctx context.Context, proxy Proxy, protocol string) HealthCheckResult {
	result := HealthCheckResult{
		ProxyID: proxy.ID,
		Working: false,
	}

	// Use our test server to avoid getting banned by external providers
	testURL := "http://ip.knws.co.uk"

	// Create HTTP client with proxy
	client := &http.Client{
		Timeout: 10 * time.Second, // 10 second timeout for faster health checks
		Transport: &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				proxyURL := fmt.Sprintf("%s://%s:%d", protocol, proxy.IP, proxy.Port)
				return url.Parse(proxyURL)
			},
		},
	}

	proxyAddr := fmt.Sprintf("%s:%d", proxy.IP, proxy.Port)
	fmt.Printf("Testing %s -> %s...\n", proxyAddr, testURL)

	// Make test request with browser-like headers
	req, err := http.NewRequestWithContext(ctx, "GET", testURL, nil)
	if err != nil {
		fmt.Printf("  Failed to create request: %v\n", err)
		result.Error = err.Error()
		return result
	}

	// Add comprehensive browser-like headers to avoid detection
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Cache-Control", "max-age=0")
	req.Header.Set("Sec-Ch-Ua", "\"Not_A Brand\";v=\"8\", \"Chromium\";v=\"120\", \"Google Chrome\";v=\"120\"")
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", "\"Windows\"")

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)

	if err != nil {
		// Format error message similar to Python's urllib3 output
		errorMsg := err.Error()
		if strings.Contains(errorMsg, "timeout") || strings.Contains(errorMsg, "deadline") {
			errorMsg = fmt.Sprintf("Connection to %s timed out. (connect timeout=10)", testURL)
		} else if strings.Contains(errorMsg, "unexpected protocol version 72") || strings.Contains(errorMsg, "protocol version 72") {
			errorMsg = "SOCKS5 proxy server sent invalid data"
		} else if strings.Contains(errorMsg, "connection refused") {
			errorMsg = "Failed to establish a new connection: Connection refused"
		} else if strings.Contains(errorMsg, "connection closed") {
			errorMsg = "Failed to establish a new connection: Connection closed unexpectedly"
		}

		fmt.Printf("❌ %s -> %s: %s (%.2fs)\n", proxyAddr, testURL, errorMsg, duration.Seconds())
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()

	// Use more lenient success criteria: any response from 200-499 indicates proxy is working
	if resp.StatusCode >= 200 && resp.StatusCode < 500 {
		// Try to read response body to get IP if possible
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("  ⚠️ Success but failed to read response body: %v\n", err)
		} else {
			// Parse plain text IP response from our test server
			ipAddress := strings.TrimSpace(string(body))
			if ipAddress != "" && net.ParseIP(ipAddress) != nil {
				fmt.Printf("✅ %s -> %s (%.3fs)\n", proxyAddr, ipAddress, duration.Seconds())
			} else {
				fmt.Printf("✅ %s -> HTTP %d (%.3fs)\n", proxyAddr, resp.StatusCode, duration.Seconds())
			}
		}
		result.Working = true
		result.Latency = int(duration.Milliseconds())
		return result
	} else {
		fmt.Printf("❌ %s -> HTTP %d (%.3fs)\n", proxyAddr, resp.StatusCode, duration.Seconds())
		result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return result
	}
}

// updateProxyType updates the proxy_type/protocol of a proxy in the database
func (r *Refresher) updateProxyType(ctx context.Context, proxyID int, proxyType string) error {
	query := `UPDATE proxies SET proxy_type = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, proxyType, proxyID)
	return err
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
		SET working = ?, latency = ?, tested_timestamp = CURRENT_TIMESTAMP, error_message = ?
		WHERE id = ?
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Update each result
	for result := range results {
		var latency *int
		if result.Working {
			latency = &result.Latency
		}

		var errorMsg *string
		if result.Error != "" {
			errorMsg = &result.Error
		}

		_, err := stmt.ExecContext(ctx, result.Working, latency, errorMsg, result.ProxyID)
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
	ID        int     `json:"id,omitempty"`
	ProxyType string  `json:"proxy_type"`
	IP        string  `json:"ip"`
	Port      int     `json:"port"`
	Source    string  `json:"source"`
	ProxyURL  *string `json:"proxy_url,omitempty"`
}

// HealthCheckResult represents the result of a health check
type HealthCheckResult struct {
	ProxyID int    `json:"proxy_id"`
	Working bool   `json:"working"`
	Latency int    `json:"latency,omitempty"`
	Error   string `json:"error,omitempty"`
}
