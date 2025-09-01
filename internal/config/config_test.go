package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	// Create a temporary config file
	configContent := `
listen:
  http_proxy: "0.0.0.0:8080"
  socks5_proxy: "0.0.0.0:1080"
  api: "0.0.0.0:8081"

timeouts:
  dial_ms: 5000
  read_ms: 30000
  write_ms: 30000

tor:
  enabled: true
  socks_address: "127.0.0.1:9050"

refresh:
  enable_general_sources: true
  interval_sec: 600
  healthcheck_concurrency: 25
  sources:
    - name: "test-source"
      url: "https://example.com/proxies.txt"
      type: "raw"

database:
  path: "/tmp/test.db"

logging:
  level: "debug"
  format: "json"

metrics:
  enabled: true
  path: "/metrics"
`

	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(configContent)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Test loading the config
	cfg, err := Load(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test all fields
	if cfg.Listen.HTTPProxy != "0.0.0.0:8080" {
		t.Errorf("Expected HTTPProxy to be '0.0.0.0:8080', got '%s'", cfg.Listen.HTTPProxy)
	}

	if cfg.Listen.Socks5Proxy != "0.0.0.0:1080" {
		t.Errorf("Expected Socks5Proxy to be '0.0.0.0:1080', got '%s'", cfg.Listen.Socks5Proxy)
	}

	if cfg.Listen.API != "0.0.0.0:8081" {
		t.Errorf("Expected API to be '0.0.0.0:8081', got '%s'", cfg.Listen.API)
	}

	if cfg.Timeouts.DialMs != 5000 {
		t.Errorf("Expected DialMs to be 5000, got %d", cfg.Timeouts.DialMs)
	}

	if cfg.Timeouts.ReadMs != 30000 {
		t.Errorf("Expected ReadMs to be 30000, got %d", cfg.Timeouts.ReadMs)
	}

	if cfg.Timeouts.WriteMs != 30000 {
		t.Errorf("Expected WriteMs to be 30000, got %d", cfg.Timeouts.WriteMs)
	}

	if !cfg.Tor.Enabled {
		t.Error("Expected Tor to be enabled")
	}

	if cfg.Tor.SocksAddress != "127.0.0.1:9050" {
		t.Errorf("Expected Tor socks address to be '127.0.0.1:9050', got '%s'", cfg.Tor.SocksAddress)
	}

	if !cfg.Refresh.EnableGeneralSources {
		t.Error("Expected general sources to be enabled")
	}

	if cfg.Refresh.IntervalSec != 600 {
		t.Errorf("Expected refresh interval to be 600, got %d", cfg.Refresh.IntervalSec)
	}

	if cfg.Refresh.HealthcheckConcurrency != 25 {
		t.Errorf("Expected healthcheck concurrency to be 25, got %d", cfg.Refresh.HealthcheckConcurrency)
	}

	if len(cfg.Refresh.Sources) != 1 {
		t.Errorf("Expected 1 source, got %d", len(cfg.Refresh.Sources))
	}

	if cfg.Refresh.Sources[0].Name != "test-source" {
		t.Errorf("Expected source name to be 'test-source', got '%s'", cfg.Refresh.Sources[0].Name)
	}

	if cfg.Database.Path != "/tmp/test.db" {
		t.Errorf("Expected database path to be '/tmp/test.db', got '%s'", cfg.Database.Path)
	}

	if cfg.Logging.Level != "debug" {
		t.Errorf("Expected logging level to be 'debug', got '%s'", cfg.Logging.Level)
	}

	if cfg.Logging.Format != "json" {
		t.Errorf("Expected logging format to be 'json', got '%s'", cfg.Logging.Format)
	}

	if !cfg.Metrics.Enabled {
		t.Error("Expected metrics to be enabled")
	}

	if cfg.Metrics.Path != "/metrics" {
		t.Errorf("Expected metrics path to be '/metrics', got '%s'", cfg.Metrics.Path)
	}
}

