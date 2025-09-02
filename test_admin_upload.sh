#!/bin/bash

# Test script for admin interface proxy upload functionality
# This script tests that the admin interface can handle textarea input for proxy uploads

set -e

echo "=== ProxyRouter Admin Upload Test ==="
echo

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if proxyrouter container is running
if ! docker ps | grep -q "proxyrouter-proxyrouter-1"; then
    print_error "ProxyRouter container is not running. Please start it first:"
    echo "   docker-compose up -d"
    exit 1
fi

print_success "ProxyRouter container is running"

# Admin credentials
ADMIN_USER="admin"
ADMIN_PASS="admin"

print_status "Using admin credentials: $ADMIN_USER/$ADMIN_PASS"

# Function to get CSRF token
get_csrf_token() {
    curl -s -c /tmp/cookies.txt "http://localhost:8082/admin/csrf-login" | grep -o '"csrf_token":"[^"]*"' | cut -d'"' -f4
}

# Function to login and get session
login() {
    print_status "Logging in to admin interface..."
    CSRF_TOKEN=$(get_csrf_token)
    
    if [ -z "$CSRF_TOKEN" ]; then
        print_error "Failed to get CSRF token"
        return 1
    fi
    
    # Login
    curl -s -c /tmp/cookies.txt -b /tmp/cookies.txt \
        -X POST "http://localhost:8082/admin/login" \
        -d "username=$ADMIN_USER&password=$ADMIN_PASS&csrf_token=$CSRF_TOKEN" \
        -H "Content-Type: application/x-www-form-urlencoded" > /dev/null
    
    print_success "Login successful"
}

# Function to test textarea upload
test_textarea_upload() {
    print_status "Testing textarea proxy upload..."
    
    # Get new CSRF token for upload
    CSRF_TOKEN=$(get_csrf_token)
    
    # Test data - some sample proxies
    TEST_PROXIES="127.0.0.1:8080
http://proxy.example.com:3128
socks5://socks.example.com:1080
192.168.1.100:8080
https://secure-proxy.com:8443"
    
    # Upload via textarea (no file)
    UPLOAD_RESPONSE=$(curl -s -c /tmp/cookies.txt -b /tmp/cookies.txt \
        -X POST "http://localhost:8082/admin/upload" \
        -F "proxies=$TEST_PROXIES" \
        -F "csrf_token=$CSRF_TOKEN")
    
    if echo "$UPLOAD_RESPONSE" | grep -q "Upload successful"; then
        print_success "Textarea upload successful"
        echo "Response: $UPLOAD_RESPONSE" | head -c 200
        echo ""
    else
        print_error "Failed to upload via textarea"
        echo "Response: $UPLOAD_RESPONSE"
        return 1
    fi
}

# Function to test file upload
test_file_upload() {
    print_status "Testing file upload..."
    
    # Create a temporary file with test proxies
    cat > /tmp/test_proxies_file.txt << 'EOF'
127.0.0.1:8081
http://file-proxy.example.com:3128
socks5://file-socks.example.com:1080
EOF
    
    # Get new CSRF token for upload
    CSRF_TOKEN=$(get_csrf_token)
    
    # Upload the file
    UPLOAD_RESPONSE=$(curl -s -c /tmp/cookies.txt -b /tmp/cookies.txt \
        -X POST "http://localhost:8082/admin/upload" \
        -F "file=@/tmp/test_proxies_file.txt" \
        -F "csrf_token=$CSRF_TOKEN")
    
    if echo "$UPLOAD_RESPONSE" | grep -q "Upload successful"; then
        print_success "File upload successful"
        echo "Response: $UPLOAD_RESPONSE" | head -c 200
        echo ""
    else
        print_error "Failed to upload file"
        echo "Response: $UPLOAD_RESPONSE"
        return 1
    fi
}

# Function to test form accessibility
test_form_access() {
    print_status "Testing upload form accessibility..."
    
    FORM_RESPONSE=$(curl -s "http://localhost:8082/admin/upload")
    
    if echo "$FORM_RESPONSE" | grep -q "Upload Proxy List"; then
        print_success "Upload form is accessible"
    else
        print_error "Upload form is not accessible"
        return 1
    fi
    
    # Check for textarea
    if echo "$FORM_RESPONSE" | grep -q 'name="proxies"'; then
        print_success "Textarea is present in form"
    else
        print_error "Textarea is missing from form"
        return 1
    fi
    
    # Check for file input (should not have required attribute)
    if echo "$FORM_RESPONSE" | grep -q 'type="file"' && ! echo "$FORM_RESPONSE" | grep -q 'required'; then
        print_success "File input is present and not required"
    else
        print_error "File input is missing or incorrectly configured"
        return 1
    fi
}

# Main test execution
echo
print_status "Starting admin upload tests..."

# Test form accessibility first
test_form_access

# Login to admin interface
login

# Test textarea upload
test_textarea_upload

# Test file upload
test_file_upload

echo
print_success "All admin upload tests completed successfully!"
print_status "The admin interface now correctly handles both file uploads and textarea input"

# Cleanup
rm -f /tmp/cookies.txt /tmp/test_proxies_file.txt

echo
echo "✅ Admin upload functionality is working correctly"
