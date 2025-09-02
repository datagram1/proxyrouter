#!/bin/bash

# Test script for optimized health check behavior
# This script demonstrates the new logic:
# 1. Try current scheme first
# 2. If it works, stop testing (no need to test other protocols)
# 3. If it fails, only then try alternative protocol

set -e

echo "=== Testing Optimized Health Check Behavior ==="
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

# Add some test proxies with different schemes
echo "📤 Adding test proxies for optimization demonstration..."
cat > /tmp/test_proxies_optimized.json << 'EOF'
{
  "proxies": [
    "socks5://127.0.0.1:1080",
    "http://127.0.0.1:8080",
    "socks5://192.168.1.100:1080",
    "http://192.168.1.100:8080",
    "socks5://10.0.0.1:1080",
    "http://10.0.0.1:8080"
  ]
}
EOF

response=$(curl -s -X POST "$API_BASE/proxies/import" \
    -H "Content-Type: application/json" \
    -d @/tmp/test_proxies_optimized.json)

echo "Add proxies response: $response"

echo
echo "🏥 Triggering health check to see optimized behavior..."
echo "Watch the logs for the new behavior:"
echo "- If SOCKS5 works, it won't test HTTP"
echo "- If SOCKS5 fails, it will only then test HTTP"
echo "- No redundant protocol testing"
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
echo "$response" | grep -E '"id"|"host"|"port"|"scheme"|"alive"' | head -20

echo
echo "📋 Expected Behavior:"
echo "1. ✅ SOCKS5 proxy that works: Only tests SOCKS5, stops there"
echo "2. ❌ SOCKS5 proxy that fails: Tests SOCKS5, then tries HTTP"
echo "3. ✅ HTTP proxy that works: Only tests HTTP, stops there"
echo "4. ❌ HTTP proxy that fails: Tests HTTP, then tries SOCKS5"
echo "5. 🚫 No redundant testing of working protocols"

echo
echo "✅ Test completed! Check the ProxyRouter logs for the optimized output:"
echo "   tail -f proxyrouter.log"
echo
echo "The optimized health check should be faster and more efficient!"
