-- Migration 002: Change unique constraint to (host, port) and clean up duplicates
-- This migration:
-- 1. Removes the existing unique constraint on (scheme, host, port)
-- 2. Cleans up duplicate entries keeping the most recent one for each (host, port)
-- 3. Adds a new unique constraint on (host, port)

-- Step 1: Create a temporary table to store the deduplicated proxies
CREATE TABLE IF NOT EXISTS proxies_temp (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  scheme TEXT NOT NULL,
  host TEXT NOT NULL,
  port INTEGER NOT NULL,
  source TEXT,
  latency_ms INTEGER,
  alive INTEGER NOT NULL DEFAULT 1,
  last_checked_at DATETIME,
  expires_at DATETIME,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  error_message TEXT
);

-- Step 2: Insert deduplicated proxies, keeping the most recent entry for each (host, port)
INSERT INTO proxies_temp (scheme, host, port, source, latency_ms, alive, last_checked_at, expires_at, created_at, error_message)
SELECT 
  scheme, host, port, source, latency_ms, alive, last_checked_at, expires_at, created_at, error_message
FROM (
  SELECT 
    scheme, host, port, source, latency_ms, alive, last_checked_at, expires_at, created_at, error_message,
    ROW_NUMBER() OVER (PARTITION BY host, port ORDER BY created_at DESC) as rn
  FROM proxies
) ranked
WHERE rn = 1;

-- Step 3: Drop the old proxies table
DROP TABLE proxies;

-- Step 4: Rename the temporary table to proxies
ALTER TABLE proxies_temp RENAME TO proxies;

-- Step 5: Add the new unique constraint on (host, port)
CREATE UNIQUE INDEX idx_proxies_host_port ON proxies(host, port);

-- Step 6: Recreate the existing indexes
CREATE INDEX IF NOT EXISTS idx_proxies_alive ON proxies(alive);
CREATE INDEX IF NOT EXISTS idx_proxies_latency ON proxies(latency_ms);
CREATE INDEX IF NOT EXISTS idx_proxies_last_checked ON proxies(last_checked_at);

-- Step 7: Add a comment to document the change
-- The unique constraint is now on (host, port) instead of (scheme, host, port)
-- This means only one proxy per IP:port combination, regardless of scheme
