-- Initial schema for ProxyRouter
-- Create tables for proxies, routes, ACL, and settings

CREATE TABLE IF NOT EXISTS proxies (
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

CREATE TABLE IF NOT EXISTS routes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  client_cidr TEXT,                 -- e.g. "192.168.10.0/24" (nullable = any client)
  host_glob TEXT,                   -- e.g. "*.github.com" (nullable = any host)
  "group" TEXT NOT NULL,              -- "LOCAL"|"GENERAL"|"TOR"|"UPSTREAM"
  proxy_id INTEGER,                 -- used when group="UPSTREAM"
  precedence INTEGER NOT NULL DEFAULT 100,
  enabled INTEGER NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS acl_subnets (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  cidr TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for better performance
CREATE INDEX IF NOT EXISTS idx_proxies_alive ON proxies(alive);
CREATE INDEX IF NOT EXISTS idx_proxies_latency ON proxies(latency_ms);
CREATE INDEX IF NOT EXISTS idx_proxies_last_checked ON proxies(last_checked_at);
CREATE INDEX IF NOT EXISTS idx_routes_precedence ON routes(precedence);
CREATE INDEX IF NOT EXISTS idx_routes_enabled ON routes(enabled);
CREATE INDEX IF NOT EXISTS idx_routes_group ON routes("group");
