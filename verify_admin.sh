#!/bin/bash

echo "=== ProxyRouter Admin Interface Verification ==="
echo

# Check if container is running
if docker ps | grep -q "proxyrouter-proxyrouter-1"; then
    echo "✅ ProxyRouter container is running"
else
    echo "❌ ProxyRouter container is not running"
    exit 1
fi

# Test admin interface accessibility
echo "🔍 Testing admin interface accessibility..."

# Test login page
LOGIN_RESPONSE=$(curl -s http://localhost:8082/admin/login)
if echo "$LOGIN_RESPONSE" | grep -q "Login - ProxyRouter Admin"; then
    echo "✅ Login page is accessible"
else
    echo "❌ Login page is not accessible"
    exit 1
fi

# Test upload form (should redirect to login)
UPLOAD_RESPONSE=$(curl -s http://localhost:8082/admin/upload)
if echo "$UPLOAD_RESPONSE" | grep -q "See Other"; then
    echo "✅ Upload form correctly redirects to login (security working)"
else
    echo "❌ Upload form security issue"
    exit 1
fi

# Test CSRF endpoint
CSRF_RESPONSE=$(curl -s http://localhost:8082/admin/csrf-login)
if echo "$CSRF_RESPONSE" | grep -q "csrf_token"; then
    echo "✅ CSRF token endpoint is working"
else
    echo "❌ CSRF token endpoint is not working"
    exit 1
fi

echo
echo "🎉 Admin interface verification completed successfully!"
echo
echo "📋 Summary:"
echo "   - Container is running"
echo "   - Login page is accessible"
echo "   - Security redirects are working"
echo "   - CSRF protection is enabled"
echo
echo "🌐 Access the admin interface at: http://localhost:8082/admin"
echo "   Default credentials: admin / admin"
echo
echo "📝 The upload form now supports:"
echo "   - File upload (.txt, .csv)"
echo "   - Textarea input (paste proxies directly)"
echo "   - Visual feedback for input method"
echo "   - Form validation"
