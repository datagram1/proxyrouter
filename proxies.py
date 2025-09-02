# proxies.py
"""
Reusable proxy helper for MySQL-backed rotation across requests + Playwright.

Usage:
    from proxies import Proxy_Helper

    # Use config system to get DB credentials
    from config import C
    p = Proxy_Helper.from_config(C)

    # (Optional) Force-refresh from GitHub + validate all
    p.refresh_proxies(force=True, test_all=True)

    # Use a latched proxy for a run of HTTP requests
    with p.session() as s:
        r = s.get("https://httpbin.org/ip", timeout=20)
        print(r.text)

    # Rotate to the next proxy mid-run if you like
    p.rotate_proxy()

    # --- Playwright example (async) ---
    # import asyncio
    # from playwright.async_api import async_playwright
    # async def run():
    #     async with async_playwright() as pw:
    #         browser, context, page = await p.new_playwright_context(pw, headless=True)
    #         await page.goto("https://httpbin.org/ip", timeout=30000)
    #         print(await page.text_content("body"))
    #         await browser.close()
    # asyncio.run(run())

Table assumed (MySQL 8+):
    CREATE TABLE `proxies` (
      `id` INT AUTO_INCREMENT PRIMARY KEY,
      `ip` VARCHAR(45) NOT NULL,
      `port` INT NOT NULL,
      `proxy_url` VARCHAR(255) NOT NULL,
      `proxy_type` VARCHAR(20) DEFAULT 'SOCKS5',
      `source` VARCHAR(50) DEFAULT 'github_socks5',
      `working` TINYINT(1) DEFAULT 0,
      `tested_timestamp` TIMESTAMP NULL,
      `last_used_timestamp` TIMESTAMP NULL,
      `latency` DECIMAL(10,3) DEFAULT 0.000,
      `tested_ip` VARCHAR(45) NULL,
      `error_message` TEXT NULL,
      `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
      CONSTRAINT `unique_proxy` UNIQUE (`ip`,`port`),
      KEY `idx_ip_port` (`ip`,`port`),
      KEY `idx_proxy_url` (`proxy_url`),
      KEY `idx_tested_timestamp` (`tested_timestamp`),
      KEY `idx_working` (`working`)
    ) ENGINE=InnoDB;

Notes:
- Requires MySQL 8 for SKIP LOCKED. If you're on 5.7, change acquire logic to a single-row UPDATE ... LIMIT 1 claim pattern.
- SOCKS support needs `requests[socks]` (PySocks).
"""

from __future__ import annotations

import os
import re
import time
import json
import math
import queue
import typing as t
import datetime as dt
from decimal import Decimal
from contextlib import contextmanager, asynccontextmanager

import requests

try:
    import pymysql as mysql
except ImportError as e:  # pragma: no cover
    raise RuntimeError("PyMySQL is required. pip install pymysql") from e


# --------------------------- Utilities ---------------------------

IP_PORT_RE = re.compile(
    r"^(?:(?P<scheme>https?|socks5|socks4)://)?(?P<ip>[A-Za-z0-9\.\-:]+?):(?P<port>\d{2,5})$"
)

def _now() -> str:
    return dt.datetime.utcnow().strftime("%Y-%m-%d %H:%M:%S")


def _normalize_proxy_row(
        raw: str, default_type: str = "SOCKS5", source: str = "github"
) -> t.Optional[dict]:
    raw = raw.strip()
    if not raw or raw.startswith("#"):
        return None
    m = IP_PORT_RE.match(raw)
    if not m:
        return None
    scheme = (m.group("scheme") or "").lower()
    ip = m.group("ip")
    port = int(m.group("port"))

    if scheme in ("http", "https"):
        proxy_type = "HTTP"
        scheme = "http"
    elif scheme in ("socks5", "socks4"):
        proxy_type = "SOCKS5" if scheme == "socks5" else "SOCKS4"
    else:
        proxy_type = default_type.upper()
        scheme = "socks5" if proxy_type == "SOCKS5" else "http"

    proxy_url = f"{scheme}://{ip}:{port}"
    return {
        "ip": ip,
        "port": port,
        "proxy_url": proxy_url,
        "proxy_type": proxy_type,
        "source": source,
    }


