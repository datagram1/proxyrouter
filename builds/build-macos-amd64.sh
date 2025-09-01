#!/bin/bash

# Build script for macOS AMD64 (Intel)
set -e

echo "🔨 Building ProxyRouter for macOS AMD64 (Intel)..."

# Set build variables
export GOOS=darwin
export GOARCH=amd64
export CGO_ENABLED=0

# Build directory
BUILD_DIR="$(dirname "$0")/macos-amd64"
mkdir -p "$BUILD_DIR"

# Build binary
echo "Building binary..."
go build -ldflags="-s -w" -o "$BUILD_DIR/proxyrouter" ./cmd/proxyrouter

# Copy configuration files
echo "Copying configuration files..."
cp configs/config.yaml "$BUILD_DIR/"
cp -r migrations "$BUILD_DIR/"

# Create README for this build
cat > "$BUILD_DIR/README.md" << 'EOF'
# ProxyRouter for macOS AMD64 (Intel)

This build is specifically compiled for macOS AMD64 (Intel) systems.

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

- Native AMD64 performance on Intel Macs
- No external dependencies (static binary)
- Includes all configuration files and migrations

## System Requirements

- macOS 10.15 or later
- Intel Mac (x86_64)

## Default Configuration

- HTTP Proxy: 127.0.0.1:8080
- SOCKS5 Proxy: 127.0.0.1:1080
- API: 127.0.0.1:8081
- Admin UI: 127.0.0.1:5000

## Quick Start

```bash
# Start with default config
./proxyrouter

# Start with custom config
./proxyrouter -config config.yaml

# Show version
./proxyrouter -version
```
EOF

echo "✅ Build completed for macOS AMD64"
echo "📁 Build location: $BUILD_DIR"
echo "📦 Binary size: $(du -h "$BUILD_DIR/proxyrouter" | cut -f1)"
