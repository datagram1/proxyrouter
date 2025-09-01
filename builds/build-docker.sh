#!/bin/bash

# Build script for Docker images
set -e

echo "🐳 Building ProxyRouter Docker images..."

# Build directory
BUILD_DIR="$(dirname "$0")/docker"
mkdir -p "$BUILD_DIR"

# Copy Docker files
echo "Copying Docker files..."
cp Dockerfile "$BUILD_DIR/"
cp docker-compose.yaml "$BUILD_DIR/"
cp test-docker.sh "$BUILD_DIR/"
cp DOCKER_TESTING.md "$BUILD_DIR/"
cp DOCKER_QUICK_START.md "$BUILD_DIR/"

# Copy configuration files
echo "Copying configuration files..."
cp configs/config.yaml "$BUILD_DIR/"
cp -r migrations "$BUILD_DIR/"

# Create Docker-specific configuration
echo "Creating Docker configuration..."
cat > "$BUILD_DIR/docker-config.yaml" << 'EOF'
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
  socks_address: "tor:9050"

refresh:
  enable_general_sources: false
  interval_sec: 900
  healthcheck_concurrency: 10
  sources: []

database:
  path: "/var/lib/proxyr/proxyrouter.db"

logging:
  level: "info"
  format: "text"

metrics:
  enabled: true
  path: "/metrics"

admin:
  enabled: true
  bind: "0.0.0.0"
  port: 5000
  basePath: "/admin"
  sessionSecret: ""
  allowCIDRs: ["0.0.0.0/0"]
  tls:
    enabled: false

security:
  passwordHash: "argon2id"
  login:
    maxAttempts: 10
    windowSeconds: 900
EOF

# Create docker-compose for testing
cat > "$BUILD_DIR/docker-compose.test.yaml" << 'EOF'
version: '3.8'

services:
  tor:
    image: dperson/torproxy:latest
    restart: unless-stopped
    ports:
      - "9050:9050"
    environment:
      - TZ=UTC

  proxyrouter:
    build: .
    depends_on: [tor]
    restart: unless-stopped
    ports:
      - "8080:8080"   # HTTP proxy
      - "1080:1080"   # SOCKS5 proxy
      - "8081:8081"   # API
      - "5000:5000"   # Admin UI
    volumes:
      - ./data:/var/lib/proxyr
      - ./docker-config.yaml:/etc/proxyrouter/config.yaml:ro
    environment:
      - TOR_SOCKS=tor:9050
    command: ["/usr/local/bin/proxyrouter", "-config", "/etc/proxyrouter/config.yaml"]
EOF

# Create README for Docker builds
cat > "$BUILD_DIR/README.md" << 'EOF'
# ProxyRouter Docker Builds

This directory contains Docker-specific builds and configurations for ProxyRouter.

## Quick Start

### One-Command Testing
```bash
chmod +x test-docker.sh
./test-docker.sh start
```

### Manual Setup
```bash
# Build and run with Tor sidecar
docker-compose up --build

# Or build image separately
docker build -t proxyrouter:latest .
docker run -p 8080:8080 -p 1080:1080 -p 8081:8081 -p 5000:5000 proxyrouter:latest
```

## Multi-Architecture Builds

### Build for specific platform
```bash
# AMD64
docker build --platform linux/amd64 -t proxyrouter:amd64 .

# ARM64
docker build --platform linux/arm64 -t proxyrouter:arm64 .

# Multi-platform
docker buildx build --platform linux/amd64,linux/arm64 -t proxyrouter:latest .
```

## Configuration Files

- `docker-config.yaml` - Docker-optimized configuration
- `docker-compose.yaml` - Production compose file
- `docker-compose.test.yaml` - Testing compose file with Tor

## Testing

### Test Services
```bash
# API
curl http://localhost:8081/healthz

# HTTP Proxy
curl -x http://localhost:8080 http://httpbin.org/ip

# SOCKS5 Proxy
curl --socks5 localhost:1080 http://httpbin.org/ip

# Admin UI
open http://localhost:5000/admin
```

### Default Credentials
- Username: `admin`
- Password: `admin`

## Production Deployment

### Docker Compose
```bash
# Start services
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

### Docker Run
```bash
# Create data volume
docker volume create proxyrouter-data

# Run container
docker run -d \
  --name proxyrouter \
  -p 8080:8080 \
  -p 1080:1080 \
  -p 8081:8081 \
  -p 5000:5000 \
  -v proxyrouter-data:/var/lib/proxyr \
  proxyrouter:latest
```

## Environment Variables

- `TOR_SOCKS` - Tor SOCKS address (default: tor:9050)
- `CONFIG_PATH` - Configuration file path (default: /etc/proxyrouter/config.yaml)

## Volumes

- `/var/lib/proxyr` - Database and persistent data
- `/etc/proxyrouter/config.yaml` - Configuration file

## Ports

- `8080` - HTTP Proxy
- `1080` - SOCKS5 Proxy
- `8081` - REST API
- `5000` - Admin Web UI

## Security Notes

- Admin UI is bound to 0.0.0.0:5000 in Docker (not 127.0.0.1)
- Use reverse proxy (nginx/traefik) for production
- Consider using Docker secrets for sensitive data
- Enable TLS for production deployments
EOF

echo "✅ Docker build files prepared"
echo "📁 Build location: $BUILD_DIR"
echo "🐳 Files created:"
echo "  - Dockerfile"
echo "  - docker-compose.yaml"
echo "  - docker-compose.test.yaml"
echo "  - docker-config.yaml"
echo "  - test-docker.sh"
echo "  - README.md"
