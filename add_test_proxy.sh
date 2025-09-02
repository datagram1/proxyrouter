#!/bin/bash

# Script to add the known good SOCKS5 proxy to the database
# Proxy: 89.46.249.253:9876

set -e

echo "=== Adding Known Good SOCKS5 Proxy ==="
echo "Proxy: 89.46.249.253:9876"
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

# Test the new test server first
echo "🌐 Testing new test server: http://ip.knws.co.uk"
test_response=$(curl -s -m 5 http://ip.knws.co.uk)
if [ $? -eq 0 ] && [ -n "$test_response" ]; then
    echo "✅ Test server responding: $test_response"
else
    echo "❌ Test server not responding"
    exit 1
fi

# Add the known good proxy
echo "📤 Adding known good SOCKS5 proxy to database..."

# Create the import request
cat > /tmp/test_proxy_import.json << 'EOF'
{
  "proxies": [
    "89.46.249.253:9876"
  ]
}
EOF

# Import the proxy
response=$(curl -s -X POST "$API_BASE/proxies/import" \
    -H "Content-Type: application/json" \
    -d @/tmp/test_proxy_import.json)

echo "Import response: $response"

if echo "$response" | grep -q "imported"; then
    echo "✅ Proxy added successfully"
else
    echo "❌ Failed to add proxy"
    echo "Response: $response"
    exit 1
fi

# Verify the proxy was added
echo "🔍 Verifying proxy was added..."
verify_response=$(curl -s "$API_BASE/proxies?limit=5" | grep -A 10 -B 5 "89.46.249.253")
if [ -n "$verify_response" ]; then
    echo "✅ Proxy found in database"
    echo "$verify_response"
else
    echo "❌ Proxy not found in database"
fi

# Cleanup
rm -f /tmp/test_proxy_import.json

echo
echo "✅ Known good proxy added successfully!"
echo "🚀 Ready to test with the new proxy"
