#!/bin/bash

# Build script for Linux AMD64
set -e

echo "🔨 Building ProxyRouter for Linux AMD64..."

# Set build variables
export GOOS=linux
export GOARCH=amd64
export CGO_ENABLED=0

# Build directory
BUILD_DIR="$(dirname "$0")/linux-amd64"
mkdir -p "$BUILD_DIR"

# Build binary
echo "Building binary..."
go build -ldflags="-s -w" -o "$BUILD_DIR/proxyrouter" ./cmd/proxyrouter

# Copy configuration files
echo "Copying configuration files..."
cp configs/config.yaml "$BUILD_DIR/"
cp -r migrations "$BUILD_DIR/"
cp systemd/proxyrouter.service "$BUILD_DIR/"

# Create README for this build
cat > "$BUILD_DIR/README.md" << 'EOF'
# ProxyRouter for Linux AMD64

This build is specifically compiled for Linux AMD64 systems.

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

- Native Linux AMD64 performance
- No external dependencies (static binary)
- Includes all configuration files and migrations
- Systemd service file included

## System Requirements

- Linux kernel 3.10 or later
- AMD64/x86_64 processor
- glibc 2.17 or later (for most distributions)

## Default Configuration

- HTTP Proxy: 0.0.0.0:8080
- SOCKS5 Proxy: 0.0.0.0:1080
- API: 0.0.0.0:8081
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

## Systemd Service Installation

1. Copy the service file:
   ```bash
   sudo cp proxyrouter.service /etc/systemd/system/
   ```

2. Create data directory:
   ```bash
   sudo mkdir -p /var/lib/proxyr
   sudo chown $USER:$USER /var/lib/proxyr
   ```

3. Enable and start the service:
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable proxyrouter
   sudo systemctl start proxyrouter
   ```

4. Check status:
   ```bash
   sudo systemctl status proxyrouter
   ```

## Docker

This build can also be used in Docker containers:

```bash
# Build Docker image
docker build -t proxyrouter .

# Run container
docker run -p 8080:8080 -p 1080:1080 -p 8081:8081 -p 5000:5000 proxyrouter
```
EOF

echo "✅ Build completed for Linux AMD64"
echo "📁 Build location: $BUILD_DIR"
echo "📦 Binary size: $(du -h "$BUILD_DIR/proxyrouter" | cut -f1)"