def _requests_proxy_mapping(proxy_url: str, proxy_type: str) -> dict:
    # Use socks5h to avoid DNS leaks when using SOCKS
    if proxy_type.upper().startswith("SOCKS"):
        proxy_url = proxy_url.replace("socks5://", "socks5h://").replace("socks4://", "socks4a://")
    return {"http": proxy_url, "https": proxy_url}


# --------------------------- Class ---------------------------

class Proxy_Helper:
    def __init__(
            self,
            db_user: str,
            db_password: str,
            db_name: str = "companies",
            db_host: str = "127.0.0.1",
            db_port: int = 3306,
            table: str = "proxies",
            force_proxy_refresh: bool = False,
            proxy_sources: t.Optional[t.List[str]] = None,
            test_target_url: str = "http://ip.knws.co.uk",
            test_timeout: int = 12,
    ):
        """
        Instantiate the helper and (optionally) refresh proxies from GitHub.

        Arguments match your shorthand `Proxy_Helper(user, pass, db, ip)` signature:
          - db_user, db_password, db_name, db_host (ip), db_port

        You can override:
          - table (defaults to 'proxies' in your db)
          - proxy_sources (list of raw text URLs, each line 'ip:port' or schema-prefixed)
          - force_proxy_refresh: if True, immediately download+upsert+test on init
        """
        self.db_user = db_user
        self.db_password = db_password
        self.db_name = db_name
        self.db_host = db_host
        self.db_port = db_port
        self.table = table
        self.test_target_url = test_target_url
        self.test_timeout = test_timeout

        # Default sources: solid public lists for socks5/http
        self.proxy_sources = proxy_sources or [
            # SOCKS5 lists
            "https://raw.githubusercontent.com/hookzof/socks5_list/master/proxy.txt",
            "https://raw.githubusercontent.com/monosans/proxy-list/main/proxies/socks5.txt",
            "https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/socks5.txt",
            # HTTP lists (optional — you can comment out if you only want SOCKS5)
            "https://raw.githubusercontent.com/monosans/proxy-list/main/proxies/http.txt",
            "https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/http.txt",
        ]

        self._conn = None  # type: t.Optional[mysql.MySQLConnection]
        self._current_proxy = None  # type: t.Optional[dict]

        self._connect()

        if force_proxy_refresh:
            self.refresh_proxies(force=True, test_all=True)

    @classmethod
    def from_config(cls, config, **kwargs):
        """
        Create Proxy_Helper instance from config object.
        
        Args:
            config: Config object from config.py
            **kwargs: Additional arguments to override config values
        """
        return cls(
            db_user=kwargs.get('db_user', config.user),
            db_password=kwargs.get('db_password', config.password),
            db_name=kwargs.get('db_name', config.proxy_db),  # Use DATABASE_PROXY from config
            db_host=kwargs.get('db_host', config.host),
            db_port=kwargs.get('db_port', config.port),
            **kwargs
        )

    # --------------------------- DB ---------------------------

    def _connect(self):
        if self._conn and hasattr(self._conn, 'open') and self._conn.open:
            return
        self._conn = mysql.connect(
            host=self.db_host,
            port=self.db_port,
            user=self.db_user,
            password=self.db_password,
            database=self.db_name,
            autocommit=False,
        )

    def _cursor(self):
        self._connect()
        return self._conn.cursor(mysql.cursors.DictCursor)

    def _commit(self):
        if self._conn:
            self._conn.commit()

    def _rollback(self):
        if self._conn:
            self._conn.rollback()

    def close(self):
        if self._conn and hasattr(self._conn, 'open') and self._conn.open:
            self._conn.close()
        self._conn = None

    # --------------------------- Refresh / Download / Upsert ---------------------------

    def refresh_proxies(self, force: bool = False, test_all: bool = True, max_to_test: int = 500):
        """
        If `force` or DB is empty, download proxy lists, upsert, and (optionally) test.
        """
        needs = force or (self._count_proxies() == 0)
        if not needs:
            return

        try:
            lines = self._download_proxy_lines(self.proxy_sources)
            items = []
            for src, content in lines.items():
                for line in content.splitlines():
                    row = _normalize_proxy_row(line, default_type="SOCKS5", source=self._source_label(src))
                    if row:
                        items.append(row)
            if not items:
                return

            self._bulk_upsert(items)

            if test_all:
                self.validate_proxies(limit=max_to_test)
        except KeyboardInterrupt:
            print("\n⚠️ Proxy refresh interrupted by user")
            raise
        except Exception as e:
            print(f"❌ Proxy refresh failed: {e}")
            raise

    def _source_label(self, url: str) -> str:
        if "socks5" in url:
            return "github_socks5"
        if "http.txt" in url:
            return "github_http"
        if "hookzof" in url:
            return "hookzof"
        if "TheSpeedX" in url:
            return "TheSpeedX"
        if "monosans" in url:
            return "monosans"
        return "github"

    def _download_proxy_lines(self, urls: t.List[str]) -> dict:
        out = {}
        for u in urls:
            try:
                r = requests.get(u, timeout=20)
                if r.status_code == 200 and r.text:
                    out[u] = r.text
            except Exception:
                pass
        return out

    def _count_proxies(self) -> int:
        with self._cursor() as cur:
            cur.execute(f"SELECT COUNT(*) AS n FROM `{self.table}`")
            row = cur.fetchone()
            return int(row["n"] or 0)

    def _bulk_upsert(self, items: t.List[dict]):
        if not items:
            return
        # De-dup in memory by (ip, port)
        seen = set()
        deduped = []
        for it in items:
            k = (it["ip"], it["port"])
            if k not in seen:
                seen.add(k)
                deduped.append(it)

        sql = (
            f"INSERT INTO `{self.table}` (ip, port, proxy_url, proxy_type, source, working, tested_timestamp) "
            f"VALUES (%s,%s,%s,%s,%s,0,NULL) "
            f"ON DUPLICATE KEY UPDATE "
            f"proxy_url=VALUES(proxy_url), proxy_type=VALUES(proxy_type), source=VALUES(source), updated_at=NOW()"
        )
        vals = [(d["ip"], d["port"], d["proxy_url"], d["proxy_type"], d["source"]) for d in deduped]

        try:
            with self._cursor() as cur:
                cur.executemany(sql, vals)
            self._commit()
        except KeyboardInterrupt:
            print("\n⚠️ Database upsert interrupted by user")
            self._rollback()
            raise
        except Exception as e:
            print(f"❌ Database upsert failed: {e}")
            self._rollback()
            raise

    # --------------------------- Validation ---------------------------

    def validate_proxies(self, limit: int = 500, workers: int = 30, retest_stale_minutes: int = 720):
        """
        Pull a chunk of proxies (stale or untested), test them concurrently, and update DB.
        """
        try:
            target = self.test_target_url
            with self._cursor() as cur:
                cur.execute(
                    f"""
                    SELECT id, ip, port, proxy_url, proxy_type
                    FROM `{self.table}`
                    WHERE tested_timestamp IS NULL
                       OR tested_timestamp < (UTC_TIMESTAMP() - INTERVAL %s MINUTE)
                    LIMIT %s
                    """,
                    (retest_stale_minutes, limit),
                )
                rows = cur.fetchall()

            if not rows:
                return 0

            from concurrent.futures import ThreadPoolExecutor, as_completed

            def _one(r):
                start = time.perf_counter()
                proxies = _requests_proxy_mapping(r["proxy_url"], r["proxy_type"])
                ok = 0
                latency = None
                tested_ip = None
                err = None
                try:
                    resp = requests.get(target, timeout=self.test_timeout, proxies=proxies)
                    latency = time.perf_counter() - start
                    if resp.status_code == 200:
                        # For ip.knws.co.uk, we expect a simple IP address as plain text
                        tested_ip = resp.text.strip()[:45]
                        ok = 1 if tested_ip else 0
                    else:
                        err = f"HTTP {resp.status_code}"
                except Exception as e:
                    err = str(e)[:500]
                return (r["id"], ok, latency, tested_ip, err)

            updates = []
            with ThreadPoolExecutor(max_workers=workers) as ex:
                futs = [ex.submit(_one, r) for r in rows]
                for f in as_completed(futs):
                    updates.append(f.result())

            with self._cursor() as cur:
                for pid, ok, latency, tested_ip, err in updates:
                    cur.execute(
                        f"""
                        UPDATE `{self.table}`
                           SET working=%s,
                               tested_timestamp=UTC_TIMESTAMP(),
                               latency=%s,
                               tested_ip=%s,
                               error_message=%s,
                               updated_at=UTC_TIMESTAMP()
                         WHERE id=%s
                        """,
                        (ok, (None if latency is None else round(latency, 3)), tested_ip, err, pid),
                    )
            self._commit()
            return len(updates)
        except KeyboardInterrupt:
            print("\n⚠️ Proxy validation interrupted by user")
            raise
        except Exception as e:
            print(f"❌ Proxy validation failed: {e}")
            raise

    def test_specific_ip(self, ip: str, verbose: bool = False):
        """
        Test a specific IP address from the database.
        """
        try:
            with self._cursor() as cur:
                cur.execute(f"SELECT * FROM `{self.table}` WHERE ip = %s", (ip,))
                row = cur.fetchone()

            if not row:
                print(f"❌ IP '{ip}' not found in database")
                return False

            print(f"🔍 Testing IP: {ip} (found in database)")
            
            # Test the proxy
            success, result = self.test_proxy(row["proxy_url"], timeout=20, proxy_type=row.get("proxy_type"), verbose=verbose)
            
            if success:
                print(f"✅ {ip} -> {result}")
                return True
            else:
                print(f"❌ {ip} -> {result}")
                return False
                
        except Exception as e:
            print(f"❌ Error testing IP {ip}: {e}")
            return False

    def test_working_proxies(self, limit: int = None, verbose: bool = False, max_workers: int = 30):
        """
        Test only working proxies (working=1).
        """
        return self._test_proxies_with_condition("working=1", "working", limit, verbose, max_workers)

    def test_failed_proxies(self, limit: int = None, verbose: bool = False, max_workers: int = 30):
        """
        Test only failed proxies (working=0).
        """
        return self._test_proxies_with_condition("working=0", "failed", limit, verbose, max_workers)

    def test_untested_proxies(self, limit: int = None, verbose: bool = False, max_workers: int = 30):
        """
        Test only untested proxies (tested_timestamp IS NULL).
        """
        return self._test_proxies_with_condition("tested_timestamp IS NULL", "untested", limit, verbose, max_workers)

    def _test_proxies_with_condition(self, condition: str, category: str, limit: int = None, verbose: bool = False, max_workers: int = 30):
        """
        Generic method to test proxies with a specific condition.
        """
        try:
            with self._cursor() as cur:
                if limit:
                    cur.execute(f"SELECT id, ip, port, proxy_url, proxy_type FROM `{self.table}` WHERE {condition} LIMIT %s", (limit,))
                else:
                    cur.execute(f"SELECT id, ip, port, proxy_url, proxy_type FROM `{self.table}` WHERE {condition}")
                rows = cur.fetchall()

            if not rows:
                print(f"No {category} proxies found in database")
                return 0

            print(f"Testing {len(rows)} {category} proxies against {self.test_target_url} with 20s timeout each...")
            
            from concurrent.futures import ThreadPoolExecutor, as_completed

            def _test_proxy(r):
                # Use the new single-proxy test method with realistic headers
                # Pass proxy_type information for proper protocol detection
                success, result = self.test_proxy(r["proxy_url"], timeout=20, proxy_type=r.get("proxy_type"), verbose=verbose)
                
                if success:
                    # Extract IP from result message
                    tested_ip = result.split("IP: ")[-1] if "IP: " in result else None
                    print(f"✅ {r['ip']}:{r['port']} -> {tested_ip}")
                    return (r["id"], 1, None, tested_ip, None)
                else:
                    print(f"❌ {r['ip']}:{r['port']} -> {result}")
                    return (r["id"], 0, None, None, result)

            updates = []
            with ThreadPoolExecutor(max_workers=max_workers) as ex:
                futs = [ex.submit(_test_proxy, r) for r in rows]
                for f in as_completed(futs):
                    updates.append(f.result())

            with self._cursor() as cur:
                for pid, ok, latency, tested_ip, err in updates:
                    cur.execute(
                        f"""
                        UPDATE `{self.table}`
                           SET working=%s,
                               tested_timestamp=UTC_TIMESTAMP(),
                               latency=%s,
                               tested_ip=%s,
                               error_message=%s,
                               updated_at=UTC_TIMESTAMP()
                         WHERE id=%s
                        """,
                        (ok, (None if latency is None else round(latency, 3)), tested_ip, err, pid),
                    )
            self._commit()
            
            # Print summary
            working = sum(1 for _, ok, _, _, _ in updates if ok)
            total = len(updates)
            print(f"\n📊 Test Results: {working}/{total} {category} proxies working ({working/total*100:.1f}%)")
            
            return len(updates)
        except KeyboardInterrupt:
            print(f"\n⚠️ {category.capitalize()} proxy testing interrupted by user")
            raise
        except Exception as e:
            print(f"❌ {category.capitalize()} proxy testing failed: {e}")
            raise

    def test_all_proxies(self, limit: int = None, verbose: bool = False, max_workers: int = 30):
        """
        Test all proxies in the database using the new test site with single long timeout.
        This is useful for refreshing the working status of all proxies.
        """
        try:
            with self._cursor() as cur:
                if limit:
                    cur.execute(f"SELECT id, ip, port, proxy_url, proxy_type FROM `{self.table}` LIMIT %s", (limit,))
                else:
                    cur.execute(f"SELECT id, ip, port, proxy_url, proxy_type FROM `{self.table}`")
                rows = cur.fetchall()

            if not rows:
                print("No proxies found in database")
                return 0

            print(f"Testing {len(rows)} proxies against {self.test_target_url} with 20s timeout each...")
            
            from concurrent.futures import ThreadPoolExecutor, as_completed

            def _test_proxy(r):
                # Use the new single-proxy test method with realistic headers
                # Pass proxy_type information for proper protocol detection
                success, result = self.test_proxy(r["proxy_url"], timeout=20, proxy_type=r.get("proxy_type"), verbose=verbose)
                
                if success:
                    # Extract IP from result message
                    tested_ip = result.split("IP: ")[-1] if "IP: " in result else None
                    print(f"✅ {r['ip']}:{r['port']} -> {tested_ip}")
                    return (r["id"], 1, None, tested_ip, None)
                else:
                    print(f"❌ {r['ip']}:{r['port']} -> {result}")
                    return (r["id"], 0, None, None, result)

            updates = []
            with ThreadPoolExecutor(max_workers=max_workers) as ex:
                futs = [ex.submit(_test_proxy, r) for r in rows]
                for f in as_completed(futs):
                    updates.append(f.result())

            with self._cursor() as cur:
                for pid, ok, latency, tested_ip, err in updates:
                    cur.execute(
                        f"""
                        UPDATE `{self.table}`
                           SET working=%s,
                               tested_timestamp=UTC_TIMESTAMP(),
                               latency=%s,
                               tested_ip=%s,
                               error_message=%s,
                               updated_at=UTC_TIMESTAMP()
                         WHERE id=%s
                        """,
                        (ok, (None if latency is None else round(latency, 3)), tested_ip, err, pid),
                    )
            self._commit()
            
            # Print summary
            working = sum(1 for _, ok, _, _, _ in updates if ok)
            total = len(updates)
            print(f"\n📊 Test Results: {working}/{total} proxies working ({working/total*100:.1f}%)")
            
            return len(updates)
        except KeyboardInterrupt:
            print("\n⚠️ Proxy testing interrupted by user")
            raise
        except Exception as e:
            print(f"❌ Proxy testing failed: {e}")
            raise

    def test_proxy(self, proxy_str, timeout=20, proxy_type=None, verbose=False):
        """Test a single proxy with realistic browser headers and single long timeout."""
        try:
            # Parse proxy string - handle both "ip:port" and "protocol://ip:port" formats
            if '://' in proxy_str:
                # URL format like "socks5://104.21.27.195:80"
                protocol_part, rest = proxy_str.split('://', 1)
                if ':' not in rest:
                    return False, "Invalid proxy format"
                host, port_str = rest.rsplit(':', 1)
            else:
                # Simple format like "104.21.27.195:80"
                if ':' not in proxy_str:
                    return False, "Invalid proxy format"
                host, port_str = proxy_str.rsplit(':', 1)
            
            try:
                port = int(port_str)
            except ValueError:
                return False, "Invalid port number"
            
            # Use urllib3 directly to avoid requests retry mechanism
            import urllib3
            from urllib3.contrib.socks import SOCKSProxyManager
            
            # Configure proxy based on proxy type with fallback strategy
            protocols_to_try = []
            
            if '://' in proxy_str:
                # Use protocol from URL as primary
                protocols_to_try.append(protocol_part)
            elif proxy_type:
                # Use labeled proxy type as primary
                protocol_map = {
                    'socks5': 'socks5',
                    'socks4': 'socks4', 
                    'http': 'http',
                    'https': 'https'
                }
                primary_protocol = protocol_map.get(proxy_type.upper(), 'socks5')
                protocols_to_try.append(primary_protocol)
            else:
                # No known type, try all protocols
                protocols_to_try = ['socks5', 'socks4', 'https', 'http']
            
            # If we have a known working proxy type, only try that one
            if proxy_type and not verbose:
                protocols_to_try = [protocols_to_try[0]]
            elif verbose:
                # In verbose mode, try fallback protocols
                if len(protocols_to_try) == 1:
                    # Add fallback protocols for verbose testing
                    primary = protocols_to_try[0]
                    if primary == 'socks5':
                        protocols_to_try = ['socks5', 'socks4', 'https', 'http']
                    elif primary == 'socks4':
                        protocols_to_try = ['socks4', 'socks5', 'https', 'http']
                    elif primary == 'https':
                        protocols_to_try = ['https', 'http', 'socks5', 'socks4']
                    elif primary == 'http':
                        protocols_to_try = ['http', 'https', 'socks5', 'socks4']
            
            # Create headers
            headers = {
                'User-Agent': 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
                'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8',
                'Accept-Language': 'en-US,en;q=0.5',
                'Accept-Encoding': 'gzip, deflate',
                'Connection': 'keep-alive',
                'Upgrade-Insecure-Requests': '1',
            }
            
            # Try each protocol until one works
            for protocol in protocols_to_try:
                if verbose and len(protocols_to_try) > 1:
                    print(f"  Trying {protocol}...", end=" ")
                
                try:
                    # Use urllib3 directly with no retries
                    if protocol in ['socks5', 'socks4']:
                        proxy_url = f'{protocol}://{host}:{port}'
                        http = SOCKSProxyManager(proxy_url, timeout=urllib3.Timeout(connect=timeout, read=timeout))
                    else:
                        # For HTTP/HTTPS proxies, use regular HTTPConnectionPool
                        proxy_url = f'{protocol}://{host}:{port}'
                        http = urllib3.ProxyManager(proxy_url, timeout=urllib3.Timeout(connect=timeout, read=timeout))
                    
                    # Single request with no retries
                    response = http.request('GET', 'http://ip.knws.co.uk', headers=headers, retries=False)
                    
                    if response.status == 200:
                        # Verify we got a valid IP response
                        content = response.data.decode('utf-8').strip()
                        if content and len(content) < 50:  # Should be just an IP address
                            if verbose and len(protocols_to_try) > 1:
                                print(f"✅ {protocol} worked!")
                            return True, f"Working - IP: {content} (via {protocol})"
                        else:
                            if verbose and len(protocols_to_try) > 1:
                                print(f"❌ Invalid response")
                            continue  # Try next protocol
                    else:
                        if verbose and len(protocols_to_try) > 1:
                            print(f"❌ HTTP {response.status}")
                        continue  # Try next protocol
                        
                except urllib3.exceptions.TimeoutError:
                    if verbose and len(protocols_to_try) > 1:
                        print(f"❌ Timeout")
                    continue  # Try next protocol
                except urllib3.exceptions.ConnectTimeoutError:
                    if verbose and len(protocols_to_try) > 1:
                        print(f"❌ Connect timeout")
                    continue  # Try next protocol
                except urllib3.exceptions.ReadTimeoutError:
                    if verbose and len(protocols_to_try) > 1:
                        print(f"❌ Read timeout")
                    continue  # Try next protocol
                except urllib3.exceptions.ProxyError as e:
                    if verbose and len(protocols_to_try) > 1:
                        print(f"❌ Proxy error")
                    continue  # Try next protocol
                except urllib3.exceptions.ConnectionError as e:
                    if verbose and len(protocols_to_try) > 1:
                        print(f"❌ Connection error")
                    continue  # Try next protocol
                except Exception as e:
                    if verbose and len(protocols_to_try) > 1:
                        print(f"❌ Error: {str(e)[:30]}...")
                    continue  # Try next protocol
            
            # If we get here, all protocols failed
            return False, f"All protocols failed: {', '.join(protocols_to_try)}"
                
        except Exception as e:
            return False, f"Error: {str(e)}"

    # --------------------------- Acquire / Rotate / Latch ---------------------------

    def _acquire_next_proxy(
            self,
            require_working: bool = True,
            max_age_minutes: int = 43200,  # must be tested within last 30 days (very flexible)
    ) -> t.Optional[dict]:
        """
        Atomically pick a proxy using row lock (MySQL 8: SKIP LOCKED).
        Order: never/least recently used, then fastest latency.
        """
        cond = ["1=1"]
        if require_working:
            cond.append("working=1")
            cond.append(f"tested_timestamp >= (UTC_TIMESTAMP() - INTERVAL {int(max_age_minutes)} MINUTE)")
        where = " AND ".join(cond)

        try:
            with self._cursor() as cur:
                # Start TX
                cur.execute("SET TRANSACTION ISOLATION LEVEL READ COMMITTED")
                self._conn.begin()

                cur.execute(
                    f"""
                    SELECT id, ip, port, proxy_url, proxy_type, latency
                    FROM `{self.table}`
                    WHERE {where}
                    ORDER BY
                        (last_used_timestamp IS NULL) DESC,
                        last_used_timestamp ASC,
                        latency ASC
                    LIMIT 1
                    FOR UPDATE SKIP LOCKED
                    """
                )
                row = cur.fetchone()
                if not row:
                    self._rollback()
                    return None

                cur.execute(
                    f"""
                    UPDATE `{self.table}`
                       SET last_used_timestamp = UTC_TIMESTAMP()
                     WHERE id=%s
                    """,
                    (row["id"],),
                )
                self._commit()
                # Set the current proxy when successfully acquired
                self._current_proxy = row
                return row
        except (mysql.Error, Exception):
            self._rollback()
            return None

    def rotate_proxy(self):
        """Drop the current proxy and acquire the next eligible one."""
        self._current_proxy = None
        # Use a short time constraint since we just tested all proxies
        self._current_proxy = self._acquire_next_proxy(require_working=True, max_age_minutes=1440)  # 24 hours

    def get_current_proxy(self) -> t.Optional[dict]:
        """Return the currently latched proxy dict (or None)."""
        return self._current_proxy

    # --------------------------- Requests Session ---------------------------

    @contextmanager
    def session(self, auto_rotate_on_fail: bool = True, retries: int = 2, backoff: float = 1.5):
        """
        Context manager yielding a requests.Session bound to a latched proxy.
        Rotates and retries on first failure if desired.
        """
        if not self._current_proxy:
            self.rotate_proxy()

        s = requests.Session()

        def _bind():
            if not self._current_proxy:
                return
            proxies = _requests_proxy_mapping(self._current_proxy["proxy_url"], self._current_proxy["proxy_type"])
            s.proxies.update(proxies)

        _bind()

        try:
            yield s
        except Exception as e:
            # First-level handling: rotate and re-raise only after retries
            if not auto_rotate_on_fail or retries <= 0:
                raise

            attempt = 0
            last_exc = e
            while attempt < retries:
                attempt += 1
                # mark this proxy as suspect (optional: decrement working?) — keep simple for now
                self.rotate_proxy()
                _bind()
                time.sleep(min(5, backoff ** attempt))
                try:
                    yield s  # re-yield; caller's next call should succeed under new proxy
                    last_exc = None
                    break
                except Exception as e2:
                    last_exc = e2
                    continue
            if last_exc:
                raise last_exc
        finally:
            s.close()

    # --------------------------- Playwright ---------------------------

    async def new_playwright_context(
            self,
            playwright,  # async_playwright() handle
            browser: str = "chromium",
            headless: bool = True,
            viewport: t.Optional[dict] = None,
            **launch_kwargs,
    ):
        """
        Create a Playwright browser/context/page trio using the latched proxy.
        Example:
            async with async_playwright() as pw:
                browser, context, page = await p.new_playwright_context(pw)
                await page.goto("https://example.com")
        """
        if not self._current_proxy:
            self.rotate_proxy()
        if not self._current_proxy:
            raise RuntimeError("No proxy available to launch Playwright.")

        server = self._current_proxy["proxy_url"]
        proxy_kw = {"server": server}

        # Choose engine
        b = getattr(playwright, browser)
        browser = await b.launch(headless=headless, proxy=proxy_kw, **launch_kwargs)
        context = await browser.new_context(viewport=viewport)
        page = await context.new_page()
        return browser, context, page

    # --------------------------- Misc helpers ---------------------------

    def mark_current_bad(self, message: str = ""):
        """Optionally mark current proxy as not working (e.g., after repeated failures)."""
        if not self._current_proxy:
            return
        with self._cursor() as cur:
            cur.execute(
                f"UPDATE `{self.table}` SET working=0, error_message=%s, updated_at=UTC_TIMESTAMP() WHERE id=%s",
                (message[:500], self._current_proxy["id"]),
            )
        self._commit()

    def stats(self) -> dict:
        """Quick counts for dashboarding."""
        with self._cursor() as cur:
            cur.execute(
                f"""
                SELECT
                    COUNT(*) as total,
                    SUM(working=1) as working,
                    SUM(working=0) as failing
                FROM `{self.table}`
                """
            )
            a = cur.fetchone()
            cur.execute(
                f"""
                SELECT COUNT(*) as fresh
                FROM `{self.table}`
                WHERE tested_timestamp >= (UTC_TIMESTAMP() - INTERVAL 12 HOUR)
                """
            )
            b = cur.fetchone()
        return {
            "total": int(a["total"] or 0),
            "working": int(a["working"] or 0),
            "failing": int(a["failing"] or 0),
            "tested_last_12h": int(b["fresh"] or 0),
        }


