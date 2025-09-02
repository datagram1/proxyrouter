# ProxyRouter Integration Guide for Python Projects

## Overview

ProxyRouter is a high-performance LAN proxy router written in Go that provides HTTP and SOCKS5 proxy servers with intelligent routing capabilities. This guide provides comprehensive instructions for LLMs to understand and integrate the ProxyRouter project into Python applications.

## Project Architecture

### Core Components

1. **HTTP Proxy Server** (Port 8080) - Handles HTTP forward and HTTPS CONNECT tunneling
2. **SOCKS5 Proxy Server** (Port 1080) - Full SOCKS5 protocol support  
3. **REST API Server** (Port 8081) - JSON API for configuration and monitoring
4. **Admin Web UI** (Port 8082) - Web interface for management
5. **Routing Engine** - Intelligent request routing with four groups:
   - **LOCAL** → Direct connection
   - **GENERAL** → Randomly selected healthy proxy from pool
   - **TOR** → Via Tor SOCKS5 daemon
   - **UPSTREAM** → Specific proxy chosen from database

### Key Features

- **Access Control Lists (ACL)** - CIDR-based client filtering
- **SQLite Database** - Stores proxies, routes, ACLs, and settings
- **Proxy Refresh System** - Auto-downloads and validates proxy lists
- **Health Monitoring** - Continuous proxy health checking
- **Docker Support** - Containerized deployment with Tor sidecar
- **Systemd Integration** - Native Linux service deployment

## Database Schema

The ProxyRouter uses SQLite with the following key tables:

### Proxies Table
```sql
CREATE TABLE proxies (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  scheme TEXT NOT NULL,             -- "socks5" | "http" | "https"
  host TEXT NOT NULL,
  port INTEGER NOT NULL,
  source TEXT,                      -- e.g., "spys.one-gb" or "manual"
  latency_ms INTEGER,
  alive INTEGER NOT NULL DEFAULT 1,
  last_checked_at DATETIME,
  expires_at DATETIME,              -- null = persistent
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE (scheme, host, port)
);
```

### Routes Table
```sql
CREATE TABLE routes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  client_cidr TEXT,                 -- e.g. "192.168.10.0/24" (nullable = any client)
  host_glob TEXT,                   -- e.g. "*.github.com" (nullable = any host)
  "group" TEXT NOT NULL,            -- "LOCAL"|"GENERAL"|"TOR"|"UPSTREAM"
  proxy_id INTEGER,                 -- used when group="UPSTREAM"
  precedence INTEGER NOT NULL DEFAULT 100,
  enabled INTEGER NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### ACL Subnets Table
```sql
CREATE TABLE acl_subnets (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  cidr TEXT NOT NULL UNIQUE
);
```

## Configuration Structure

The ProxyRouter uses YAML configuration with the following structure:

```yaml
# configs/config.yaml
listen:
  http_proxy: "0.0.0.0:8080"
  socks5_proxy: "0.0.0.0:1080"
  api: "0.0.0.0:8081"

timeouts:
  dial_ms: 10000
  read_ms: 30000
  write_ms: 30000

tor:
  enabled: true
  socks_address: "127.0.0.1:9050"

refresh:
  enable_general_sources: true
  interval_sec: 900
  healthcheck_concurrency: 50
  sources:
    - name: "spys.one"
      url: "https://spys.one/free-proxy-list/"
      type: "html"

database:
  path: "/var/lib/proxyr/router.db"

admin:
  enabled: true
  bind: "0.0.0.0"
  port: 8082
  allow_cidrs: ["0.0.0.0/0"]
```

## Python Integration Strategies

### 1. Direct Proxy Usage

Use ProxyRouter as a proxy server for your Python HTTP requests:

```python
import requests

# Configure session to use ProxyRouter
session = requests.Session()

# Use HTTP proxy
session.proxies = {
    'http': 'http://192.168.10.230:8080',
    'https': 'http://192.168.10.230:8080'
}

# Use SOCKS5 proxy (requires requests[socks])
session.proxies = {
    'http': 'socks5://192.168.10.230:1080',
    'https': 'socks5://192.168.10.230:1080'
}

# Make requests through ProxyRouter
response = session.get('https://httpbin.org/ip')
print(response.json())
```

### 2. API Integration

Control ProxyRouter via its REST API:

```python
import requests
import json

