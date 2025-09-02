-- Migration 009: Rename fields and add proxy_url in proxies table
-- This migration:
-- 1. Renames 'latency_ms' to 'latency'
-- 2. Renames 'alive' to 'working'
-- 3. Renames 'last_checked_at' to 'tested_timestamp'
-- 4. Adds new 'proxy_url' field to store original import format

-- SQLite doesn't support ALTER TABLE RENAME COLUMN in older versions, so we need to recreate the table
-- Step 1: Create a temporary table with the new column names
CREATE TABLE IF NOT EXISTS proxies_temp (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  proxy_type TEXT NOT NULL,         -- "socks5" | "http" | "https"
  ip TEXT NOT NULL,                 -- IP address
  port INTEGER NOT NULL,
  source TEXT,                      -- e.g., "spys.one-gb" or "manual"
  latency INTEGER,                  -- renamed from 'latency_ms'
  working INTEGER NOT NULL DEFAULT 1, -- renamed from 'alive'
  tested_timestamp DATETIME,         -- renamed from 'last_checked_at'
  expires_at DATETIME,              -- null = persistent
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  error_message TEXT,
  proxy_url TEXT                    -- new field: original import format
);

-- Step 2: Copy data from the old table to the new table
INSERT INTO proxies_temp (id, proxy_type, ip, port, source, latency, working, tested_timestamp, expires_at, created_at, error_message, proxy_url)
SELECT 
  id, 
  proxy_type, 
  ip, 
  port, 
  source, 
  latency_ms, 
  alive, 
  last_checked_at, 
  expires_at, 
  created_at, 
  error_message,
  NULL -- proxy_url will be NULL for existing records
FROM proxies;

-- Step 3: Drop the old proxies table
DROP TABLE proxies;

-- Step 4: Rename the temporary table to proxies
ALTER TABLE proxies_temp RENAME TO proxies;

-- Step 5: Recreate the unique constraint and indexes
CREATE UNIQUE INDEX idx_proxies_ip_port ON proxies(ip, port);
CREATE INDEX IF NOT EXISTS idx_proxies_working ON proxies(working);
CREATE INDEX IF NOT EXISTS idx_proxies_latency ON proxies(latency);
CREATE INDEX IF NOT EXISTS idx_proxies_tested_timestamp ON proxies(tested_timestamp);

-- Step 6: Add a comment to document the changes
-- The following fields have been renamed for better clarity:
-- - 'latency_ms' -> 'latency' (milliseconds)
-- - 'alive' -> 'working' (boolean, 1=working, 0=not working)
-- - 'last_checked_at' -> 'tested_timestamp' (when last health check was performed)
-- - Added 'proxy_url' field to store original import format (e.g., "socks5://40.172.232.213:13279")
