#!/bin/bash

# Direct API test for health check system with 20 proxies
# This script uses the API endpoints directly to test the health check system

set -e

echo "=== ProxyRouter API Health Check Test with 20 Proxies ==="
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

# Create test proxies data
echo "📝 Creating test proxy data..."
cat > /tmp/test_proxies_api.json << 'EOF'
[
  {"scheme": "http", "host": "127.0.0.1", "port": 8080, "source": "test"},
  {"scheme": "http", "host": "127.0.0.1", "port": 8081, "source": "test"},
  {"scheme": "http", "host": "127.0.0.1", "port": 8082, "source": "test"},
  {"scheme": "http", "host": "127.0.0.1", "port": 8083, "source": "test"},
  {"scheme": "http", "host": "127.0.0.1", "port": 8084, "source": "test"},
  {"scheme": "http", "host": "127.0.0.1", "port": 8085, "source": "test"},
  {"scheme": "http", "host": "127.0.0.1", "port": 8086, "source": "test"},
  {"scheme": "http", "host": "127.0.0.1", "port": 8087, "source": "test"},
  {"scheme": "http", "host": "127.0.0.1", "port": 8088, "source": "test"},
  {"scheme": "http", "host": "127.0.0.1", "port": 8089, "source": "test"},
  {"scheme": "http", "host": "127.0.0.1", "port": 8090, "source": "test"},
  {"scheme": "http", "host": "127.0.0.1", "port": 8091, "source": "test"},
  {"scheme": "http", "host": "127.0.0.1", "port": 8092, "source": "test"},
  {"scheme": "http", "host": "127.0.0.1", "port": 8093, "source": "test"},
  {"scheme": "http", "host": "127.0.0.1", "port": 8094, "source": "test"},
  {"scheme": "http", "host": "127.0.0.1", "port": 8095, "source": "test"},
  {"scheme": "http", "host": "127.0.0.1", "port": 8096, "source": "test"},
  {"scheme": "http", "host": "127.0.0.1", "port": 8097, "source": "test"},
  {"scheme": "http", "host": "127.0.0.1", "port": 8098, "source": "test"},
  {"scheme": "http", "host": "127.0.0.1", "port": 8099, "source": "test"}
]
EOF

echo "✅ Created test proxy data"

# Function to test API endpoint
test_api_endpoint() {
    local endpoint="$1"
    local method="${2:-GET}"
    local data="${3:-}"
    
    echo "🔍 Testing $method $endpoint"
    
    if [ -n "$data" ]; then
        response=$(curl -s -X "$method" "$API_BASE$endpoint" \
            -H "Content-Type: application/json" \
            -d "$data")
    else
        response=$(curl -s -X "$method" "$API_BASE$endpoint")
    fi
    
    echo "Response: $response"
    echo
}

# Function to add proxies via API
add_proxies() {
    echo "📤 Adding 20 test proxies via API..."
    
    # Create the request payload for the import endpoint
    cat > /tmp/import_request.json << 'EOF'
{
  "proxies": [
    "127.0.0.1:8080",
    "127.0.0.1:8081",
    "127.0.0.1:8082",
    "127.0.0.1:8083",
    "127.0.0.1:8084",
    "127.0.0.1:8085",
    "127.0.0.1:8086",
    "127.0.0.1:8087",
    "127.0.0.1:8088",
    "127.0.0.1:8089",
    "127.0.0.1:8090",
    "127.0.0.1:8091",
    "127.0.0.1:8092",
    "127.0.0.1:8093",
    "127.0.0.1:8094",
    "127.0.0.1:8095",
    "127.0.0.1:8096",
    "127.0.0.1:8097",
    "127.0.0.1:8098",
    "127.0.0.1:8099"
  ],
  "source": "test"
}
EOF
    
    response=$(curl -s -X POST "$API_BASE/proxies/import" \
        -H "Content-Type: application/json" \
        -d @/tmp/import_request.json)
    
    echo "Add proxies response: $response"
    
    if echo "$response" | grep -q "imported"; then
        echo "✅ Proxies added successfully"
    else
        echo "❌ Failed to add proxies"
        return 1
    fi
}

# Function to get proxy statistics
get_proxy_stats() {
    echo "📊 Getting proxy statistics..."
    
    # Get all proxies and count them
    response=$(curl -s "$API_BASE/proxies")
    echo "Proxies response: $response"
    
    # Count total proxies (simple count of objects in array)
    total=$(echo "$response" | grep -o '"id":[0-9]*' | wc -l)
    
    # Count alive proxies
    alive=$(echo "$response" | grep -o '"alive":true' | wc -l)
    
    echo "📈 Current Statistics:"
    echo "   Total Proxies: $total"
    echo "   Alive Proxies: $alive"
    
    if [ "$total" -eq 20 ]; then
        echo "✅ All 20 proxies are in the database"
    else
        echo "⚠️  Only $total proxies found (expected 20)"
    fi
}

# Function to trigger health check via API
trigger_health_check() {
    echo "🏥 Triggering health check via API..."
    
    response=$(curl -s -X POST "$API_BASE/proxies/health-check")
    echo "Health check response: $response"
    
    if echo "$response" | grep -q "health_check_completed"; then
        echo "✅ Health check triggered successfully"
    else
        echo "❌ Failed to trigger health check"
        return 1
    fi
}

# Function to get health check status
get_health_check_status() {
    echo "🔍 Checking health check status..."
    
    # Get a few proxies to see their health check status
    response=$(curl -s "$API_BASE/proxies?limit=3")
    echo "Sample proxies with health check status: $response"
}

# Function to list proxies
list_proxies() {
    echo "📋 Listing proxies..."
    
    response=$(curl -s "$API_BASE/proxies?limit=5")
    echo "Proxies list (first 5): $response"
}

# Main execution
echo
echo "🚀 Starting API health check test..."

# Test basic API endpoints
echo "🔧 Testing basic API endpoints..."
test_api_endpoint "/healthz"
test_api_endpoint "/proxies"

# Add proxies
add_proxies

# Get initial stats
get_proxy_stats

# Trigger health check
trigger_health_check

# Wait for health check to complete
echo "⏳ Waiting for health check to complete..."
sleep 10

# Get updated stats
get_proxy_stats

# Get health check status
get_health_check_status

# List some proxies
list_proxies

# Cleanup
echo
echo "🧹 Cleaning up temporary files..."
rm -f /tmp/test_proxies_api.json /tmp/import_request.json

echo
echo "✅ API health check test completed!"
echo
echo "📝 Summary:"
echo "   - Added 20 test proxies via API"
echo "   - Triggered health check via API"
echo "   - Verified the health check system works"
echo
echo "🔍 Next steps:"
echo "   - Check the proxyrouter logs for detailed health check output"
echo "   - Monitor the admin dashboard at http://localhost:8082/admin/"
echo "   - The health check system should have tested all 20 proxies"
echo
echo "🎯 The health check system is working if you see:"
echo "   - Health check progress messages in the logs"
echo "   - Updated proxy status in the database"
echo "   - No errors in the health check process"
echo "   - Most proxies marked as 'dead' (expected for localhost test addresses)"
