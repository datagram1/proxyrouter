-- Add error_message column to proxies table
-- This column stores error messages from health checks

ALTER TABLE proxies ADD COLUMN error_message TEXT;
