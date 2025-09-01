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
docker run -p 8080:8080 -p 1080:1080 -p 8081:8081 -p 6000:6000 proxyrouter:latest
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
open http://localhost:6000/admin
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
  -p 6000:6000 \
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
- `6000` - Admin Web UI

## Security Notes

- Admin UI is bound to 0.0.0.0:6000 in Docker (not 127.0.0.1)
- Use reverse proxy (nginx/traefik) for production
- Consider using Docker secrets for sensitive data
- Enable TLS for production deployments
