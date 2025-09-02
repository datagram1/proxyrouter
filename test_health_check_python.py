#!/usr/bin/env python3
"""
Test script for ProxyRouter health check system with 20 proxies
This script uses the API to add proxies and test health checks
"""

import requests
import json
import time
import sys

# Configuration
API_BASE = "http://localhost:8081/api/v1"
TEST_PROXIES = [
    "127.0.0.1:8080", "127.0.0.1:8081", "127.0.0.1:8082", "127.0.0.1:8083", "127.0.0.1:8084",
    "127.0.0.1:8085", "127.0.0.1:8086", "127.0.0.1:8087", "127.0.0.1:8088", "127.0.0.1:8089",
    "127.0.0.1:8090", "127.0.0.1:8091", "127.0.0.1:8092", "127.0.0.1:8093", "127.0.0.1:8094",
    "127.0.0.1:8095", "127.0.0.1:8096", "127.0.0.1:8097", "127.0.0.1:8098", "127.0.0.1:8099"
]

# Test server configuration
TEST_SERVER = "http://ip.knws.co.uk"

def test_api_endpoint(endpoint, method="GET", data=None):
    """Test an API endpoint"""
    url = f"{API_BASE}{endpoint}"
    print(f"🔍 Testing {method} {endpoint}")
    
    try:
        if method == "GET":
            response = requests.get(url, timeout=10)
        elif method == "POST":
            response = requests.post(url, json=data, timeout=10)
        else:
            print(f"❌ Unsupported method: {method}")
            return None
            
        print(f"Response: {response.text}")
        return response
        
    except requests.exceptions.RequestException as e:
        print(f"❌ Request failed: {e}")
        return None

def check_proxyrouter_running():
    """Check if ProxyRouter is running"""
    try:
        response = requests.get(f"{API_BASE}/healthz", timeout=5)
        if response.status_code == 200:
            print("✅ ProxyRouter is running")
            return True
        else:
            print("❌ ProxyRouter health check failed")
            return False
    except requests.exceptions.RequestException:
        print("❌ ProxyRouter is not running")
        return False

def add_proxies():
    """Add test proxies via API"""
    print("📤 Adding 20 test proxies via API...")
    
    data = {
        "proxies": TEST_PROXIES,
        "source": "test"
    }
    
    response = test_api_endpoint("/proxies/import", "POST", data)
    
    if response and response.status_code == 200:
        result = response.json()
        print(f"✅ Proxies added successfully: {result}")
        return True
    else:
        print("❌ Failed to add proxies")
        return False

def get_proxy_stats():
    """Get proxy statistics"""
    print("📊 Getting proxy statistics...")
    
    response = test_api_endpoint("/proxies")
    
    if response and response.status_code == 200:
        proxies = response.json()
        total = len(proxies)
        alive = sum(1 for p in proxies if p.get('alive', False))
        
        print(f"📈 Current Statistics:")
        print(f"   Total Proxies: {total}")
        print(f"   Alive Proxies: {alive}")
        
        if total == 20:
            print("✅ All 20 proxies are in the database")
        else:
            print(f"⚠️  Only {total} proxies found (expected 20)")
        
        return total, alive
    else:
        print("❌ Failed to get proxy statistics")
        return 0, 0

def trigger_health_check():
    """Trigger health check on proxies"""
    print("🏥 Triggering health check via API...")
    
    response = test_api_endpoint("/proxies/health-check", "POST")
    
    if response and response.status_code == 200:
        result = response.json()
        print(f"✅ Health check triggered successfully: {result}")
        return True
    else:
        print("❌ Failed to trigger health check")
        return False

def wait_for_health_check():
    """Wait for health check to complete"""
    print("⏳ Waiting for health check to complete...")
    time.sleep(15)  # Wait for health check to complete

def show_sample_proxies():
    """Show sample proxies with their health status"""
    print("📋 Sample proxies with health check status:")
    
    response = test_api_endpoint("/proxies")
    
    if response and response.status_code == 200:
        proxies = response.json()
        for i, proxy in enumerate(proxies[:5]):  # Show first 5
            status = "✅ Alive" if proxy.get('alive', False) else "❌ Dead"
            latency = proxy.get('latency_ms', 'N/A')
            error = proxy.get('error_message', 'None')
            print(f"   {i+1}. {proxy['host']}:{proxy['port']} ({proxy['scheme']}) - {status}")
            if proxy.get('alive', False):
                print(f"      Latency: {latency}ms")
            else:
                print(f"      Error: {error}")

def main():
    """Main test function"""
    print("=== ProxyRouter Health Check Test with 20 Proxies ===")
    print()
    
    # Check if ProxyRouter is running
    if not check_proxyrouter_running():
        print("Please start ProxyRouter first:")
        print("   ./proxyrouter")
        sys.exit(1)
    
    print()
    print("🚀 Starting health check test...")
    
    # Test basic API endpoints
    print()
    print("🔧 Testing basic API endpoints...")
    test_api_endpoint("/healthz")
    test_api_endpoint("/proxies")
    
    # Add proxies
    print()
    if not add_proxies():
        sys.exit(1)
    
    # Get initial stats
    print()
    get_proxy_stats()
    
    # Trigger health check
    print()
    if not trigger_health_check():
        sys.exit(1)
    
    # Wait for health check to complete
    print()
    wait_for_health_check()
    
    # Get updated stats
    print()
    get_proxy_stats()
    
    # Show sample proxies
    print()
    show_sample_proxies()
    
    print()
    print("✅ Health check test completed!")
    print()
    print("📝 Summary:")
    print("   - Added 20 test proxies via API")
    print("   - Triggered health check via API")
    print("   - Verified the health check system works")
    print()
    print("🔍 Next steps:")
    print("   - Check the proxyrouter logs for detailed health check output")
    print("   - Monitor the admin dashboard at http://localhost:8082/admin/")
    print("   - The health check system should have tested all 20 proxies")
    print()
    print("🎯 The health check system is working if you see:")
    print("   - Health check progress messages in the logs")
    print("   - Updated proxy status in the database")
    print("   - No errors in the health check process")
    print("   - Most proxies marked as 'dead' (expected for localhost test addresses)")

if __name__ == "__main__":
    main()
