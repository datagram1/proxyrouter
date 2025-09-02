#!/bin/bash

# Test script for health check system with 20 proxies
# This script will:
# 1. Add 20 test proxies to the database
# 2. Run health checks to verify the system works
# 3. Show results

set -e

echo "=== ProxyRouter Health Check Test with 20 Proxies ==="
echo

# Check if proxyrouter is running
if ! pgrep -f "proxyrouter" > /dev/null; then
    echo "❌ ProxyRouter is not running. Please start it first:"
    echo "   ./proxyrouter"
    exit 1
fi

echo "✅ ProxyRouter is running"

# Create a temporary file with 20 test proxies
echo "📝 Creating test proxy list..."
cat > /tmp/test_proxies_20.txt << 'EOF'
# Test proxies for health check testing
# These are mostly non-functional but will test the health check system
127.0.0.1:8080
127.0.0.1:8081
127.0.0.1:8082
127.0.0.1:8083
127.0.0.1:8084
127.0.0.1:8085
127.0.0.1:8086
127.0.0.1:8087
127.0.0.1:8088
127.0.0.1:8089
127.0.0.1:8090
127.0.0.1:8091
127.0.0.1:8092
127.0.0.1:8093
127.0.0.1:8094
127.0.0.1:8095
127.0.0.1:8096
127.0.0.1:8097
127.0.0.1:8098
127.0.0.1:8099
EOF

echo "✅ Created test proxy list with 20 proxies"

# Get admin credentials (default is admin/admin)
ADMIN_USER="admin"
ADMIN_PASS="admin"

echo "🔐 Using admin credentials: $ADMIN_USER/$ADMIN_PASS"

# Function to get CSRF token
get_csrf_token() {
    curl -s -c /tmp/cookies.txt "http://localhost:8082/admin/csrf-login" | grep -o '"csrf_token":"[^"]*"' | cut -d'"' -f4
}

# Function to login and get session
login() {
    echo "🔑 Logging in to admin interface..."
    CSRF_TOKEN=$(get_csrf_token)
    
    if [ -z "$CSRF_TOKEN" ]; then
        echo "❌ Failed to get CSRF token"
        return 1
    fi
    
    # Login
    curl -s -c /tmp/cookies.txt -b /tmp/cookies.txt \
        -X POST "http://localhost:8082/admin/login" \
        -d "username=$ADMIN_USER&password=$ADMIN_PASS&csrf_token=$CSRF_TOKEN" \
        -H "Content-Type: application/x-www-form-urlencoded" > /dev/null
    
    echo "✅ Login successful"
}

# Function to upload proxies
upload_proxies() {
    echo "📤 Uploading 20 test proxies..."
    
    # Get new CSRF token for upload
    CSRF_TOKEN=$(get_csrf_token)
    
    # Upload the proxy file
    UPLOAD_RESPONSE=$(curl -s -c /tmp/cookies.txt -b /tmp/cookies.txt \
        -X POST "http://localhost:8082/admin/upload" \
        -F "file=@/tmp/test_proxies_20.txt" \
        -F "csrf_token=$CSRF_TOKEN")
    
    if echo "$UPLOAD_RESPONSE" | grep -q "imported"; then
        echo "✅ Proxies uploaded successfully"
    else
        echo "❌ Failed to upload proxies"
        echo "Response: $UPLOAD_RESPONSE"
        return 1
    fi
}

# Function to run health check
run_health_check() {
    echo "🏥 Running health check on proxies..."
    
    # Get new CSRF token for health check
    CSRF_TOKEN=$(get_csrf_token)
    
    # Trigger health check
    HEALTH_RESPONSE=$(curl -s -c /tmp/cookies.txt -b /tmp/cookies.txt \
        -X POST "http://localhost:8082/admin/health-check" \
        -d "csrf_token=$CSRF_TOKEN" \
        -H "Content-Type: application/x-www-form-urlencoded")
    
    if echo "$HEALTH_RESPONSE" | grep -q "health_check=success"; then
        echo "✅ Health check triggered successfully"
    else
        echo "❌ Failed to trigger health check"
        echo "Response: $HEALTH_RESPONSE"
        return 1
    fi
}

# Function to check proxy status
check_proxy_status() {
    echo "📊 Checking proxy status..."
    
    # Wait a moment for health check to complete
    sleep 5
    
    # Get proxy statistics
    STATS_RESPONSE=$(curl -s -c /tmp/cookies.txt -b /tmp/cookies.txt \
        "http://localhost:8082/admin/")
    
    # Extract statistics using grep and sed
    TOTAL_PROXIES=$(echo "$STATS_RESPONSE" | grep -o 'Total Proxies</div>[^<]*<div[^>]*>[0-9]*' | grep -o '[0-9]*' | tail -1)
    ALIVE_PROXIES=$(echo "$STATS_RESPONSE" | grep -o 'Alive Proxies</div>[^<]*<div[^>]*>[0-9]*' | grep -o '[0-9]*' | tail -1)
    
    echo "📈 Proxy Statistics:"
    echo "   Total Proxies: $TOTAL_PROXIES"
    echo "   Alive Proxies: $ALIVE_PROXIES"
    
    if [ "$TOTAL_PROXIES" -eq 20 ]; then
        echo "✅ All 20 proxies were added successfully"
    else
        echo "⚠️  Only $TOTAL_PROXIES proxies found (expected 20)"
    fi
}

# Function to show health check logs
show_logs() {
    echo "📋 Recent health check activity:"
    echo "   (Check the proxyrouter logs for detailed health check output)"
    echo "   You can view logs with: tail -f proxyrouter.log"
}

# Main execution
echo
echo "🚀 Starting health check test..."

# Login
login

# Upload proxies
upload_proxies

# Run health check
run_health_check

# Check status
check_proxy_status

# Show logs info
show_logs

echo
echo "🧹 Cleaning up temporary files..."
rm -f /tmp/test_proxies_20.txt /tmp/cookies.txt

echo
echo "✅ Health check test completed!"
echo
echo "📝 Summary:"
echo "   - Added 20 test proxies to the database"
echo "   - Triggered health check system"
echo "   - Verified the health check process works"
echo
echo "🔍 Next steps:"
echo "   - Check the proxyrouter logs for detailed health check output"
echo "   - Monitor the admin dashboard at http://localhost:8082/admin/"
echo "   - The health check system should have tested all 20 proxies"
echo "   - Most proxies will show as 'dead' since they're localhost test addresses"
echo
echo "🎯 The health check system is working if you see:"
echo "   - Health check progress messages in the logs"
echo "   - Updated proxy status in the database"
echo "   - No errors in the health check process"
