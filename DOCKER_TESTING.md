# Testing ProxyRouter in Docker on ARM Mac

This guide provides step-by-step instructions for testing ProxyRouter in a local Docker container on Apple Silicon (ARM) Macs.

## Prerequisites

1. **Docker Desktop for Mac** installed and running
2. **Git** for cloning the repository
3. **curl** or **wget** for testing HTTP endpoints
4. **nc** (netcat) for testing SOCKS5 proxy

## Quick Start

### 1. Clone and Build

```bash
# Clone the repository
git clone <repository-url>
cd proxyrouter

# Build the Docker image
docker build -t proxyrouter:latest .
```

### 2. Create Test Configuration

Create a test configuration file for Docker testing:

```bash
cat > docker-test-config.yaml << 'EOF'
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
  port: 6000
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
```

### 3. Create Docker Compose for Testing

Create a test docker-compose file:

```bash
cat > docker-compose.test.yaml << 'EOF'
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
      - "6000:6000"   # Admin UI
    volumes:
      - ./data:/var/lib/proxyr
      - ./docker-test-config.yaml:/etc/proxyrouter/config.yaml:ro
    environment:
      - TOR_SOCKS=tor:9050
    command: ["/usr/local/bin/proxyrouter", "-config", "/etc/proxyrouter/config.yaml"]
EOF
```

### 4. Start the Services

```bash
# Create data directory
mkdir -p data

# Start services
docker-compose -f docker-compose.test.yaml up -d

# Check service status
docker-compose -f docker-compose.test.yaml ps

# View logs
docker-compose -f docker-compose.test.yaml logs -f proxyrouter
```

## Testing the Services

### 1. Test API Endpoints

```bash
# Health check
curl -s http://localhost:8081/healthz | jq

# Metrics
curl -s http://localhost:8081/metrics

# Version
curl -s http://localhost:8081/version | jq

# ACL status
curl -s http://localhost:8081/v1/acl | jq

# Routes
curl -s http://localhost:8081/v1/routes | jq

# Proxies
curl -s http://localhost:8081/v1/proxies | jq
```

### 2. Test HTTP Proxy

```bash
# Test HTTP proxy with curl
curl -x http://localhost:8080 http://httpbin.org/ip

# Test HTTPS proxy with curl
curl -x http://localhost:8080 https://httpbin.org/ip

# Test with environment variables
export http_proxy=http://localhost:8080
export https_proxy=http://localhost:8080
curl http://httpbin.org/ip
unset http_proxy https_proxy
```

### 3. Test SOCKS5 Proxy

```bash
# Test SOCKS5 proxy with curl
curl --socks5 localhost:1080 http://httpbin.org/ip

# Test with netcat (if available)
echo -e "GET http://httpbin.org/ip HTTP/1.1\r\nHost: httpbin.org\r\n\r\n" | \
  nc -X 5 -x localhost:1080 httpbin.org 80
```

### 4. Test Admin UI

```bash
# Open Admin UI in browser
open http://localhost:6000/admin

# Or test with curl
curl -s http://localhost:6000/admin/login

# Test login (will redirect to dashboard if successful)
curl -c cookies.txt -b cookies.txt -X POST http://localhost:6000/admin/login \
  -d "username=admin&password=admin" \
  -H "Content-Type: application/x-www-form-urlencoded"
```

### 5. Test Tor Integration

```bash
# Check if Tor is accessible
curl --socks5 localhost:9050 http://httpbin.org/ip

# Test through ProxyRouter with Tor route
# (This requires setting up a route in the admin UI or API)
```

## Advanced Testing

### 1. Load Testing

```bash
# Install hey (load testing tool)
brew install hey

# Test HTTP proxy performance
hey -n 100 -c 10 -x http://localhost:8080 http://httpbin.org/delay/1

# Test API endpoints
hey -n 50 -c 5 http://localhost:8081/healthz
```

### 2. Network Testing

