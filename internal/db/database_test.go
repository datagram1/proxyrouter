package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	// Create a temporary directory for the database
	tmpDir, err := os.MkdirTemp("", "proxyrouter-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")

	// Test creating a new database
	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Test that the database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}

	// Test that we can ping the database
	if err := db.Ping(context.Background()); err != nil {
		t.Errorf("Failed to ping database: %v", err)
	}
}

func TestNewWithNonExistentDirectory(t *testing.T) {
	// Test creating database in non-existent directory
	dbPath := "/tmp/nonexistent/dir/test.db"

	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Test that the directory was created
	dir := filepath.Dir(dbPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("Directory was not created")
	}

	// Clean up
	os.RemoveAll(dir)
}

func TestRunMigrations(t *testing.T) {
	// Create a temporary directory for the database
	tmpDir, err := os.MkdirTemp("", "proxyrouter-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	migrationsDir := filepath.Join(tmpDir, "migrations")

	// Create migrations directory
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a test migration file
	migrationContent := `
CREATE TABLE IF NOT EXISTS test_table (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO test_table (name) VALUES ('test');
`

	migrationFile := filepath.Join(migrationsDir, "001_test_migration.sql")
	if err := os.WriteFile(migrationFile, []byte(migrationContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create database and run migrations
	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	if err := db.RunMigrations(migrationsDir); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Test that the migration was applied
	var count int
	err = db.QueryRow(context.Background(), "SELECT COUNT(*) FROM test_table").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query test table: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 row in test table, got %d", count)
	}

	// Test that the migration is recorded
	err = db.QueryRow(context.Background(), "SELECT COUNT(*) FROM migrations WHERE name = ?", "001_test_migration.sql").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query migrations table: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 migration record, got %d", count)
	}
}

func TestRunMigrationsTwice(t *testing.T) {
	// Create a temporary directory for the database
	tmpDir, err := os.MkdirTemp("", "proxyrouter-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	migrationsDir := filepath.Join(tmpDir, "migrations")

	// Create migrations directory
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a test migration file
	migrationContent := `
CREATE TABLE IF NOT EXISTS test_table (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL
);
`

	migrationFile := filepath.Join(migrationsDir, "001_test_migration.sql")
	if err := os.WriteFile(migrationFile, []byte(migrationContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create database
	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Run migrations twice
	if err := db.RunMigrations(migrationsDir); err != nil {
		t.Fatalf("Failed to run migrations first time: %v", err)
	}

	if err := db.RunMigrations(migrationsDir); err != nil {
		t.Fatalf("Failed to run migrations second time: %v", err)
	}

	// Test that the migration is only recorded once
	var count int
	err = db.QueryRow(context.Background(), "SELECT COUNT(*) FROM migrations WHERE name = ?", "001_test_migration.sql").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query migrations table: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 migration record, got %d", count)
	}
}

func TestDatabaseOperations(t *testing.T) {
	// Create a temporary directory for the database
	tmpDir, err := os.MkdirTemp("", "proxyrouter-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")

	// Create database
	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Test Exec
	result, err := db.Exec(context.Background(), "CREATE TABLE test_table (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("Failed to execute CREATE TABLE: %v", err)
	}

	// Test Exec with parameters
	result, err = db.Exec(context.Background(), "INSERT INTO test_table (name) VALUES (?)", "test")
	if err != nil {
		t.Fatalf("Failed to execute INSERT: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("Failed to get rows affected: %v", err)
	}

	if rowsAffected != 1 {
		t.Errorf("Expected 1 row affected, got %d", rowsAffected)
	}

	// Test Query
	rows, err := db.Query(context.Background(), "SELECT id, name FROM test_table")
	if err != nil {
		t.Fatalf("Failed to execute SELECT: %v", err)
	}
	defer rows.Close()

	var id int
	var name string
	if rows.Next() {
		if err := rows.Scan(&id, &name); err != nil {
			t.Fatalf("Failed to scan row: %v", err)
		}

		if name != "test" {
			t.Errorf("Expected name 'test', got '%s'", name)
		}
	} else {
		t.Error("Expected to find a row")
	}

	// Test QueryRow
	err = db.QueryRow(context.Background(), "SELECT COUNT(*) FROM test_table").Scan(&id)
	if err != nil {
		t.Fatalf("Failed to execute SELECT COUNT: %v", err)
	}

	if id != 1 {
		t.Errorf("Expected count 1, got %d", id)
	}
}

func TestTransaction(t *testing.T) {
	// Create a temporary directory for the database
	tmpDir, err := os.MkdirTemp("", "proxyrouter-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")

	// Create database
	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create test table
	_, err = db.Exec(context.Background(), "CREATE TABLE test_table (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	// Test successful transaction
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	_, err = tx.Exec("INSERT INTO test_table (name) VALUES (?)", "test1")
	if err != nil {
		t.Fatalf("Failed to execute INSERT in transaction: %v", err)
	}

	_, err = tx.Exec("INSERT INTO test_table (name) VALUES (?)", "test2")
	if err != nil {
		t.Fatalf("Failed to execute second INSERT in transaction: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	// Verify both rows were inserted
	var count int
	err = db.QueryRow(context.Background(), "SELECT COUNT(*) FROM test_table").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query count: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 rows, got %d", count)
	}

	// Test rollback
	tx, err = db.Begin()
	if err != nil {
		t.Fatalf("Failed to begin second transaction: %v", err)
	}

	_, err = tx.Exec("INSERT INTO test_table (name) VALUES (?)", "test3")
	if err != nil {
		t.Fatalf("Failed to execute INSERT in second transaction: %v", err)
	}

	tx.Rollback()

	// Verify the row was not inserted
	err = db.QueryRow(context.Background(), "SELECT COUNT(*) FROM test_table").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query count after rollback: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 rows after rollback, got %d", count)
	}
}

func TestStats(t *testing.T) {
	// Create a temporary directory for the database
	tmpDir, err := os.MkdirTemp("", "proxyrouter-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")

	// Create database
	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Get stats
	stats := db.Stats()

	// Basic stats validation
	if stats.MaxOpenConnections < 0 {
		t.Error("MaxOpenConnections should be non-negative")
	}

	if stats.OpenConnections < 0 {
		t.Error("OpenConnections should be non-negative")
	}

	if stats.InUse < 0 {
		t.Error("InUse should be non-negative")
	}

	if stats.Idle < 0 {
		t.Error("Idle should be non-negative")
	}
}
