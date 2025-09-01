#!/bin/bash

# Build script for Windows AMD64
set -e

echo "🔨 Building ProxyRouter for Windows AMD64..."

# Set build variables
export GOOS=windows
export GOARCH=amd64
export CGO_ENABLED=0

# Build directory
BUILD_DIR="$(dirname "$0")/windows-amd64"
mkdir -p "$BUILD_DIR"

# Build binary
echo "Building binary..."
go build -ldflags="-s -w" -o "$BUILD_DIR/proxyrouter.exe" ./cmd/proxyrouter

# Copy configuration files
echo "Copying configuration files..."
cp configs/config.yaml "$BUILD_DIR/"
cp -r migrations "$BUILD_DIR/"

# Create README for this build
cat > "$BUILD_DIR/README.md" << 'EOF'
# ProxyRouter for Windows AMD64

This build is specifically compiled for Windows AMD64 systems.

## Installation

1. Extract the files to a directory of your choice

2. Run the application:
   ```cmd
   proxyrouter.exe -config config.yaml
   ```

   Or in PowerShell:
   ```powershell
   .\proxyrouter.exe -config config.yaml
   ```

## Features

- Native Windows AMD64 performance
- No external dependencies (static binary)
- Includes all configuration files and migrations

## System Requirements

- Windows 10 or later
- AMD64/x86_64 processor

## Default Configuration

- HTTP Proxy: 127.0.0.1:8080
- SOCKS5 Proxy: 127.0.0.1:1080
- API: 127.0.0.1:8081
- Admin UI: 127.0.0.1:5000

## Quick Start

```cmd
# Start with default config
proxyrouter.exe

# Start with custom config
proxyrouter.exe -config config.yaml

# Show version
proxyrouter.exe -version
```

## Windows Service (Optional)

To run as a Windows service, you can use tools like:
- NSSM (Non-Sucking Service Manager)
- WinSW (Windows Service Wrapper)

Example with NSSM:
```cmd
nssm install ProxyRouter "C:\path\to\proxyrouter.exe"
nssm set ProxyRouter AppParameters "-config C:\path\to\config.yaml"
nssm start ProxyRouter
```
EOF

echo "✅ Build completed for Windows AMD64"
echo "📁 Build location: $BUILD_DIR"
echo "📦 Binary size: $(du -h "$BUILD_DIR/proxyrouter.exe" | cut -f1)"