```bash
# Test different protocols
curl -x http://localhost:8080 ftp://ftp.gnu.org/README

# Test with different user agents
curl -x http://localhost:8080 -H "User-Agent: TestBot/1.0" http://httpbin.org/user-agent

# Test large file download
curl -x http://localhost:8080 -o /dev/null http://speedtest.ftp.otenet.gr/files/test100k.db
```

### 3. Security Testing

```bash
# Test ACL restrictions (if configured)
curl -x http://localhost:8080 http://httpbin.org/ip

# Test admin UI security headers
curl -I http://localhost:6000/admin/login

# Test rate limiting
for i in {1..20}; do
  curl -s http://localhost:8081/healthz > /dev/null
done
```

## Monitoring and Debugging

### 1. View Logs

```bash
# View all logs
docker-compose -f docker-compose.test.yaml logs

# Follow logs in real-time
docker-compose -f docker-compose.test.yaml logs -f

# View specific service logs
docker-compose -f docker-compose.test.yaml logs proxyrouter
docker-compose -f docker-compose.test.yaml logs tor
```

### 2. Access Container Shell

```bash
# Access ProxyRouter container
docker-compose -f docker-compose.test.yaml exec proxyrouter /bin/sh

# Access Tor container
docker-compose -f docker-compose.test.yaml exec tor /bin/sh
```

### 3. Check Database

```bash
# Access the SQLite database
docker-compose -f docker-compose.test.yaml exec proxyrouter sqlite3 /var/lib/proxyr/proxyrouter.db

# Example queries:
# .tables
# SELECT * FROM admin_users;
# SELECT * FROM proxies;
# SELECT * FROM routes;
```

### 4. Monitor Resources

```bash
# Check container resource usage
docker stats

# Check specific container
docker stats proxyrouter tor
```

## Troubleshooting

### Common Issues

1. **Port Already in Use**
   ```bash
   # Check what's using the ports
   lsof -i :8080 -i :1080 -i :8081 -i :6000 -i :9050
   
   # Stop conflicting services
   sudo lsof -ti:8080 | xargs kill -9
   ```

2. **Permission Issues**
   ```bash
   # Fix data directory permissions
   sudo chown -R $(whoami):$(id -gn) data/
   ```

3. **Tor Connection Issues**
   ```bash
   # Check Tor container logs
   docker-compose -f docker-compose.test.yaml logs tor
   
   # Test Tor connectivity
   curl --socks5 localhost:9050 http://httpbin.org/ip
   ```

4. **Database Issues**
   ```bash
   # Reset database
   rm -rf data/*
   docker-compose -f docker-compose.test.yaml restart proxyrouter
   ```

### Performance Tuning

1. **Increase Docker Resources**
   - Open Docker Desktop → Settings → Resources
   - Increase CPU, Memory, and Disk allocation

2. **Optimize for ARM**
   ```bash
   # Build with ARM optimizations
   docker build --platform linux/arm64 -t proxyrouter:arm64 .
   ```

## Cleanup

```bash
# Stop and remove containers
docker-compose -f docker-compose.test.yaml down

# Remove volumes (this will delete the database)
docker-compose -f docker-compose.test.yaml down -v

# Remove images
docker rmi proxyrouter:latest

# Clean up data
rm -rf data/
rm docker-test-config.yaml docker-compose.test.yaml
```

## Next Steps

After successful testing:

1. **Configure Production Settings**: Update configuration for production use
2. **Set Up Monitoring**: Configure Prometheus/Grafana for metrics
3. **Security Hardening**: Review and tighten security settings
4. **Performance Tuning**: Optimize based on your use case
5. **Documentation**: Create deployment guides for your environment

## Support

If you encounter issues:

1. Check the logs: `docker-compose -f docker-compose.test.yaml logs`
2. Verify Docker Desktop is running and has sufficient resources
3. Ensure ports are not in use by other services
4. Check the GitHub issues for known problems
