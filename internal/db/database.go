package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Database represents the database connection
type Database struct {
	db *sql.DB
}

// New creates a new database connection
func New(dbPath string) (*Database, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure SQLite
	if err := configureSQLite(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to configure SQLite: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Database{db: db}, nil
}

// configureSQLite sets up SQLite with optimal settings
func configureSQLite(db *sql.DB) error {
	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return err
	}

	// Set busy timeout
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		return err
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return err
	}

	// Set synchronous mode for better performance
	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		return err
	}

	// Set cache size
	if _, err := db.Exec("PRAGMA cache_size=10000"); err != nil {
		return err
	}

	return nil
}

// Close closes the database connection
func (d *Database) Close() error {
	return d.db.Close()
}

// GetDB returns the underlying sql.DB
func (d *Database) GetDB() *sql.DB {
	return d.db
}

// RunMigrations runs all database migrations
func (d *Database) RunMigrations(migrationsDir string) error {
	// Read migration files
	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Create migrations table if it doesn't exist
	if err := d.createMigrationsTable(); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Run migrations in order
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".sql" {
			continue
		}

		migrationPath := filepath.Join(migrationsDir, file.Name())
		if err := d.runMigration(file.Name(), migrationPath); err != nil {
			return fmt.Errorf("failed to run migration %s: %w", file.Name(), err)
		}
	}

	return nil
}

// createMigrationsTable creates the migrations tracking table
func (d *Database) createMigrationsTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS migrations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`
	
	_, err := d.db.Exec(query)
	return err
}

// runMigration runs a single migration file
func (d *Database) runMigration(name, path string) error {
	// Check if migration already applied
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM migrations WHERE name = ?", name).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check migration status: %w", err)
	}

	if count > 0 {
		// Migration already applied
		return nil
	}

	// Read migration file
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	// Start transaction
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute migration
	if _, err := tx.Exec(string(content)); err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	// Record migration as applied
	if _, err := tx.Exec("INSERT INTO migrations (name) VALUES (?)", name); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migration: %w", err)
	}

	return nil
}

// Exec executes a query with context
func (d *Database) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return d.db.ExecContext(ctx, query, args...)
}

// Query executes a query that returns rows
func (d *Database) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return d.db.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns a single row
func (d *Database) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return d.db.QueryRowContext(ctx, query, args...)
}

// Begin starts a new transaction
func (d *Database) Begin() (*sql.Tx, error) {
	return d.db.Begin()
}

// BeginTx starts a new transaction with context
func (d *Database) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return d.db.BeginTx(ctx, opts)
}

// Ping checks if the database is accessible
func (d *Database) Ping(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

// Stats returns database statistics
func (d *Database) Stats() sql.DBStats {
	return d.db.Stats()
}