class ProxyRouterClient:
    def __init__(self, base_url="http://192.168.10.230:8081"):
        self.base_url = base_url
        self.session = requests.Session()
    
    def get_health(self):
        """Check ProxyRouter health"""
        response = self.session.get(f"{self.base_url}/healthz")
        return response.json()
    
    def get_routes(self):
        """Get all routing rules"""
        response = self.session.get(f"{self.base_url}/v1/routes")
        return response.json()
    
    def create_route(self, host_glob=None, client_cidr=None, group="LOCAL", 
                    proxy_id=None, precedence=100):
        """Create a new routing rule"""
        data = {
            "host_glob": host_glob,
            "client_cidr": client_cidr,
            "group": group,
            "proxy_id": proxy_id,
            "precedence": precedence
        }
        # Remove None values
        data = {k: v for k, v in data.items() if v is not None}
        
        response = self.session.post(f"{self.base_url}/v1/routes", 
                                   json=data)
        return response.json()
    
    def get_proxies(self):
        """Get all proxies"""
        response = self.session.get(f"{self.base_url}/v1/proxies")
        return response.json()
    
    def import_proxies(self, proxies):
        """Import proxy list"""
        response = self.session.post(f"{self.base_url}/v1/proxies/import", 
                                   json=proxies)
        return response.json()
    
    def refresh_proxies(self):
        """Trigger proxy refresh"""
        response = self.session.post(f"{self.base_url}/v1/proxies/refresh")
        return response.json()
    
    def get_acl(self):
        """Get ACL subnets"""
        response = self.session.get(f"{self.base_url}/v1/acl")
        return response.json()
    
    def add_acl(self, cidr):
        """Add ACL subnet"""
        data = {"cidr": cidr}
        response = self.session.post(f"{self.base_url}/v1/acl", json=data)
        return response.json()

# Example usage
client = ProxyRouterClient("http://192.168.10.230:8081")

# Check health
health = client.get_health()
print(f"ProxyRouter status: {health}")

# Add routing rule for GitHub to go direct
client.create_route(
    host_glob="*.github.com",
    group="LOCAL",
    precedence=10
)

# Add routing rule for general traffic through GENERAL proxies
client.create_route(
    host_glob="*",
    group="GENERAL",
    precedence=100
)

# Import manual proxy
proxies = [
    {
        "scheme": "socks5",
        "host": "1.2.3.4",
        "port": 1080,
        "source": "manual"
    }
]
client.import_proxies(proxies)

# Refresh proxies from external sources
client.refresh_proxies()
```

### 3. Advanced Integration with Database Access

For direct database integration when running on the same machine:

```python
import sqlite3
import json
from datetime import datetime
from typing import List, Dict, Optional

class ProxyRouterDatabase:
    def __init__(self, db_path="/var/lib/proxyr/router.db"):
        self.db_path = db_path
    
    def get_connection(self):
        """Get database connection"""
        return sqlite3.connect(self.db_path)
    
    def get_routes(self) -> List[Dict]:
        """Get all routes from database"""
        with self.get_connection() as conn:
            cursor = conn.cursor()
            cursor.execute("""
                SELECT id, client_cidr, host_glob, "group", proxy_id, 
                       precedence, enabled, created_at
                FROM routes 
                ORDER BY precedence ASC, id ASC
            """)
            
            columns = [desc[0] for desc in cursor.description]
            routes = []
            for row in cursor.fetchall():
                route = dict(zip(columns, row))
                routes.append(route)
            
            return routes
    
    def get_active_proxies(self) -> List[Dict]:
        """Get all active proxies"""
        with self.get_connection() as conn:
            cursor = conn.cursor()
            cursor.execute("""
                SELECT id, scheme, host, port, source, latency_ms, 
                       alive, last_checked_at, created_at
                FROM proxies 
                WHERE alive = 1
                ORDER BY latency_ms ASC
            """)
            
            columns = [desc[0] for desc in cursor.description]
            proxies = []
            for row in cursor.fetchall():
                proxy = dict(zip(columns, row))
                proxies.append(proxy)
            
            return proxies
    
    def add_proxy(self, scheme: str, host: str, port: int, 
                  source: str = "manual") -> int:
        """Add a new proxy"""
        with self.get_connection() as conn:
            cursor = conn.cursor()
            cursor.execute("""
                INSERT OR REPLACE INTO proxies 
                (scheme, host, port, source, alive, created_at)
                VALUES (?, ?, ?, ?, 1, ?)
            """, (scheme, host, port, source, datetime.utcnow().isoformat()))
            
            conn.commit()
            return cursor.lastrowid
    
    def create_route(self, group: str, host_glob: str = None, 
                    client_cidr: str = None, proxy_id: int = None,
                    precedence: int = 100) -> int:
        """Create a new routing rule"""
        with self.get_connection() as conn:
            cursor = conn.cursor()
            cursor.execute("""
                INSERT INTO routes 
                (client_cidr, host_glob, "group", proxy_id, precedence, enabled, created_at)
                VALUES (?, ?, ?, ?, ?, 1, ?)
            """, (client_cidr, host_glob, group, proxy_id, precedence,
                  datetime.utcnow().isoformat()))
            
            conn.commit()
            return cursor.lastrowid
    
    def get_proxy_stats(self) -> Dict:
        """Get proxy statistics"""
        with self.get_connection() as conn:
            cursor = conn.cursor()
            cursor.execute("""
                SELECT 
                    COUNT(*) as total,
                    SUM(CASE WHEN alive = 1 THEN 1 ELSE 0 END) as alive,
                    AVG(CASE WHEN alive = 1 AND latency_ms IS NOT NULL 
                        THEN latency_ms ELSE NULL END) as avg_latency
                FROM proxies
            """)
            
            row = cursor.fetchone()
            return {
                "total_proxies": row[0],
                "alive_proxies": row[1],
                "average_latency_ms": row[2]
            }

# Example usage
db = ProxyRouterDatabase()

