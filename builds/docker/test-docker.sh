#!/bin/bash

# ProxyRouter Docker Testing Script for ARM Mac
# This script automates the Docker testing process

set -e

echo "🚀 ProxyRouter Docker Testing Script for ARM Mac"
echo "================================================"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    print_status "Checking prerequisites..."
    
    if ! command -v docker &> /dev/null; then
        print_error "Docker is not installed. Please install Docker Desktop for Mac."
        exit 1
    fi
    
    if ! docker info &> /dev/null; then
        print_error "Docker is not running. Please start Docker Desktop."
        exit 1
    fi
    
    if ! command -v docker-compose &> /dev/null; then
        print_error "Docker Compose is not installed."
        exit 1
    fi
    
    if ! command -v curl &> /dev/null; then
        print_warning "curl is not installed. Some tests will be skipped."
    fi
    
    print_success "Prerequisites check completed"
}

# Create test configuration
create_test_config() {
    print_status "Creating test configuration..."
    
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
    build:
      context: ../..
      dockerfile: builds/docker/Dockerfile
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

    print_success "Test configuration created"
}

# Build and start services
start_services() {
    print_status "Building Docker image..."
    docker build -t proxyrouter:test -f Dockerfile ../..
    
    print_status "Creating data directory..."
    mkdir -p data
    
    print_status "Starting services..."
    docker-compose -f docker-compose.test.yaml up -d
    
    print_status "Waiting for services to start..."
    sleep 10
    
    print_success "Services started successfully"
}

# Test services
test_services() {
    print_status "Testing services..."
    
    # Test API endpoints
    if command -v curl &> /dev/null; then
        print_status "Testing API endpoints..."
        
        # Health check
        if curl -s http://localhost:8081/healthz > /dev/null; then
            print_success "Health check endpoint is working"
        else
            print_error "Health check endpoint failed"
        fi
        
        # Version
        if curl -s http://localhost:8081/version > /dev/null; then
            print_success "Version endpoint is working"
        else
            print_error "Version endpoint failed"
        fi
        
        # Metrics
        if curl -s http://localhost:8081/metrics > /dev/null; then
            print_success "Metrics endpoint is working"
        else
            print_error "Metrics endpoint failed"
        fi
        
        # Test HTTP proxy
        print_status "Testing HTTP proxy..."
        if curl -x http://localhost:8080 -s http://httpbin.org/ip > /dev/null; then
            print_success "HTTP proxy is working"
        else
            print_error "HTTP proxy failed"
        fi
        
        # Test SOCKS5 proxy
        print_status "Testing SOCKS5 proxy..."
        if curl --socks5 localhost:1080 -s http://httpbin.org/ip > /dev/null; then
            print_success "SOCKS5 proxy is working"
        else
            print_error "SOCKS5 proxy failed"
        fi
        
        # Test Admin UI
        print_status "Testing Admin UI..."
        if curl -s http://localhost:6000/admin/login > /dev/null; then
            print_success "Admin UI is accessible"
        else
            print_error "Admin UI is not accessible"
        fi
        
        # Test Tor
        print_status "Testing Tor connectivity..."
        if curl --socks5 localhost:9050 -s http://httpbin.org/ip > /dev/null; then
            print_success "Tor is working"
        else
            print_warning "Tor connectivity failed (this is normal if Tor is still starting)"
        fi
    else
        print_warning "curl not available, skipping API tests"
    fi
}

# Show service information
show_info() {
    print_status "Service Information:"
    echo "  HTTP Proxy:     http://localhost:8080"
    echo "  SOCKS5 Proxy:   socks5://localhost:1080"
    echo "  API:           http://localhost:8081"
    echo "  Admin UI:      http://localhost:6000/admin"
    echo "  Tor:           socks5://localhost:9050"
    echo ""
    echo "  Default Admin Credentials:"
    echo "    Username: admin"
    echo "    Password: admin"
    echo ""
    echo "  Useful Commands:"
    echo "    View logs:    docker-compose -f docker-compose.test.yaml logs -f"
    echo "    Stop:         docker-compose -f docker-compose.test.yaml down"
    echo "    Restart:      docker-compose -f docker-compose.test.yaml restart"
    echo ""
}

# Cleanup function
cleanup() {
    print_status "Cleaning up..."
    docker-compose -f docker-compose.test.yaml down 2>/dev/null || true
    rm -f docker-test-config.yaml docker-compose.test.yaml
    print_success "Cleanup completed"
}

# Main execution
main() {
    case "${1:-start}" in
        "start")
            check_prerequisites
            create_test_config
            start_services
            test_services
            show_info
            ;;
        "stop")
            cleanup
            ;;
        "restart")
            cleanup
            check_prerequisites
            create_test_config
            start_services
            test_services
            show_info
            ;;
        "test")
            test_services
            ;;
        "logs")
            docker-compose -f docker-compose.test.yaml logs -f
            ;;
        "shell")
            docker-compose -f docker-compose.test.yaml exec proxyrouter /bin/sh
            ;;
        *)
            echo "Usage: $0 {start|stop|restart|test|logs|shell}"
            echo ""
            echo "Commands:"
            echo "  start   - Start services and run tests"
            echo "  stop    - Stop and cleanup services"
            echo "  restart - Restart services"
            echo "  test    - Run tests only"
            echo "  logs    - Show logs"
            echo "  shell   - Access container shell"
            exit 1
            ;;
    esac
}

# Handle script interruption
# trap cleanup EXIT

# Run main function
main "$@"
