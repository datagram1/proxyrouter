# ProxyRouter

A high-performance LAN proxy router for Ubuntu Linux, written in Go. ProxyRouter exposes HTTP and SOCKS5 proxy servers, routes each request via LOCAL, GENERAL, TOR, or a chosen UPSTREAM proxy, and provides a REST API for control.

## Features

- **HTTP Proxy Server** (`0.0.0.0:8080`) - Supports HTTP forward + HTTPS CONNECT tunneling
- **SOCKS5 Proxy Server** (`0.0.0.0:1080`) - Full SOCKS5 protocol support
- **REST API** (`0.0.0.0:8081`) - JSON API for configuration and monitoring
- **Routing Engine** - Routes requests by policy into four groups:
  - **LOCAL** → direct connection
  - **GENERAL** → randomly selected, healthy proxy from a downloaded pool
  - **TOR** → via a Tor SOCKS5 daemon (`127.0.0.1:9050`)
  - **UPSTREAM** → a specific proxy chosen from the database
- **Access Control** - Only clients from `192.168.10.0/24` and `192.168.11.0/24` may connect (configurable)
- **SQLite Database** - Fast, lightweight storage for proxies, routes, ACLs, and settings
- **Docker Support** - Run as a container with Tor sidecar
- **Systemd Integration** - Run as a native Linux service

## Quick Start

### Prerequisites

- Go 1.22 or later
- SQLite (included via modernc.org/sqlite)
- Tor daemon (optional, for TOR routing)

### Build and Run

```bash
# Clone the repository
git clone https://github.com/yourusername/proxyrouter.git
cd proxyrouter

# Install dependencies and build
make deps
make build

# Run the application
./bin/proxyrouter -config configs/config.yaml
```

### Docker

```bash
# Build and run with Tor sidecar
docker-compose up --build

# Or build image separately
make docker
docker run -p 8080:8080 -p 1080:1080 -p 8081:8081 proxyrouter
```

### Systemd Service

```bash
# Install systemd service
sudo cp systemd/proxyrouter.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable proxyrouter
sudo systemctl start proxyrouter

# Check status
sudo systemctl status proxyrouter
```

## Configuration

The application uses YAML configuration with environment variable overrides. Default configuration is in `configs/config.yaml`:

```yaml
listen:
  http_proxy: "0.0.0.0:8080"
  socks5_proxy: "0.0.0.0:1080"
  api: "0.0.0.0:8081"

timeouts:
  dial_ms: 8000
  read_ms: 60000
  write_ms: 60000

tor:
  enabled: true
  socks_address: "127.0.0.1:9050"

refresh:
  enable_general_sources: true
  interval_sec: 900
  healthcheck_concurrency: 50
  sources:
    - name: "spys.one-gb"
      url: "https://spys.one/free-proxy-list/GB/"
      type: "html"
    - name: "raw-list"
      url: "https://example.com/proxies.txt"
      type: "raw"

database:
  path: "/var/lib/proxyr/router.db"

logging:
  level: "info"
  format: "json"

metrics:
  enabled: true
  path: "/metrics"
```

## API Reference

### Base URL
`http://host:8081/v1`

### Endpoints

#### Health Check
```http
GET /healthz
```
Returns: `{"status":"ok","time":"2024-01-01T00:00:00Z"}`

#### Metrics
```http
GET /metrics
```
Returns Prometheus metrics

#### ACL Management
```http
GET /acl                    # List ACL subnets
POST /acl                   # Add ACL subnet
DELETE /acl/{id}            # Remove ACL subnet
```

#### Route Management
```http
GET /routes                 # List routes
POST /routes                # Create route
PATCH /routes/{id}          # Update route
DELETE /routes/{id}         # Delete route
```

#### Proxy Management
```http
GET /proxies                # List proxies
POST /proxies/import        # Import proxies
POST /proxies/refresh       # Refresh from sources
POST /proxies/{id}/check    # Health check proxy
DELETE /proxies/{id}        # Delete proxy
```

#### Settings
```http
GET /settings               # Get settings
PATCH /settings             # Update settings
```

### Example API Usage

