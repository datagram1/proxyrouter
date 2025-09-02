#!/bin/bash

# Test script for 10-second timeout optimization
# This script demonstrates the improved timeout behavior

set -e

echo "=== Testing 10-Second Timeout Optimization ==="
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

# Add some test proxies to demonstrate timeout behavior
echo "📤 Adding test proxies for timeout demonstration..."
cat > /tmp/test_proxies_timeout.json << 'EOF'
{
  "proxies": [
    "socks5://127.0.0.1:1080",
    "http://127.0.0.1:8080",
    "socks5://192.168.1.100:1080",
    "http://192.168.1.100:8080",
    "socks5://10.0.0.1:1080",
    "http://10.0.0.1:8080",
    "socks5://172.16.0.1:1080",
    "http://172.16.0.1:8080"
  ]
}
EOF

response=$(curl -s -X POST "$API_BASE/proxies/import" \
    -H "Content-Type: application/json" \
    -d @/tmp/test_proxies_timeout.json)

echo "Add proxies response: $response"

echo
echo "🏥 Triggering health check with 10-second timeout..."
echo "Watch the logs for the improved timeout behavior:"
echo "- Timeout messages now show 'connect timeout=10'"
echo "- Faster failure detection for slow proxies"
echo "- More efficient health check process"
echo

# Record start time
start_time=$(date +%s)

# Trigger health check
response=$(curl -s -X POST "$API_BASE/proxies/health-check")
echo "Health check response: $response"

# Record end time
end_time=$(date +%s)
duration=$((end_time - start_time))

echo
echo "📊 Health check completed in ${duration} seconds"
echo "📋 Expected Improvements:"
echo "1. ✅ Faster timeout: 10 seconds instead of 12 seconds"
echo "2. ✅ Quicker failure detection for slow proxies"
echo "3. ✅ More efficient overall health check process"
echo "4. ✅ Better resource utilization"
echo "5. ✅ Reduced waiting time for non-responsive proxies"

echo
echo "📋 Timeout Behavior:"
echo "- Fast proxies (< 10s): ✅ Work normally"
echo "- Slow proxies (> 10s): ❌ Fail quickly with 'connect timeout=10'"
echo "- Non-responsive proxies: ❌ Fail after exactly 10 seconds"

echo
echo "✅ Test completed! Check the ProxyRouter logs for the improved timeout output:"
echo "   tail -f proxyrouter.log"
echo
echo "The 10-second timeout should make health checks much more efficient!"