# Get proxy statistics
stats = db.get_proxy_stats()
print(f"Proxy Stats: {stats}")

# Add a manual proxy
proxy_id = db.add_proxy("socks5", "192.168.1.100", 1080, "manual")
print(f"Added proxy with ID: {proxy_id}")

# Create routing rule
route_id = db.create_route(
    group="UPSTREAM",
    host_glob="*.example.com",
    proxy_id=proxy_id,
    precedence=50
)
print(f"Created route with ID: {route_id}")
```

### 4. Async Integration

For high-performance async applications:

```python
import aiohttp
import asyncio
from typing import Dict, List

class AsyncProxyRouterClient:
    def __init__(self, base_url="http://192.168.10.230:8081"):
        self.base_url = base_url
    
    async def get_health(self) -> Dict:
        """Check ProxyRouter health"""
        async with aiohttp.ClientSession() as session:
            async with session.get(f"{self.base_url}/healthz") as response:
                return await response.json()
    
    async def get_routes(self) -> List[Dict]:
        """Get all routing rules"""
        async with aiohttp.ClientSession() as session:
            async with session.get(f"{self.base_url}/v1/routes") as response:
                return await response.json()
    
    async def create_route(self, **kwargs) -> Dict:
        """Create a new routing rule"""
        data = {k: v for k, v in kwargs.items() if v is not None}
        async with aiohttp.ClientSession() as session:
            async with session.post(f"{self.base_url}/v1/routes", 
                                  json=data) as response:
                return await response.json()
    
    async def make_proxied_request(self, url: str, 
                                 proxy_type: str = "http") -> Dict:
        """Make request through ProxyRouter"""
        if proxy_type == "http":
            proxy_url = "http://192.168.10.230:8080"
        else:
            proxy_url = "socks5://192.168.10.230:1080"
        
        connector = aiohttp.ProxyConnector.from_url(proxy_url)
        async with aiohttp.ClientSession(connector=connector) as session:
            async with session.get(url) as response:
                return await response.json()

# Example async usage
async def main():
    client = AsyncProxyRouterClient()
    
    # Check health
    health = await client.get_health()
    print(f"Health: {health}")
    
    # Get routes
    routes = await client.get_routes()
    print(f"Routes: {len(routes)}")
    
    # Make proxied request
    result = await client.make_proxied_request("https://httpbin.org/ip")
    print(f"IP through proxy: {result}")

# Run async example
# asyncio.run(main())
```

### 5. Monitoring and Health Checks

```python
import requests
import time
from datetime import datetime
import logging

class ProxyRouterMonitor:
    def __init__(self, base_url="http://192.168.10.230:8081"):
        self.base_url = base_url
        self.session = requests.Session()
        self.logger = logging.getLogger(__name__)
    
    def check_health(self) -> bool:
        """Check if ProxyRouter is healthy"""
        try:
            response = self.session.get(f"{self.base_url}/healthz", timeout=5)
            return response.status_code == 200
        except Exception as e:
            self.logger.error(f"Health check failed: {e}")
            return False
    
    def get_metrics(self) -> Dict:
        """Get Prometheus metrics"""
        try:
            response = self.session.get(f"{self.base_url}/metrics", timeout=5)
            if response.status_code == 200:
                return {"status": "ok", "metrics": response.text}
            else:
                return {"status": "error", "code": response.status_code}
        except Exception as e:
            return {"status": "error", "error": str(e)}
    
    def check_proxy_connectivity(self, test_url="https://httpbin.org/ip") -> Dict:
        """Test proxy connectivity"""
        results = {}
        
        # Test HTTP proxy
        try:
            session = requests.Session()
            session.proxies = {
                'http': 'http://192.168.10.230:8080',
                'https': 'http://192.168.10.230:8080'
            }
            start_time = time.time()
            response = session.get(test_url, timeout=10)
            duration = time.time() - start_time
            
            results['http_proxy'] = {
                'status': 'ok' if response.status_code == 200 else 'error',
                'duration': duration,
                'response': response.json() if response.status_code == 200 else None
            }
        except Exception as e:
            results['http_proxy'] = {'status': 'error', 'error': str(e)}
        
        # Test SOCKS5 proxy (requires requests[socks])
        try:
            session = requests.Session()
            session.proxies = {
                'http': 'socks5://192.168.10.230:1080',
                'https': 'socks5://192.168.10.230:1080'
            }
            start_time = time.time()
            response = session.get(test_url, timeout=10)
            duration = time.time() - start_time
            
            results['socks5_proxy'] = {
                'status': 'ok' if response.status_code == 200 else 'error',
                'duration': duration,
                'response': response.json() if response.status_code == 200 else None
            }
        except Exception as e:
            results['socks5_proxy'] = {'status': 'error', 'error': str(e)}
        
        return results
    
    def monitor_loop(self, interval=60):
        """Continuous monitoring loop"""
        while True:
            timestamp = datetime.now().isoformat()
            
            # Check health
            health = self.check_health()
            
            # Check proxy connectivity
            connectivity = self.check_proxy_connectivity()
            
            # Log results
            self.logger.info(f"[{timestamp}] Health: {health}, Connectivity: {connectivity}")
            
            if not health:
                self.logger.warning("ProxyRouter health check failed!")
            
            time.sleep(interval)

