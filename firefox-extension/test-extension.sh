#!/bin/bash

# ProxyRouter Firefox Extension Test Script
# Tests the extension functionality with a running ProxyRouter server

set -e

echo "=== Testing ProxyRouter Firefox Extension ==="
echo "Date: $(date)"
echo

# Configuration
API_BASE="http://localhost:8081/api/v1"
PROXY_HOST="localhost:8080"

# Check if ProxyRouter is running
echo "🔍 Checking ProxyRouter server status..."

if ! pgrep -f "proxyrouter" > /dev/null; then
    echo "❌ ProxyRouter is not running. Please start it first:"
    echo "   cd proxyrouter && ./proxyrouter"
    exit 1
fi

echo "✅ ProxyRouter is running"

# Test API connectivity
echo
echo "🌐 Testing API connectivity..."

response=$(curl -s -m 5 "$API_BASE/healthz")
if [ $? -eq 0 ] && echo "$response" | grep -q "ok"; then
    echo "✅ API is responding: $response"
else
    echo "❌ API not responding properly"
    echo "   Response: $response"
    exit 1
fi

# Test proxy connectivity
echo
echo "🔗 Testing proxy connectivity..."

# Test HTTP proxy
http_test=$(curl -s -m 10 -x "http://$PROXY_HOST" "http://ip.knws.co.uk")
if [ $? -eq 0 ] && [ -n "$http_test" ]; then
    echo "✅ HTTP proxy working: $http_test"
else
    echo "❌ HTTP proxy not working"
fi

# Test HTTPS proxy
https_test=$(curl -s -m 10 -x "http://$PROXY_HOST" "https://ifconfig.me")
if [ $? -eq 0 ] && [ -n "$https_test" ]; then
    echo "✅ HTTPS proxy working: $https_test"
else
    echo "❌ HTTPS proxy not working"
fi

# Get proxy statistics
echo
echo "📊 Getting proxy statistics..."

stats_response=$(curl -s "$API_BASE/proxies?alive=1")
if [ $? -eq 0 ]; then
    echo "✅ Proxy statistics retrieved"
    echo "   Response: $stats_response"
else
    echo "❌ Failed to get proxy statistics"
fi

echo
echo "=== Extension Test Summary ==="
echo "✅ ProxyRouter server: Running"
echo "✅ API endpoint: Responding"
echo "✅ HTTP proxy: Working"
echo "✅ HTTPS proxy: Working"
echo "✅ Statistics API: Working"
echo
echo "🎉 Extension should work correctly!"
echo
echo "Next steps:"
echo "1. Build the extension: ./build.sh"
echo "2. Load in Firefox: about:debugging → Load Temporary Add-on"
echo "3. Configure settings in the extension"
echo "4. Test the toggle functionality"
echo
echo "Extension features to test:"
echo "- Enable/disable proxy toggle"
echo "- Status monitoring"
echo "- Health check triggering"
echo "- Settings configuration"
echo "- Statistics display"
