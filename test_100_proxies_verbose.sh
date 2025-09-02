#!/bin/bash

# Verbose test script for first 100 proxies
# This script will test the first 100 proxies with detailed output

set -e

echo "=== ProxyRouter Verbose Test - First 100 Proxies ==="
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

# Function to get first 100 proxies
get_first_100_proxies() {
    echo "📋 Fetching first 100 proxies from database..."
    
    response=$(curl -s "$API_BASE/proxies?limit=100")
    
    if [ $? -ne 0 ]; then
        echo "❌ Failed to fetch proxies"
        return 1
    fi
    
    # Count proxies in response
    proxy_count=$(echo "$response" | grep -o '"id":[0-9]*' | wc -l)
    echo "📊 Found $proxy_count proxies in database"
    
    if [ "$proxy_count" -eq 0 ]; then
        echo "❌ No proxies found in database"
        return 1
    fi
    
    return 0
}

# Function to trigger health check with verbose output
trigger_verbose_health_check() {
    echo "🏥 Triggering verbose health check on first 100 proxies..."
    echo "⏱️  This may take several minutes..."
    echo
    
    # Start the health check
    response=$(curl -s -X POST "$API_BASE/proxies/health-check")
    
    if echo "$response" | grep -q "health_check_completed"; then
        echo "✅ Health check triggered successfully"
    else
        echo "❌ Failed to trigger health check"
        echo "Response: $response"
        return 1
    fi
}

# Function to monitor health check progress
monitor_progress() {
    echo "📈 Monitoring health check progress..."
    echo "Press Ctrl+C to stop monitoring"
    echo
    
    while true; do
        # Get current stats
        stats_response=$(curl -s "$API_BASE/proxies")
        
        # Extract alive and total counts
        alive_count=$(echo "$stats_response" | grep -o '"alive":true' | wc -l)
        total_count=$(echo "$stats_response" | grep -o '"id":[0-9]*' | wc -l)
        checked_count=$(echo "$stats_response" | grep -o '"last_checked_at"' | wc -l)
        
        echo "[$(date '+%H:%M:%S')] Progress: $checked_count/$total_count checked, $alive_count alive"
        
        # Check if health check is still running
        if ! pgrep -f "proxyrouter" > /dev/null; then
            echo "❌ ProxyRouter process stopped"
            break
        fi
        
        sleep 10
    done
}

# Function to show detailed results
show_detailed_results() {
    echo
    echo "📊 Detailed Results Summary"
    echo "=========================="
    
    # Get all proxies with health check status
    response=$(curl -s "$API_BASE/proxies?limit=100")
    
    # Count different statuses
    total_proxies=$(echo "$response" | grep -o '"id":[0-9]*' | wc -l)
    alive_proxies=$(echo "$response" | grep -o '"alive":true' | wc -l)
    dead_proxies=$(echo "$response" | grep -o '"alive":false' | wc -l)
    checked_proxies=$(echo "$response" | grep -o '"last_checked_at"' | wc -l)
    
    echo "📈 Statistics:"
    echo "   Total Proxies: $total_proxies"
    echo "   Alive Proxies: $alive_proxies"
    echo "   Dead Proxies: $dead_proxies"
    echo "   Checked Proxies: $checked_proxies"
    echo "   Success Rate: $(echo "scale=2; $alive_proxies * 100 / $total_proxies" | bc)%"
    
    echo
    echo "🏆 Top 10 Alive Proxies (by latency):"
    echo "$response" | grep -A 5 -B 5 '"alive":true' | grep -E '"id"|"latency_ms"|"host"' | head -30
    
    echo
    echo "❌ Common Error Types:"
    echo "$response" | grep -o '"error_message":"[^"]*"' | sort | uniq -c | sort -nr | head -10
    
    echo
    echo "🔍 Sample Alive Proxies:"
    echo "$response" | grep -A 10 '"alive":true' | head -20
}

# Function to show test server status
test_server_status() {
    echo "🌐 Testing server status..."
    
    # Test direct connection
    direct_response=$(curl -s -m 5 http://ip.knws.co.uk)
    if [ $? -eq 0 ] && [ -n "$direct_response" ]; then
        echo "✅ Test server responding: $direct_response"
    else
        echo "❌ Test server not responding"
        return 1
    fi
    
    # Test with a simple proxy (if available)
    echo "🔍 Testing with sample proxy..."
    sample_proxy=$(echo "$response" | grep -o '"host":"[^"]*"' | head -1 | cut -d'"' -f4)
    if [ -n "$sample_proxy" ]; then
        echo "   Sample proxy: $sample_proxy"
    fi
}

# Main execution
echo "🚀 Starting verbose test of first 100 proxies..."
echo

# Step 1: Get proxy count
get_first_100_proxies

# Step 2: Test server status
test_server_status

# Step 3: Trigger health check
trigger_verbose_health_check

# Step 4: Monitor progress (run in background)
monitor_progress &
monitor_pid=$!

# Wait for health check to complete (or timeout after 10 minutes)
echo "⏳ Waiting for health check to complete..."
timeout 600 bash -c 'while pgrep -f "proxyrouter" > /dev/null; do sleep 5; done'

# Stop monitoring
kill $monitor_pid 2>/dev/null || true

# Step 5: Show detailed results
show_detailed_results

echo
echo "✅ Verbose test completed!"
echo "📝 Check proxyrouter.log for detailed health check output"
echo "🌐 Admin dashboard: http://localhost:8082/admin/"