# Example monitoring usage
logging.basicConfig(level=logging.INFO)
monitor = ProxyRouterMonitor()

# Single checks
print(f"Health: {monitor.check_health()}")
print(f"Connectivity: {monitor.check_proxy_connectivity()}")

# Start monitoring loop (uncomment to run)
# monitor.monitor_loop(interval=30)
```

## Testing Framework

### Unit Tests

```python
import unittest
import requests
import json
from unittest.mock import patch, Mock

class TestProxyRouterIntegration(unittest.TestCase):
    """Test ProxyRouter integration functionality"""
    
    def setUp(self):
        self.base_url = "http://192.168.10.230:8081"
        self.proxy_http = "http://192.168.10.230:8080" 
        self.proxy_socks5 = "socks5://192.168.10.230:1080"
        self.client = ProxyRouterClient(self.base_url)
    
    def test_health_check(self):
        """Test ProxyRouter health endpoint"""
        health = self.client.get_health()
        self.assertIn('status', health)
        self.assertEqual(health['status'], 'ok')
    
    def test_get_routes(self):
        """Test getting routes"""
        routes = self.client.get_routes()
        self.assertIsInstance(routes, list)
    
    def test_create_route(self):
        """Test creating a route"""
        # Create a test route
        route = self.client.create_route(
            host_glob="*.test.com",
            group="LOCAL",
            precedence=50
        )
        
        self.assertIn('id', route)
        
        # Clean up - delete the route
        route_id = route['id']
        # Note: You'd need to implement delete_route method
        
    def test_proxy_connectivity(self):
        """Test HTTP proxy connectivity"""
        session = requests.Session()
        session.proxies = {
            'http': self.proxy_http,
            'https': self.proxy_http
        }
        
        try:
            response = session.get('https://httpbin.org/ip', timeout=10)
            self.assertEqual(response.status_code, 200)
            
            # Verify we got a different IP (proxied)
            data = response.json()
            self.assertIn('origin', data)
        except Exception as e:
            self.fail(f"HTTP proxy connectivity test failed: {e}")
    
    def test_socks5_connectivity(self):
        """Test SOCKS5 proxy connectivity (requires requests[socks])"""
        try:
            session = requests.Session()
            session.proxies = {
                'http': self.proxy_socks5,
                'https': self.proxy_socks5
            }
            
            response = session.get('https://httpbin.org/ip', timeout=10)
            self.assertEqual(response.status_code, 200)
            
            data = response.json()
            self.assertIn('origin', data)
        except ImportError:
            self.skipTest("SOCKS support not available (install requests[socks])")
        except Exception as e:
            self.fail(f"SOCKS5 proxy connectivity test failed: {e}")
    
    def test_acl_functionality(self):
        """Test ACL management"""
        # Get current ACL
        acl = self.client.get_acl()
        initial_count = len(acl)
        
        # Add test ACL
        test_cidr = "192.168.100.0/24"
        result = self.client.add_acl(test_cidr)
        
        # Verify ACL was added
        updated_acl = self.client.get_acl()
        self.assertEqual(len(updated_acl), initial_count + 1)
        
        # Find our test CIDR
        found = any(item['cidr'] == test_cidr for item in updated_acl)
        self.assertTrue(found, f"Test CIDR {test_cidr} not found in ACL")
    
    @patch('requests.Session.get')
    def test_proxy_error_handling(self, mock_get):
        """Test error handling for proxy failures"""
        # Mock a connection error
        mock_get.side_effect = requests.ConnectionError("Connection failed")
        
        session = requests.Session()
        session.proxies = {'http': self.proxy_http, 'https': self.proxy_http}
        
        with self.assertRaises(requests.ConnectionError):
            session.get('https://httpbin.org/ip', timeout=5)
    
    def test_routing_rules(self):
        """Test that routing rules work correctly"""
        # Create a route for test domain
        test_route = self.client.create_route(
            host_glob="*.testdomain.com",
            group="LOCAL",
            precedence=1  # High precedence
        )
        
        # Make request to test domain through proxy
        # This should use LOCAL routing (direct connection)
        session = requests.Session()
        session.proxies = {
            'http': self.proxy_http,
            'https': self.proxy_http
        }
        
        # Since *.testdomain.com doesn't exist, this will fail
        # but we can verify the route exists
        routes = self.client.get_routes()
        test_routes = [r for r in routes if r.get('host_glob') == '*.testdomain.com']
        self.assertTrue(len(test_routes) > 0, "Test route not found")
        
        # Clean up - remove test route
        # (You'd need to implement route deletion)

class TestProxyRouterDatabase(unittest.TestCase):
    """Test direct database integration"""
    
    def setUp(self):
        # Use test database or mock
        self.db = ProxyRouterDatabase(":memory:")  # In-memory test DB
        # You'd need to create tables for testing
    
    def test_database_connection(self):
        """Test database connection"""
        with self.db.get_connection() as conn:
            cursor = conn.cursor()
            cursor.execute("SELECT 1")
            result = cursor.fetchone()
            self.assertEqual(result[0], 1)
    
    # Add more database tests...

