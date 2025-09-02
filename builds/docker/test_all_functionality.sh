#!/bin/bash

# Comprehensive ProxyRouter Test Script
# Tests all functionality including API endpoints, proxy operations, and Tor control

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
API_BASE="http://localhost:8081/api/v1"
HTTP_PROXY="http://localhost:8080"
SOCKS5_PROXY="socks5://localhost:1080"

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

# Function to wait for services to be ready
wait_for_services() {
    print_status "Waiting for services to be ready..."
    local max_attempts=30
    local attempt=1
    
    while [ $attempt -le $max_attempts ]; do
        if curl -s "${API_BASE}/healthz" > /dev/null 2>&1; then
            print_success "Services are ready!"
            return 0
        fi
        
        print_status "Attempt $attempt/$max_attempts - waiting for services..."
        sleep 2
        ((attempt++))
    done
    
    print_error "Services failed to start within expected time"
    return 1
}

# Function to test API endpoint
test_api_endpoint() {
    local endpoint="$1"
    local method="${2:-GET}"
    local data="$3"
    local description="$4"
    
    print_status "Testing $description..."
    
    local response
    if [ "$method" = "POST" ] && [ -n "$data" ]; then
        response=$(curl -s -X POST "http://localhost:8081${endpoint}" \
            -H "Content-Type: application/json" \
            -d "$data" 2>/dev/null || echo "ERROR")
    else
        response=$(curl -s -X "$method" "http://localhost:8081${endpoint}" 2>/dev/null || echo "ERROR")
    fi
    
    if [ "$response" = "ERROR" ]; then
        print_error "Failed to call $endpoint"
        return 1
    fi
    
    # Check if response contains error
    if echo "$response" | grep -q '"error"'; then
        print_error "API error: $response"
        return 1
    fi
    
    print_success "$description - Response: $response"
    return 0
}

# Function to test proxy functionality
test_proxy() {
    local proxy_type="$1"
    local proxy_url="$2"
    local description="$3"
    
    print_status "Testing $description..."
    
    local response
    if [ "$proxy_type" = "socks5" ]; then
        response=$(curl -s --socks5 "$proxy_url" http://ifconfig.me 2>/dev/null || echo "ERROR")
    else
        response=$(curl -s -x "$proxy_url" http://ifconfig.me 2>/dev/null || echo "ERROR")
    fi
    
    if [ "$response" = "ERROR" ]; then
        print_error "Failed to use $proxy_type proxy"
        return 1
    fi
    
    if [ -n "$response" ] && [ "$response" != "ERROR" ]; then
        print_success "$description - IP: $response"
        return 0
    else
        print_error "$description failed"
        return 1
    fi
}

# Main test function
run_tests() {
    print_status "Starting comprehensive ProxyRouter tests..."
    echo "================================================"
    
    # Wait for services
    if ! wait_for_services; then
        print_error "Services not ready, exiting tests"
        exit 1
    fi
    
    # Test 1: Health Check
    echo
    print_status "Test 1: Health Check"
    test_api_endpoint "/healthz" "GET" "" "Health Check"
    
    # Test 2: Version
    echo
    print_status "Test 2: Version"
    test_api_endpoint "/version" "GET" "" "Version Info"
    
    # Test 3: Metrics
    echo
    print_status "Test 3: Metrics"
    test_api_endpoint "/metrics" "GET" "" "Metrics"
    
    # Test 4: ACL Management
    echo
    print_status "Test 4: ACL Management"
    test_api_endpoint "/api/v1/acl" "GET" "" "Get ACL"
    test_api_endpoint "/api/v1/acl" "POST" '{"cidr": "192.168.100.0/24"}' "Add ACL"
    
    # Test 5: Routes Management
    echo
    print_status "Test 5: Routes Management"
    test_api_endpoint "/api/v1/routes" "GET" "" "Get Routes"
    test_api_endpoint "/api/v1/routes" "POST" '{"group": "GENERAL", "precedence": 200}' "Create Route"
    
    # Test 6: Proxies Management
    echo
    print_status "Test 6: Proxies Management"
    test_api_endpoint "/api/v1/proxies" "GET" "" "Get Proxies"
    
    # Test 7: Import Proxies
    echo
    print_status "Test 7: Import Proxies"
    test_api_endpoint "/api/v1/proxies/import" "POST" '[
        {"scheme":"socks5","host":"proxy1.example.com","port":1080,"source":"manual"},
        {"scheme":"socks5","host":"proxy2.example.com","port":1080,"source":"manual"}
    ]' "Import Proxies"
    
    # Test 8: Refresh Proxies
    echo
    print_status "Test 8: Refresh Proxies"
    test_api_endpoint "/api/v1/proxies/refresh" "POST" "" "Refresh Proxies"
    
    # Test 9: Check Proxy Health
    echo
    print_status "Test 9: Check Proxy Health"
    test_api_endpoint "/api/v1/proxies/1/check" "POST" "" "Check Proxy Health"
    
    # Test 10: Tor Control
    echo
    print_status "Test 10: Tor Control"
    test_api_endpoint "/api/v1/tor/status" "GET" "" "Tor Status"
    test_api_endpoint "/api/v1/tor/ip" "GET" "" "Tor IP"
    test_api_endpoint "/api/v1/tor/newcircuit" "POST" "" "Tor Circuit Rotation"
    
    # Test 11: Settings
    echo
    print_status "Test 11: Settings"
    test_api_endpoint "/api/v1/settings" "GET" "" "Get Settings"
    test_api_endpoint "/api/v1/settings" "PATCH" '{"test_setting": "test_value"}' "Update Settings"
    
    # Test 12: Proxy Functionality
    echo
    print_status "Test 12: Proxy Functionality"
    
    # Test SOCKS5 proxy
    test_proxy "socks5" "localhost:1080" "SOCKS5 Proxy"
    
    # Test HTTP proxy (if working)
    test_proxy "http" "localhost:8080" "HTTP Proxy"
    
    # Test direct Tor connection
    test_proxy "socks5" "localhost:9050" "Direct Tor Connection"
    
    # Test 13: Real IP comparison
    echo
    print_status "Test 13: Real IP Comparison"
    real_ip=$(curl -s http://ifconfig.me)
    print_status "Real IP: $real_ip"
    
    # Test 14: Tor IP after rotation
    echo
    print_status "Test 14: Tor IP Rotation Test"
    print_status "Getting Tor IP before rotation..."
    tor_ip_before=$(curl -s --socks5 localhost:1080 http://ifconfig.me)
    print_status "Tor IP before: $tor_ip_before"
    
    print_status "Rotating Tor circuit..."
    curl -s -X POST "http://localhost:8081/api/v1/tor/newcircuit" > /dev/null
    sleep 3
    
    print_status "Getting Tor IP after rotation..."
    tor_ip_after=$(curl -s --socks5 localhost:1080 http://ifconfig.me)
    print_status "Tor IP after: $tor_ip_after"
    
    if [ "$tor_ip_before" != "$tor_ip_after" ]; then
        print_success "Tor IP rotation successful! Before: $tor_ip_before, After: $tor_ip_after"
    else
        print_warning "Tor IP rotation may not have worked (same IP detected)"
    fi
    
    echo
    echo "================================================"
    print_success "All tests completed!"
}

# Run the tests
run_tests