func TestLoadWithDefaults(t *testing.T) {
	// Create a minimal config file
	configContent := `
listen:
  http_proxy: "0.0.0.0:8080"
`

	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(configContent)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Test loading the config with defaults
	cfg, err := Load(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test that defaults are applied
	if cfg.Listen.Socks5Proxy != "0.0.0.0:1080" {
		t.Errorf("Expected default Socks5Proxy to be '0.0.0.0:1080', got '%s'", cfg.Listen.Socks5Proxy)
	}

	if cfg.Listen.API != "0.0.0.0:8081" {
		t.Errorf("Expected default API to be '0.0.0.0:8081', got '%s'", cfg.Listen.API)
	}

	if cfg.Timeouts.DialMs != 8000 {
		t.Errorf("Expected default DialMs to be 8000, got %d", cfg.Timeouts.DialMs)
	}
}

func TestLoadInvalidConfig(t *testing.T) {
	// Test loading non-existent file
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Error("Expected error when loading non-existent config file")
	}

	// Test loading invalid YAML
	configContent := `
listen:
  http_proxy: "0.0.0.0:8080"
  socks5_proxy: "invalid:yaml:content
`

	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(configContent)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = Load(tmpfile.Name())
	if err == nil {
		t.Error("Expected error when loading invalid YAML")
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				Listen: ListenConfig{
					HTTPProxy:  "0.0.0.0:8080",
					Socks5Proxy: "0.0.0.0:1080",
					API:        "0.0.0.0:8081",
				},
				Timeouts: TimeoutConfig{
					DialMs:  8000,
					ReadMs:  60000,
					WriteMs: 60000,
				},
				Refresh: RefreshConfig{
					IntervalSec:          900,
					HealthcheckConcurrency: 50,
				},
				Database: DatabaseConfig{
					Path: "/tmp/test.db",
				},
				Security: SecurityConfig{
					PasswordHash: "argon2id",
					Login: LoginConfig{
						MaxAttempts:   10,
						WindowSeconds: 900,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing http proxy",
			config: &Config{
				Listen: ListenConfig{
					Socks5Proxy: "0.0.0.0:1080",
					API:        "0.0.0.0:8081",
				},
				Timeouts: TimeoutConfig{
					DialMs:  8000,
					ReadMs:  60000,
					WriteMs: 60000,
				},
				Refresh: RefreshConfig{
					IntervalSec:          900,
					HealthcheckConcurrency: 50,
				},
				Database: DatabaseConfig{
					Path: "/tmp/test.db",
				},
				Security: SecurityConfig{
					PasswordHash: "argon2id",
					Login: LoginConfig{
						MaxAttempts:   10,
						WindowSeconds: 900,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing database path",
			config: &Config{
				Listen: ListenConfig{
					HTTPProxy:  "0.0.0.0:8080",
					Socks5Proxy: "0.0.0.0:1080",
					API:        "0.0.0.0:8081",
				},
				Timeouts: TimeoutConfig{
					DialMs:  8000,
					ReadMs:  60000,
					WriteMs: 60000,
				},
				Refresh: RefreshConfig{
					IntervalSec:          900,
					HealthcheckConcurrency: 50,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid dial timeout",
			config: &Config{
				Listen: ListenConfig{
					HTTPProxy:  "0.0.0.0:8080",
					Socks5Proxy: "0.0.0.0:1080",
					API:        "0.0.0.0:8081",
				},
				Timeouts: TimeoutConfig{
					DialMs:  -1,
					ReadMs:  60000,
					WriteMs: 60000,
				},
				Refresh: RefreshConfig{
					IntervalSec:          900,
					HealthcheckConcurrency: 50,
				},
				Database: DatabaseConfig{
					Path: "/tmp/test.db",
				},
				Security: SecurityConfig{
					PasswordHash: "argon2id",
					Login: LoginConfig{
						MaxAttempts:   10,
						WindowSeconds: 900,
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetTimeouts(t *testing.T) {
	cfg := &Config{
		Timeouts: TimeoutConfig{
			DialMs:  5000,
			ReadMs:  30000,
			WriteMs: 30000,
		},
		Refresh: RefreshConfig{
			IntervalSec: 600,
		},
	}

	if cfg.GetDialTimeout() != 5*time.Second {
		t.Errorf("Expected dial timeout to be 5s, got %v", cfg.GetDialTimeout())
	}

	if cfg.GetReadTimeout() != 30*time.Second {
		t.Errorf("Expected read timeout to be 30s, got %v", cfg.GetReadTimeout())
	}

	if cfg.GetWriteTimeout() != 30*time.Second {
		t.Errorf("Expected write timeout to be 30s, got %v", cfg.GetWriteTimeout())
	}

	if cfg.GetRefreshInterval() != 10*time.Minute {
		t.Errorf("Expected refresh interval to be 10m, got %v", cfg.GetRefreshInterval())
	}
}