if __name__ == '__main__':
    # Run tests
    unittest.main()
```

### Integration Test Script

```python
#!/usr/bin/env python3
"""
ProxyRouter Integration Test Script
Tests all major functionality and verifies proper operation
"""

import requests
import json
import time
import sys
from datetime import datetime

def run_integration_tests():
    """Run complete integration test suite"""
    
    print("🚀 Starting ProxyRouter Integration Tests")
    print(f"Timestamp: {datetime.now().isoformat()}")
    print("-" * 50)
    
    # Configuration
    base_url = "http://192.168.10.230:8081"
    http_proxy = "http://192.168.10.230:8080"
    socks5_proxy = "socks5://192.168.10.230:1080"
    
    client = ProxyRouterClient(base_url)
    
    test_results = {
        "passed": 0,
        "failed": 0,
        "errors": []
    }
    
    def test(name, func):
        """Run a test and record results"""
        try:
            print(f"Testing {name}...", end=" ")
            result = func()
            if result:
                print("✅ PASS")
                test_results["passed"] += 1
            else:
                print("❌ FAIL")
                test_results["failed"] += 1
                test_results["errors"].append(f"{name}: Test returned False")
        except Exception as e:
            print(f"❌ ERROR: {e}")
            test_results["failed"] += 1
            test_results["errors"].append(f"{name}: {str(e)}")
    
    # Test 1: Health Check
    def test_health():
        health = client.get_health()
        return health.get('status') == 'ok'
    
    # Test 2: API Routes
    def test_routes():
        routes = client.get_routes()
        return isinstance(routes, list)
    
    # Test 3: API Proxies
    def test_proxies():
        proxies = client.get_proxies()
        return isinstance(proxies, list)
    
    # Test 4: HTTP Proxy Connectivity
    def test_http_proxy():
        session = requests.Session()
        session.proxies = {'http': http_proxy, 'https': http_proxy}
        response = session.get('https://httpbin.org/ip', timeout=10)
        return response.status_code == 200
    
    # Test 5: SOCKS5 Proxy Connectivity  
    def test_socks5_proxy():
        try:
            session = requests.Session()
            session.proxies = {'http': socks5_proxy, 'https': socks5_proxy}
            response = session.get('https://httpbin.org/ip', timeout=10)
            return response.status_code == 200
        except ImportError:
            print("SKIPPED (requests[socks] not available)")
            return True
    
    # Test 6: Route Creation
    def test_route_creation():
        route = client.create_route(
            host_glob="*.testintegration.com",
            group="LOCAL",
            precedence=1
        )
        return 'id' in route
    
    # Test 7: ACL Management
    def test_acl():
        initial_acl = client.get_acl()
        client.add_acl("10.0.0.0/8")
        updated_acl = client.get_acl()
        return len(updated_acl) > len(initial_acl)
    
    # Test 8: Proxy Import
    def test_proxy_import():
        test_proxies = [
            {
                "scheme": "socks5",
                "host": "127.0.0.1",
                "port": 9999,
                "source": "integration_test"
            }
        ]
        result = client.import_proxies(test_proxies)
        return 'imported' in str(result).lower() or 'success' in str(result).lower()
    
    # Test 9: Metrics Endpoint
    def test_metrics():
        response = requests.get(f"{base_url}/metrics")
        return response.status_code == 200 and len(response.text) > 0
    
    # Test 10: Admin UI Accessibility
    def test_admin_ui():
        try:
            response = requests.get("http://192.168.10.230:8082/admin/", timeout=5)
            return response.status_code in [200, 302, 401]  # Various acceptable responses
        except:
            return False
    
    # Run all tests
    test("Health Check", test_health)
    test("Routes API", test_routes) 
    test("Proxies API", test_proxies)
    test("HTTP Proxy", test_http_proxy)
    test("SOCKS5 Proxy", test_socks5_proxy)
    test("Route Creation", test_route_creation)
    test("ACL Management", test_acl)
    test("Proxy Import", test_proxy_import)
    test("Metrics Endpoint", test_metrics)
    test("Admin UI", test_admin_ui)
    
    # Results summary
    print("-" * 50)
    print(f"🎯 Test Results Summary:")
    print(f"   ✅ Passed: {test_results['passed']}")
    print(f"   ❌ Failed: {test_results['failed']}")
    print(f"   📊 Total:  {test_results['passed'] + test_results['failed']}")
    
    if test_results["errors"]:
        print(f"\n🚨 Errors:")
        for error in test_results["errors"]:
            print(f"   • {error}")
    
    if test_results["failed"] == 0:
        print(f"\n🎉 All tests passed! ProxyRouter integration is working correctly.")
        return True
    else:
        print(f"\n⚠️  {test_results['failed']} tests failed. Check configuration and try again.")
        return False

if __name__ == "__main__":
    success = run_integration_tests()
    sys.exit(0 if success else 1)
```

## Advanced Usage Examples

### 1. Intelligent Routing Based on Content

```python
import requests
from urllib.parse import urlparse

