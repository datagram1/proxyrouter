-- Migration 007: Rename 'scheme' column to 'proxy_type' in proxies table
-- This migration renames the scheme column to proxy_type for better clarity

-- SQLite doesn't support ALTER TABLE RENAME COLUMN in older versions, so we need to recreate the table
-- Step 1: Create a temporary table with the new column name
CREATE TABLE IF NOT EXISTS proxies_temp (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  proxy_type TEXT NOT NULL,         -- renamed from 'scheme'
  host TEXT NOT NULL,
  port INTEGER NOT NULL,
  source TEXT,                      -- e.g., "spys.one-gb" or "manual"
  latency_ms INTEGER,
  alive INTEGER NOT NULL DEFAULT 1,
  last_checked_at DATETIME,
  expires_at DATETIME,              -- null = persistent
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  error_message TEXT
);

-- Step 2: Copy data from the old table to the new table
INSERT INTO proxies_temp (id, proxy_type, host, port, source, latency_ms, alive, last_checked_at, expires_at, created_at, error_message)
SELECT id, scheme, host, port, source, latency_ms, alive, last_checked_at, expires_at, created_at, error_message
FROM proxies;

-- Step 3: Drop the old proxies table
DROP TABLE proxies;

-- Step 4: Rename the temporary table to proxies
ALTER TABLE proxies_temp RENAME TO proxies;

-- Step 5: Recreate the unique constraint and indexes
CREATE UNIQUE INDEX idx_proxies_host_port ON proxies(host, port);
CREATE INDEX IF NOT EXISTS idx_proxies_alive ON proxies(alive);
CREATE INDEX IF NOT EXISTS idx_proxies_latency ON proxies(latency_ms);
CREATE INDEX IF NOT EXISTS idx_proxies_last_checked ON proxies(last_checked_at);

-- Step 6: Add a comment to document the change
-- The 'scheme' column has been renamed to 'proxy_type' for better clarity
-- Values remain the same: "socks5" | "http" | "https"