```bash
# List routes
curl http://localhost:8081/v1/routes

# Route *.github.com via LOCAL at high precedence
curl -X POST http://localhost:8081/v1/routes \
  -H 'content-type: application/json' \
  -d '{"host_glob":"*.github.com","group":"LOCAL","precedence":10}'

# Add ACL subnet
curl -X POST http://localhost:8081/v1/acl \
  -H 'content-type: application/json' \
  -d '{"cidr":"192.168.10.0/24"}'

# Import an upstream manually
curl -X POST http://localhost:8081/v1/proxies/import \
  -H 'content-type: application/json' \
  -d '[{"scheme":"socks5","host":"1.2.3.4","port":1080,"source":"manual"}]'

# Trigger refresh
curl -X POST http://localhost:8081/v1/proxies/refresh
```

## Database Schema

### Proxies Table
```sql
CREATE TABLE proxies (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  scheme TEXT NOT NULL,             -- "socks5" | "http" | "https"
  host TEXT NOT NULL,
  port INTEGER NOT NULL,
  source TEXT,                      -- e.g., "spys.one-gb" or "manual"
  latency_ms INTEGER,
  alive INTEGER NOT NULL DEFAULT 1,
  last_checked_at DATETIME,
  expires_at DATETIME,              -- null = persistent
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE (scheme, host, port)
);
```

### Routes Table
```sql
CREATE TABLE routes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  client_cidr TEXT,                 -- e.g. "192.168.10.0/24" (nullable = any client)
  host_glob TEXT,                   -- e.g. "*.github.com" (nullable = any host)
  group TEXT NOT NULL,              -- "LOCAL"|"GENERAL"|"TOR"|"UPSTREAM"
  proxy_id INTEGER,                 -- used when group="UPSTREAM"
  precedence INTEGER NOT NULL DEFAULT 100,
  enabled INTEGER NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### ACL Subnets Table
```sql
CREATE TABLE acl_subnets (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  cidr TEXT NOT NULL UNIQUE
);
```

### Settings Table
```sql
CREATE TABLE settings (
  k TEXT PRIMARY KEY,
  v TEXT NOT NULL
);
```

## Development

### Project Structure
```
proxyrouter/
├── cmd/proxyrouter/main.go          # Main application entry point
├── internal/
│   ├── config/config.go             # Configuration loader (Viper)
│   ├── db/database.go               # SQLite connection & migrations
│   ├── acl/acl.go                   # CIDR-based access control
│   ├── router/router.go             # Routing engine
│   ├── router/dialer.go             # Dialer factory
│   ├── proxyhttp/server.go          # HTTP proxy server
│   ├── proxysocks/server.go         # SOCKS5 proxy server
│   └── api/server.go                # REST API server (Chi)
├── migrations/                      # Database migrations
├── configs/config.yaml              # Default configuration
├── Makefile                         # Build targets
├── Dockerfile                       # Multi-stage build
├── docker-compose.yaml              # With Tor sidecar
├── systemd/proxyrouter.service      # Systemd unit
└── proxies.py                       # Reference implementation
```

### Build Commands
```bash
make deps          # Install dependencies
make build         # Build static binary
make test          # Run tests
make docker        # Build Docker image
make clean         # Clean build artifacts
```

### Testing
```bash
# Run all tests
make test

# Run tests with race detection
make test-race

# Run specific package tests
go test ./internal/router
go test ./internal/acl
```

## Routing Rules

Resolution order:
1. **ACL check** (client IP ∈ allowlist) → otherwise 403
2. **Highest-precedence matching route** by (client_cidr, host_glob)
3. **group → dialer**:
   - **LOCAL**: direct net.Dialer
   - **TOR**: SOCKS5 dialer to tor.socks_address
   - **GENERAL**: choose best alive, unexpired proxy (lowest latency, most recent success)
   - **UPSTREAM**: use proxy_id or choose by label

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Roadmap

- [ ] Complete SOCKS5 server implementation
- [ ] Implement proxy refresh system
- [ ] Add structured logging
- [ ] Add Prometheus metrics
- [ ] Implement authentication
- [ ] Add rate limiting
- [ ] Web dashboard
- [ ] Cluster/distributed mode

## Acknowledgments

- Based on concepts from the reference `proxies.py` implementation
- Uses [modernc.org/sqlite](https://modernc.org/sqlite) for pure Go SQLite
- Uses [go-chi/chi](https://github.com/go-chi/chi) for HTTP routing
- Uses [armon/go-socks5](https://github.com/armon/go-socks5) for SOCKS5 server