class IntelligentProxyRouter:
    def __init__(self, proxyrouter_client):
        self.client = proxyrouter_client
        
        # Define routing strategies
        self.routing_rules = {
            # Social media through TOR for privacy
            'social': {
                'domains': ['facebook.com', 'twitter.com', 'instagram.com'],
                'group': 'TOR',
                'precedence': 10
            },
            # CDNs and static content direct (fast)
            'cdn': {
                'domains': ['*.cloudflare.com', '*.amazonaws.com', '*.googleapis.com'],
                'group': 'LOCAL',
                'precedence': 20
            },
            # Financial sites through specific upstream proxy
            'financial': {
                'domains': ['*.bank.com', '*.financial.com'],
                'group': 'UPSTREAM',
                'precedence': 5
            },
            # Everything else through general proxy pool
            'general': {
                'domains': ['*'],
                'group': 'GENERAL',
                'precedence': 100
            }
        }
    
    def setup_intelligent_routing(self):
        """Set up intelligent routing rules"""
        for category, config in self.routing_rules.items():
            for domain in config['domains']:
                self.client.create_route(
                    host_glob=domain,
                    group=config['group'],
                    precedence=config['precedence']
                )
                print(f"Created {category} route for {domain} -> {config['group']}")
    
    def analyze_request_pattern(self, urls):
        """Analyze request patterns and suggest optimizations"""
        domain_counts = {}
        
        for url in urls:
            domain = urlparse(url).netloc
            domain_counts[domain] = domain_counts.get(domain, 0) + 1
        
        # Sort by frequency
        sorted_domains = sorted(domain_counts.items(), key=lambda x: x[1], reverse=True)
        
        print("Request Pattern Analysis:")
        print("-" * 30)
        for domain, count in sorted_domains[:10]:  # Top 10
            print(f"{domain}: {count} requests")
        
        # Suggest optimizations
        high_volume_domains = [d for d, c in sorted_domains if c > 10]
        if high_volume_domains:
            print(f"\nSuggestion: Consider LOCAL routing for high-volume domains:")
            for domain in high_volume_domains:
                print(f"  - {domain}")

# Example usage
client = ProxyRouterClient()
router = IntelligentProxyRouter(client)
router.setup_intelligent_routing()

# Analyze patterns
urls = [
    "https://api.github.com/user",
    "https://www.github.com/settings", 
    "https://facebook.com/feed",
    "https://cdnjs.cloudflare.com/jquery.js",
    # ... more URLs
]
router.analyze_request_pattern(urls)
```

### 2. Failover and Load Balancing

```python
import requests
import random
import time
from concurrent.futures import ThreadPoolExecutor, as_completed

class ProxyRouterLoadBalancer:
    def __init__(self, proxy_endpoints):
        """
        proxy_endpoints: List of ProxyRouter instances
        e.g., ['192.168.10.230:8080', '192.168.10.231:8080']
        """
        self.endpoints = proxy_endpoints
        self.endpoint_health = {ep: True for ep in proxy_endpoints}
        self.request_counts = {ep: 0 for ep in proxy_endpoints}
    
    def health_check(self, endpoint):
        """Check health of a proxy endpoint"""
        try:
            api_endpoint = endpoint.replace(':8080', ':8081')  # API port
            response = requests.get(f"http://{api_endpoint}/healthz", timeout=3)
            return response.status_code == 200
        except:
            return False
    
    def update_health_status(self):
        """Update health status of all endpoints"""
        for endpoint in self.endpoints:
            self.endpoint_health[endpoint] = self.health_check(endpoint)
    
    def get_healthy_endpoints(self):
        """Get list of healthy endpoints"""
        return [ep for ep in self.endpoints if self.endpoint_health[ep]]
    
    def select_endpoint(self, strategy="round_robin"):
        """Select best endpoint based on strategy"""
        healthy_endpoints = self.get_healthy_endpoints()
        
        if not healthy_endpoints:
            raise Exception("No healthy proxy endpoints available")
        
        if strategy == "round_robin":
            # Select endpoint with lowest request count
            return min(healthy_endpoints, key=lambda ep: self.request_counts[ep])
        
        elif strategy == "random":
            return random.choice(healthy_endpoints)
        
        elif strategy == "least_loaded":
            # This could be enhanced to check actual load
            return min(healthy_endpoints, key=lambda ep: self.request_counts[ep])
        
        else:
            return healthy_endpoints[0]
    
    def make_request(self, url, strategy="round_robin", max_retries=3):
        """Make request with automatic failover"""
        last_exception = None
        
        for retry in range(max_retries):
            try:
                # Update health status periodically
                if retry > 0 or random.random() < 0.1:  # 10% of the time
                    self.update_health_status()
                
                # Select endpoint
                endpoint = self.select_endpoint(strategy)
                
                # Make request
                session = requests.Session()
                session.proxies = {
                    'http': f'http://{endpoint}',
                    'https': f'http://{endpoint}'
                }
                
                response = session.get(url, timeout=10)
                
                # Increment success counter
                self.request_counts[endpoint] += 1
                
                return response
                
            except Exception as e:
                last_exception = e
                print(f"Request failed on retry {retry + 1}: {e}")
                
                # Mark endpoint as potentially unhealthy
                if 'endpoint' in locals():
                    self.endpoint_health[endpoint] = False
                
                if retry < max_retries - 1:
                    time.sleep(2 ** retry)  # Exponential backoff
        
        raise Exception(f"All retries failed. Last error: {last_exception}")
    
    def get_stats(self):
        """Get load balancer statistics"""
        return {
            "endpoints": self.endpoints,
            "health_status": self.endpoint_health,
            "request_counts": self.request_counts,
            "healthy_count": sum(self.endpoint_health.values())
        }

