# ProxyRouter for macOS ARM64 (Apple Silicon)

This build is specifically compiled for macOS ARM64 (Apple Silicon) systems.

## Installation

1. Make the binary executable:
   ```bash
   chmod +x proxyrouter
   ```

2. Run the application:
   ```bash
   ./proxyrouter -config config.yaml
   ```

## Features

- Native ARM64 performance on Apple Silicon
- No external dependencies (static binary)
- Includes all configuration files and migrations

## System Requirements

- macOS 11.0 or later
- Apple Silicon Mac (M1, M1 Pro, M1 Max, M2, etc.)

## Default Configuration

- HTTP Proxy: 127.0.0.1:8080
- SOCKS5 Proxy: 127.0.0.1:1080
- API: 127.0.0.1:8081
- Admin UI: 127.0.0.1:6000

## Quick Start

```bash
# Start with default config
./proxyrouter

# Start with custom config
./proxyrouter -config config.yaml

# Show version
./proxyrouter -version
```
