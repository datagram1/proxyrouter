package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	Listen   ListenConfig   `mapstructure:"listen"`
	Timeouts TimeoutConfig  `mapstructure:"timeouts"`
	Tor      TorConfig      `mapstructure:"tor"`
	Refresh  RefreshConfig  `mapstructure:"refresh"`
	Database DatabaseConfig `mapstructure:"database"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Metrics  MetricsConfig  `mapstructure:"metrics"`
	Admin    AdminConfig    `mapstructure:"admin"`
	Security SecurityConfig `mapstructure:"security"`
}

// ListenConfig holds listening addresses
type ListenConfig struct {
	HTTPProxy  string `mapstructure:"http_proxy"`
	Socks5Proxy string `mapstructure:"socks5_proxy"`
	API        string `mapstructure:"api"`
}

// TimeoutConfig holds timeout settings
type TimeoutConfig struct {
	DialMs  int `mapstructure:"dial_ms"`
	ReadMs  int `mapstructure:"read_ms"`
	WriteMs int `mapstructure:"write_ms"`
}

// TorConfig holds Tor-related settings
type TorConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	SocksAddress string `mapstructure:"socks_address"`
}

// RefreshConfig holds proxy refresh settings
type RefreshConfig struct {
	EnableGeneralSources bool           `mapstructure:"enable_general_sources"`
	IntervalSec          int            `mapstructure:"interval_sec"`
	HealthcheckConcurrency int          `mapstructure:"healthcheck_concurrency"`
	Sources              []SourceConfig `mapstructure:"sources"`
}

// SourceConfig holds proxy source configuration
type SourceConfig struct {
	Name string `mapstructure:"name"`
	URL  string `mapstructure:"url"`
	Type string `mapstructure:"type"` // "html" or "raw"
}

// DatabaseConfig holds database settings
type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// MetricsConfig holds metrics settings
type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
}

// AdminConfig holds admin UI settings
type AdminConfig struct {
	Enabled      bool     `mapstructure:"enabled"`
	Bind         string   `mapstructure:"bind"`
	Port         int      `mapstructure:"port"`
	BasePath     string   `mapstructure:"base_path"`
	SessionSecret string  `mapstructure:"session_secret"`
	AllowCIDRs   []string `mapstructure:"allow_cidrs"`
	TLS          TLSConfig `mapstructure:"tls"`
}

// TLSConfig holds TLS settings
type TLSConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
}

// SecurityConfig holds security settings
type SecurityConfig struct {
	PasswordHash string         `mapstructure:"password_hash"`
	Login        LoginConfig    `mapstructure:"login"`
}

// LoginConfig holds login security settings
type LoginConfig struct {
	MaxAttempts    int `mapstructure:"max_attempts"`
	WindowSeconds  int `mapstructure:"window_seconds"`
}

// Load loads configuration from file and environment variables
func Load(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")
	
	// Set defaults
	setDefaults()
	
	// Read environment variables
	viper.AutomaticEnv()
	viper.SetEnvPrefix("PROXYROUTER")
	
	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	
	// Validate config
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	
	return &config, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	viper.SetDefault("listen.http_proxy", "0.0.0.0:8080")
	viper.SetDefault("listen.socks5_proxy", "0.0.0.0:1080")
	viper.SetDefault("listen.api", "0.0.0.0:8081")
	viper.SetDefault("timeouts.dial_ms", 8000)
	viper.SetDefault("timeouts.read_ms", 60000)
	viper.SetDefault("timeouts.write_ms", 60000)
	viper.SetDefault("tor.enabled", true)
	viper.SetDefault("tor.socks_address", "127.0.0.1:9050")
	viper.SetDefault("refresh.enable_general_sources", true)
	viper.SetDefault("refresh.interval_sec", 900)
	viper.SetDefault("refresh.healthcheck_concurrency", 50)
	viper.SetDefault("database.path", "/var/lib/proxyr/router.db")
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("metrics.enabled", true)
	viper.SetDefault("metrics.path", "/metrics")
	
	// Admin UI defaults
	viper.SetDefault("admin.enabled", true)
	viper.SetDefault("admin.bind", "127.0.0.1")
	viper.SetDefault("admin.port", 5000)
	viper.SetDefault("admin.base_path", "/admin")
	viper.SetDefault("admin.session_secret", "")
	viper.SetDefault("admin.allow_cidrs", []string{"127.0.0.1/32"})
	viper.SetDefault("admin.tls.enabled", false)
	viper.SetDefault("admin.tls.cert_file", "")
	viper.SetDefault("admin.tls.key_file", "")
	
	// Security defaults
	viper.SetDefault("security.password_hash", "argon2id")
	viper.SetDefault("security.login.max_attempts", 10)
	viper.SetDefault("security.login.window_seconds", 900)
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	var errors []string

	// Check required listen addresses
	if config.Listen.HTTPProxy == "" {
		errors = append(errors, "http_proxy listen address is required")
	}
	if config.Listen.Socks5Proxy == "" {
		errors = append(errors, "socks5_proxy listen address is required")
	}
	if config.Listen.API == "" {
		errors = append(errors, "api listen address is required")
	}

	// Check for port clashes by parsing listen addresses
	ports := make(map[string]string)
	if config.Listen.HTTPProxy != "" {
		if port := extractPort(config.Listen.HTTPProxy); port != "" {
			ports[port] = "HTTP Proxy"
		}
	}
	if config.Listen.Socks5Proxy != "" {
		if port := extractPort(config.Listen.Socks5Proxy); port != "" {
			ports[port] = "SOCKS5 Proxy"
		}
	}
	if config.Listen.API != "" {
		if port := extractPort(config.Listen.API); port != "" {
			ports[port] = "API"
		}
	}

	// Check for duplicate ports
	portCount := make(map[string]int)
	for port := range ports {
		portCount[port]++
	}
	for port, count := range portCount {
		if count > 1 {
			errors = append(errors, fmt.Sprintf("Port %s is used by multiple services", port))
		}
	}

	// Check database configuration
	if config.Database.Path == "" {
		errors = append(errors, "database path is required")
	}

	// Check timeout configurations
	if config.Timeouts.DialMs <= 0 {
		errors = append(errors, "dial timeout must be positive")
	}
	if config.Timeouts.ReadMs <= 0 {
		errors = append(errors, "read timeout must be positive")
	}
	if config.Timeouts.WriteMs <= 0 {
		errors = append(errors, "write timeout must be positive")
	}

	// Check Tor configuration
	if config.Tor.Enabled {
		if config.Tor.SocksAddress == "" {
			errors = append(errors, "Tor socks_address is required when Tor is enabled")
		} else {
			if port := extractPort(config.Tor.SocksAddress); port != "" {
				if portInt := parsePort(port); portInt < 1 || portInt > 65535 {
					errors = append(errors, fmt.Sprintf("Tor port %s is invalid (must be 1-65535)", port))
				}
			}
		}
	}

	// Check refresh configuration
	if config.Refresh.IntervalSec < 1 {
		errors = append(errors, "refresh interval must be at least 1 second")
	}
	if config.Refresh.HealthcheckConcurrency < 1 {
		errors = append(errors, "healthcheck concurrency must be at least 1")
	}

	// Check logging configuration
	if config.Logging.Level != "" {
		validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
		if !validLevels[config.Logging.Level] {
			errors = append(errors, fmt.Sprintf("invalid logging level: %s (must be debug, info, warn, or error)", config.Logging.Level))
		}
	}

	if config.Logging.Format != "" {
		validFormats := map[string]bool{"json": true, "text": true}
		if !validFormats[config.Logging.Format] {
			errors = append(errors, fmt.Sprintf("invalid logging format: %s (must be json or text)", config.Logging.Format))
		}
	}

	// Check admin configuration
	if config.Admin.Enabled {
		if config.Admin.Port < 1 || config.Admin.Port > 65535 {
			errors = append(errors, fmt.Sprintf("admin port %d is invalid (must be 1-65535)", config.Admin.Port))
		}
		if config.Admin.Bind == "" {
			errors = append(errors, "admin bind address is required when admin is enabled")
		}
		if config.Admin.TLS.Enabled {
			if config.Admin.TLS.CertFile == "" {
				errors = append(errors, "admin TLS cert file is required when TLS is enabled")
			}
			if config.Admin.TLS.KeyFile == "" {
				errors = append(errors, "admin TLS key file is required when TLS is enabled")
			}
		}
	}

	// Check security configuration
	if config.Security.PasswordHash != "" {
		validHashes := map[string]bool{"argon2id": true, "bcrypt": true}
		if !validHashes[config.Security.PasswordHash] {
			errors = append(errors, fmt.Sprintf("invalid password hash algorithm: %s (must be argon2id or bcrypt)", config.Security.PasswordHash))
		}
	}
	if config.Security.Login.MaxAttempts < 1 {
		errors = append(errors, "login max attempts must be at least 1")
	}
	if config.Security.Login.WindowSeconds < 1 {
		errors = append(errors, "login window seconds must be at least 1")
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation failed:\n%s", strings.Join(errors, "\n"))
	}

	return nil
}

// extractPort extracts port from address string (e.g., "127.0.0.1:8080" -> "8080")
func extractPort(addr string) string {
	if idx := strings.LastIndex(addr, ":"); idx != -1 && idx < len(addr)-1 {
		return addr[idx+1:]
	}
	return ""
}

// parsePort converts port string to integer, returns 0 if invalid
func parsePort(portStr string) int {
	var port int
	_, err := fmt.Sscanf(portStr, "%d", &port)
	if err != nil {
		return 0
	}
	return port
}

// GetDialTimeout returns the dial timeout as time.Duration
func (c *Config) GetDialTimeout() time.Duration {
	return time.Duration(c.Timeouts.DialMs) * time.Millisecond
}

// GetReadTimeout returns the read timeout as time.Duration
func (c *Config) GetReadTimeout() time.Duration {
	return time.Duration(c.Timeouts.ReadMs) * time.Millisecond
}

// GetWriteTimeout returns the write timeout as time.Duration
func (c *Config) GetWriteTimeout() time.Duration {
	return time.Duration(c.Timeouts.WriteMs) * time.Millisecond
}

// GetRefreshInterval returns the refresh interval as time.Duration
func (c *Config) GetRefreshInterval() time.Duration {
	return time.Duration(c.Refresh.IntervalSec) * time.Second
}

// GetHealthCheckInterval returns the health check interval as time.Duration
func (c *Config) GetHealthCheckInterval() time.Duration {
	// For now, use the same interval as refresh, but this could be configurable
	return time.Duration(c.Refresh.IntervalSec) * time.Second
}