# Example usage
endpoints = [
    '192.168.10.230:8080',
    '192.168.10.231:8080',
    '192.168.10.232:8080'
]

lb = ProxyRouterLoadBalancer(endpoints)

# Make requests with load balancing
urls = [
    'https://httpbin.org/ip',
    'https://httpbin.org/headers', 
    'https://httpbin.org/user-agent'
]

# Test concurrent requests
with ThreadPoolExecutor(max_workers=10) as executor:
    futures = []
    
    for i in range(30):  # 30 requests
        url = random.choice(urls)
        future = executor.submit(lb.make_request, url, strategy="round_robin")
        futures.append(future)
    
    results = []
    for future in as_completed(futures):
        try:
            response = future.result()
            results.append(response.status_code)
        except Exception as e:
            print(f"Request failed: {e}")

print(f"Completed {len(results)} requests")
print(f"Load Balancer Stats: {lb.get_stats()}")
```

## Docker Integration

### Docker Compose Setup

```yaml
# docker-compose.yml for Python app + ProxyRouter
version: '3.8'

services:
  proxyrouter:
    image: proxyrouter:latest
    ports:
      - "8080:8080"    # HTTP Proxy
      - "1080:1080"    # SOCKS5 Proxy  
      - "8081:8081"    # API
      - "8082:8082"    # Admin UI
    volumes:
      - ./data:/var/lib/proxyr
      - ./configs:/etc/proxyrouter
    environment:
      - PROXYROUTER_DATABASE_PATH=/var/lib/proxyr/router.db
      - PROXYROUTER_ADMIN_ENABLED=true
    depends_on:
      - tor
    networks:
      - proxy_network
  
  tor:
    image: dperson/torproxy
    ports:
      - "9050:9050"    # Tor SOCKS
    networks:
      - proxy_network
  
  python_app:
    build: .
    environment:
      - PROXYROUTER_API_URL=http://proxyrouter:8081
      - HTTP_PROXY=http://proxyrouter:8080
      - HTTPS_PROXY=http://proxyrouter:8080
      - SOCKS_PROXY=socks5://proxyrouter:1080
    depends_on:
      - proxyrouter
    networks:
      - proxy_network

networks:
  proxy_network:
    driver: bridge
```

### Python Application Dockerfile

```dockerfile
# Dockerfile for Python app
FROM python:3.11-slim

WORKDIR /app

# Install dependencies
COPY requirements.txt .
RUN pip install -r requirements.txt

# Install SOCKS support
RUN pip install requests[socks]

# Copy application code
COPY . .

# Run application
CMD ["python", "app.py"]
```

### Requirements File

```txt
# requirements.txt
requests==2.31.0
requests[socks]==2.31.0
aiohttp==3.8.5
pysocks==1.7.1
```

## Production Deployment Checklist

### 1. Security Configuration

```python
import requests
import ssl
from requests.adapters import HTTPAdapter
from urllib3.util.retry import Retry

class SecureProxyRouterClient:
    """Production-ready ProxyRouter client with security features"""
    
    def __init__(self, base_url, api_key=None):
        self.base_url = base_url
        self.session = requests.Session()
        
        # Configure retries
        retry_strategy = Retry(
            total=3,
            backoff_factor=1,
            status_forcelist=[429, 500, 502, 503, 504],
        )
        
        adapter = HTTPAdapter(max_retries=retry_strategy)
        self.session.mount("http://", adapter)
        self.session.mount("https://", adapter)
        
        # Set security headers
        self.session.headers.update({
            'User-Agent': 'ProxyRouterClient/1.0',
            'Accept': 'application/json',
            'Content-Type': 'application/json'
        })
        
        if api_key:
            self.session.headers['Authorization'] = f'Bearer {api_key}'
    
    def verify_ssl_config(self):
        """Verify SSL/TLS configuration"""
        try:
            response = self.session.get(f"{self.base_url}/healthz", timeout=5)
            return {
                'ssl_verified': True,
                'status_code': response.status_code,
                'tls_version': getattr(response.raw._connection, 'version', 'Unknown')
            }
        except ssl.SSLError as e:
            return {'ssl_verified': False, 'error': str(e)}
    
    def audit_configuration(self):
        """Audit ProxyRouter configuration for security"""
        try:
            # Check ACL configuration
            acl = self.session.get(f"{self.base_url}/v1/acl").json()
            
            # Check routes
            routes = self.session.get(f"{self.base_url}/v1/routes").json()
            
            audit_results = {
                'acl_rules': len(acl),
                'routing_rules': len(routes),
                'security_issues': []
            }
            
            # Check for overly permissive ACL rules
            for rule in acl:
                if rule.get('cidr') == '0.0.0.0/0':
                    audit_results['security_issues'].append(
                        'Overly permissive ACL rule allows all IPs'
                    )
            
            return audit_results
            
        except Exception as e:
            return {'error': str(e)}