# Optional: quick smoke test when run directly
if __name__ == "__main__":  # pragma: no cover
    import argparse

    ap = argparse.ArgumentParser(description="Proxy helper smoke test")
    ap.add_argument("--force-refresh", action="store_true")
    ap.add_argument("--test-all", action="store_true", help="Test all proxies")
    ap.add_argument("--test-working", action="store_true", help="Test only working proxies (working=1)")
    ap.add_argument("--test-failed", action="store_true", help="Test only failed proxies (working=0)")
    ap.add_argument("--test-untested", action="store_true", help="Test only untested proxies (tested_timestamp IS NULL)")
    ap.add_argument("--verbose", "-v", action="store_true", help="Verbose output showing protocol fallback testing")
    ap.add_argument("--threads", "-t", type=int, default=30, help="Number of concurrent threads for testing (default: 30)")
    ap.add_argument("--limit", "-l", type=int, help="Limit number of proxies to test (default: all)")
    ap.add_argument("--ip", type=str, help="Test a specific IP address (e.g., --ip '98.178.72.21')")
    args = ap.parse_args()

    # Use config from .env file
    from config import C
    helper = Proxy_Helper.from_config(C, force_proxy_refresh=args.force_refresh)

    print("Stats:", helper.stats())
    
    if args.ip:
        helper.test_specific_ip(args.ip, verbose=args.verbose)
    elif args.test_working:
        helper.test_working_proxies(limit=args.limit, verbose=args.verbose, max_workers=args.threads)
    elif args.test_failed:
        helper.test_failed_proxies(limit=args.limit, verbose=args.verbose, max_workers=args.threads)
    elif args.test_untested:
        helper.test_untested_proxies(limit=args.limit, verbose=args.verbose, max_workers=args.threads)
    elif args.test_all:
        helper.test_all_proxies(limit=args.limit, verbose=args.verbose, max_workers=args.threads)
    else:
        helper.rotate_proxy()
        print("Latched proxy:", helper.get_current_proxy())

        try:
            with helper.session() as s:
                r = s.get("http://ip.knws.co.uk", timeout=20)
                print("IP via proxy:", r.text)
        finally:
            helper.close()
