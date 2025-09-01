package config

import (
	"fmt"
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
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	if config.Listen.HTTPProxy == "" {
		return fmt.Errorf("http_proxy listen address is required")
	}
	if config.Listen.Socks5Proxy == "" {
		return fmt.Errorf("socks5_proxy listen address is required")
	}
	if config.Listen.API == "" {
		return fmt.Errorf("api listen address is required")
	}
	if config.Database.Path == "" {
		return fmt.Errorf("database path is required")
	}
	if config.Timeouts.DialMs <= 0 {
		return fmt.Errorf("dial timeout must be positive")
	}
	if config.Timeouts.ReadMs <= 0 {
		return fmt.Errorf("read timeout must be positive")
	}
	if config.Timeouts.WriteMs <= 0 {
		return fmt.Errorf("write timeout must be positive")
	}
	return nil
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