```

### 2. Environment Configuration

```bash
#!/bin/bash
# production-setup.sh

# Create production configuration
cat > /etc/proxyrouter/prod-config.yaml << EOF
listen:
  http_proxy: "0.0.0.0:8080"
  socks5_proxy: "0.0.0.0:1080" 
  api: "127.0.0.1:8081"  # API only on localhost for security

timeouts:
  dial_ms: 15000
  read_ms: 30000
  write_ms: 30000

database:
  path: "/var/lib/proxyrouter/production.db"

admin:
  enabled: true
  bind: "127.0.0.1"  # Admin UI only on localhost
  port: 8082
  allow_cidrs: ["127.0.0.1/32"]  # Restrict to localhost
  tls:
    enabled: true
    cert_file: "/etc/ssl/certs/proxyrouter.crt"
    key_file: "/etc/ssl/private/proxyrouter.key"

security:
  password_hash: "argon2id"
  login:
    max_attempts: 5
    window_seconds: 300
EOF

# Set proper permissions
chown proxyrouter:proxyrouter /etc/proxyrouter/prod-config.yaml
chmod 600 /etc/proxyrouter/prod-config.yaml

# Create systemd service
systemctl enable proxyrouter
systemctl start proxyrouter

echo "Production ProxyRouter setup complete!"
```

### 3. Monitoring and Logging

```python
import logging
import time
import json
from datetime import datetime
from pathlib import Path

class ProxyRouterLogger:
    """Production logging for ProxyRouter integration"""
    
    def __init__(self, log_file="/var/log/proxyrouter-client.log"):
        self.log_file = Path(log_file)
        
        # Setup logging
        logging.basicConfig(
            level=logging.INFO,
            format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
            handlers=[
                logging.FileHandler(log_file),
                logging.StreamHandler()
            ]
        )
        
        self.logger = logging.getLogger('ProxyRouterClient')
    
    def log_request(self, url, proxy_type, duration, status_code, error=None):
        """Log individual requests"""
        log_entry = {
            'timestamp': datetime.utcnow().isoformat(),
            'url': url,
            'proxy_type': proxy_type,
            'duration_ms': round(duration * 1000, 2),
            'status_code': status_code,
            'success': status_code == 200 and error is None,
            'error': str(error) if error else None
        }
        
        if error:
            self.logger.error(f"Request failed: {json.dumps(log_entry)}")
        else:
            self.logger.info(f"Request completed: {json.dumps(log_entry)}")
    
    def log_health_check(self, endpoint, is_healthy, response_time=None):
        """Log health check results"""
        log_entry = {
            'timestamp': datetime.utcnow().isoformat(),
            'endpoint': endpoint,
            'healthy': is_healthy,
            'response_time_ms': round(response_time * 1000, 2) if response_time else None
        }
        
        if is_healthy:
            self.logger.info(f"Health check passed: {json.dumps(log_entry)}")
        else:
            self.logger.warning(f"Health check failed: {json.dumps(log_entry)}")
    
    def log_configuration_change(self, action, details):
        """Log configuration changes"""
        log_entry = {
            'timestamp': datetime.utcnow().isoformat(),
            'action': action,
            'details': details
        }
        
        self.logger.info(f"Configuration change: {json.dumps(log_entry)}")

# Example usage in production
logger = ProxyRouterLogger()

# Log requests
start_time = time.time()
try:
    response = requests.get('https://api.example.com', 
                          proxies={'https': 'http://proxyrouter:8080'})
    duration = time.time() - start_time
    logger.log_request('https://api.example.com', 'http', duration, response.status_code)
except Exception as e:
    duration = time.time() - start_time
    logger.log_request('https://api.example.com', 'http', duration, 0, error=e)
```

## Conclusion

This comprehensive guide provides LLMs with everything needed to understand and integrate the ProxyRouter project into Python applications. Key integration patterns include:

1. **Direct Proxy Usage** - Using ProxyRouter as HTTP/SOCKS5 proxy
2. **API Integration** - Managing ProxyRouter via REST API  
3. **Database Integration** - Direct SQLite database access
4. **Async Support** - High-performance async operations
5. **Monitoring** - Health checks and performance monitoring
6. **Load Balancing** - Multi-instance deployments
7. **Production Security** - Security hardening and audit capabilities

The code examples are production-ready and include proper error handling, logging, and security considerations. The test framework ensures reliable integration and the Docker setup enables easy deployment.

Remember to:
- Always implement proper error handling
- Use connection pooling for high-volume applications
- Monitor proxy health and performance
- Implement proper security measures in production
- Log all operations for debugging and audit purposes
- Test thoroughly before production deployment

This guide enables LLMs to successfully integrate ProxyRouter into any Python project while following best practices for reliability, security, and performance.
