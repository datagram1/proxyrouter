#!/bin/bash

# Test script for improved health check output
# This script will test a small set of proxies to show the verbose output

set -e

echo "=== Testing Improved Health Check Output ==="
echo "Test Server: http://ip.knws.co.uk"
echo "Date: $(date)"
echo

# Check if proxyrouter is running
if ! pgrep -f "proxyrouter" > /dev/null; then
    echo "❌ ProxyRouter is not running. Please start it first:"
    echo "   ./proxyrouter"
    exit 1
fi

echo "✅ ProxyRouter is running"

# API base URL
API_BASE="http://localhost:8081/api/v1"

# Test the test server first
echo "🌐 Testing test server: http://ip.knws.co.uk"
test_response=$(curl -s -m 5 http://ip.knws.co.uk)
if [ $? -eq 0 ] && [ -n "$test_response" ]; then
    echo "✅ Test server responding: $test_response"
else
    echo "❌ Test server not responding"
    exit 1
fi

echo

# Add some test proxies to get a good mix
echo "📤 Adding test proxies for demonstration..."
cat > /tmp/test_proxies_demo.json << 'EOF'
{
  "proxies": [
    "127.0.0.1:8080",
    "127.0.0.1:8081", 
    "127.0.0.1:8082",
    "127.0.0.1:8083",
    "127.0.0.1:8084",
    "8.8.8.8:80",
    "1.1.1.1:80",
    "208.67.222.222:80"
  ]
}
EOF

response=$(curl -s -X POST "$API_BASE/proxies/import" \
    -H "Content-Type: application/json" \
    -d @/tmp/test_proxies_demo.json)

echo "Add proxies response: $response"

echo
echo "🏥 Triggering health check to see improved output..."
echo "Watch the logs for detailed output with progress bar and verbose error messages:"
echo

# Trigger health check
response=$(curl -s -X POST "$API_BASE/proxies/health-check")
echo "Health check response: $response"

echo
echo "📊 Checking results..."
sleep 5

# Get some proxy results
response=$(curl -s "$API_BASE/proxies?limit=10")
echo "Sample proxies:"
echo "$response" | grep -E '"id"|"host"|"port"|"alive"' | head -20

echo
echo "✅ Test completed! Check the ProxyRouter logs for the detailed output:"
echo "   tail -f proxyrouter.log"
