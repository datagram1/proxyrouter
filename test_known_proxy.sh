#!/bin/bash

# Test script for known good proxy with new test server
# Proxy: 89.46.249.253:9876

echo "=== Testing Known Good Proxy ==="
echo "Proxy: 89.46.249.253:9876"
echo "Test Server: http://ip.knws.co.uk"
echo "Date: $(date)"
echo

# Test direct connection first
echo "🌐 Testing direct connection to test server..."
direct_response=$(curl -s -m 5 http://ip.knws.co.uk)
if [ $? -eq 0 ] && [ -n "$direct_response" ]; then
    echo "✅ Direct connection works: $direct_response"
else
    echo "❌ Direct connection failed"
    exit 1
fi

echo

# Test proxy connection
echo "🔗 Testing proxy connection..."
proxy_response=$(curl -s --proxy socks5://89.46.249.253:9876 --connect-timeout 10 http://ip.knws.co.uk)
if [ $? -eq 0 ] && [ -n "$proxy_response" ]; then
    echo "✅ Proxy connection works: $proxy_response"
    echo "🎉 SUCCESS: Known good proxy is working with new test server!"
else
    echo "❌ Proxy connection failed"
    echo "This might indicate the proxy is no longer working or the test server is not accessible via this proxy"
fi

echo
echo "=== Test Complete ==="
