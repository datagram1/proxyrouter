#!/bin/bash

# Comprehensive API and Proxy Testing Script
# This script tests all API endpoints and proxy functionality

set -e

echo "🚀 ProxyRouter Comprehensive Testing Script"
echo "============================================"

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

# Test function
test_endpoint() {
    local method=$1
    local endpoint=$2
    local data=$3
    local description=$4
    
    print_status "Testing $description: $method $endpoint"
    
    if [ "$method" = "GET" ]; then
        response=$(curl -s -w "%{http_code}" -o /tmp/response.json "http://127.0.0.1:8081$endpoint")
    elif [ "$method" = "POST" ]; then
        response=$(curl -s -w "%{http_code}" -o /tmp/response.json -X POST -H "Content-Type: application/json" -d "$data" "http://127.0.0.1:8081$endpoint")
    elif [ "$method" = "PUT" ]; then
        response=$(curl -s -w "%{http_code}" -o /tmp/response.json -X PUT -H "Content-Type: application/json" -d "$data" "http://127.0.0.1:8081$endpoint")
    elif [ "$method" = "DELETE" ]; then
        response=$(curl -s -w "%{http_code}" -o /tmp/response.json -X DELETE "http://127.0.0.1:8081$endpoint")
    fi
    
    http_code="${response: -3}"
    response_body=$(cat /tmp/response.json)
    
    if [ "$http_code" = "200" ] || [ "$http_code" = "201" ]; then
        print_success "$description: HTTP $http_code"
        echo "Response: $response_body" | head -c 100
        echo ""
    else
        print_error "$description: HTTP $http_code"
        echo "Response: $response_body"
    fi
}

# Wait for services to be ready
print_status "Waiting for services to be ready..."
sleep 5

# Test basic connectivity
print_status "Testing basic connectivity..."
if curl -s http://127.0.0.1:8081/healthz > /dev/null; then
    print_success "API server is accessible"
else
    print_error "API server is not accessible"
    exit 1
fi

# Test API v1 endpoints
echo ""
print_status "Testing API v1 endpoints..."

# Health check
test_endpoint "GET" "/api/v1/healthz" "" "Health Check"

# Version
test_endpoint "GET" "/api/v1/version" "" "Version Info"

# ACL endpoints
test_endpoint "GET" "/api/v1/acl" "" "Get ACL Rules"
test_endpoint "POST" "/api/v1/acl" '{"cidr":"192.168.100.0/24"}' "Add ACL Rule"

# Routes endpoints
test_endpoint "GET" "/api/v1/routes" "" "Get Routes"
test_endpoint "POST" "/api/v1/routes" '{"host_pattern":"*","group":"LOCAL","precedence":1}' "Create Route"

# Proxies endpoints
test_endpoint "GET" "/api/v1/proxies" "" "Get Proxies"
test_endpoint "POST" "/api/v1/proxies/refresh" "" "Refresh Proxies"

# Settings endpoints
test_endpoint "GET" "/api/v1/settings" "" "Get Settings"
test_endpoint "PATCH" "/api/v1/settings" '{"test_setting":"test_value"}' "Update Settings"

# Tor endpoints
test_endpoint "GET" "/api/v1/tor/status" "" "Tor Status"
test_endpoint "GET" "/api/v1/tor/ip" "" "Tor IP"

# Test proxy functionality
echo ""
print_status "Testing proxy functionality..."

# Get baseline IP
print_status "Getting baseline IP..."
baseline_ip=$(curl -s ifconfig.me)
print_success "Baseline IP: $baseline_ip"

# Test HTTP proxy
print_status "Testing HTTP proxy..."
if timeout 10 curl -x http://127.0.0.1:8080 -s ifconfig.me > /tmp/proxy_ip.txt; then
    proxy_ip=$(cat /tmp/proxy_ip.txt)
    print_success "HTTP Proxy IP: $proxy_ip"
    if [ "$baseline_ip" != "$proxy_ip" ]; then
        print_success "HTTP proxy is routing traffic (IP changed)"
    else
        print_warning "HTTP proxy may not be routing traffic (IP unchanged)"
    fi
else
    print_error "HTTP proxy test failed"
fi

# Test SOCKS5 proxy
print_status "Testing SOCKS5 proxy..."
if timeout 10 curl --socks5 127.0.0.1:1080 -s ifconfig.me > /tmp/socks_ip.txt; then
    socks_ip=$(cat /tmp/socks_ip.txt)
    print_success "SOCKS5 Proxy IP: $socks_ip"
    if [ "$baseline_ip" != "$socks_ip" ]; then
        print_success "SOCKS5 proxy is routing traffic (IP changed)"
    else
        print_warning "SOCKS5 proxy may not be routing traffic (IP unchanged)"
    fi
else
    print_error "SOCKS5 proxy test failed"
fi

# Test web interface
echo ""
print_status "Testing web interface..."
if curl -s http://127.0.0.1:8082/ > /dev/null; then
    print_success "Web interface is accessible"
else
    print_warning "Web interface is not accessible"
fi

# Test metrics
echo ""
print_status "Testing metrics endpoint..."
if curl -s http://127.0.0.1:8081/metrics > /dev/null; then
    print_success "Metrics endpoint is accessible"
else
    print_warning "Metrics endpoint is not accessible"
fi

echo ""
print_success "Testing completed!"
