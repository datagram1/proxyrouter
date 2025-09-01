#!/bin/bash

# Build script for macOS ARM64 (Apple Silicon)
set -e

echo "🔨 Building ProxyRouter for macOS ARM64..."

# Set build variables
export GOOS=darwin
export GOARCH=arm64
export CGO_ENABLED=0

# Build directory
BUILD_DIR="$(dirname "$0")/macos-arm64"
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

echo "✅ Build completed for macOS ARM64"
echo "📁 Build location: $BUILD_DIR"
echo "📦 Binary size: $(du -h "$BUILD_DIR/proxyrouter" | cut -f1)"
