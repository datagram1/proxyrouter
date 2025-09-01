-- Settings table for runtime-editable configuration
CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Insert default settings
INSERT OR IGNORE INTO settings (key, value) VALUES
  ('session_secret', ''),
  ('proxy_sources', '[]'),
  ('refresh_interval_sec', '900'),
  ('health_check_interval_sec', '300'),
  ('max_upload_size_mb', '10');
