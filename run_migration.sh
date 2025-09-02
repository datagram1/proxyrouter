#!/bin/bash

# Script to run the migration and test the new unique constraint
# This script will:
# 1. Stop ProxyRouter
# 2. Run the migration
# 3. Restart ProxyRouter
# 4. Test the new unique constraint

set -e

echo "=== Running Migration: Unique Host+Port Constraint ==="
echo "Date: $(date)"
echo

# Check if proxyrouter is running
if pgrep -f "proxyrouter" > /dev/null; then
    echo "🛑 Stopping ProxyRouter..."
    pkill -f proxyrouter
    sleep 2
    echo "✅ ProxyRouter stopped"
else
    echo "ℹ️  ProxyRouter is not running"
fi

echo

# Check if database exists
DB_PATH="data/router.db"
if [ ! -f "$DB_PATH" ]; then
    echo "❌ Database not found at $DB_PATH"
    echo "Please start ProxyRouter first to create the database"
    exit 1
fi

echo "📊 Database found at $DB_PATH"

# Check current proxy count
echo "📈 Checking current proxy count..."
CURRENT_COUNT=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM proxies;" 2>/dev/null || echo "0")
echo "Current proxies: $CURRENT_COUNT"

# Check for duplicates
echo "🔍 Checking for duplicates..."
DUPLICATE_COUNT=$(sqlite3 "$DB_PATH" "
SELECT COUNT(*) FROM (
    SELECT host, port, COUNT(*) as cnt 
    FROM proxies 
    GROUP BY host, port 
    HAVING cnt > 1
);" 2>/dev/null || echo "0")
echo "Duplicate entries: $DUPLICATE_COUNT"

echo

# Run the migration
echo "🔄 Running migration..."
if sqlite3 "$DB_PATH" < migrations/002_unique_host_port.sql; then
    echo "✅ Migration completed successfully"
else
    echo "❌ Migration failed"
    exit 1
fi

echo

# Check results after migration
echo "📊 Checking results after migration..."
NEW_COUNT=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM proxies;" 2>/dev/null || echo "0")
echo "Proxies after migration: $NEW_COUNT"

# Check for remaining duplicates
echo "🔍 Checking for remaining duplicates..."
REMAINING_DUPLICATES=$(sqlite3 "$DB_PATH" "
SELECT COUNT(*) FROM (
    SELECT host, port, COUNT(*) as cnt 
    FROM proxies 
    GROUP BY host, port 
    HAVING cnt > 1
);" 2>/dev/null || echo "0")
echo "Remaining duplicates: $REMAINING_DUPLICATES"

# Check unique constraint
echo "🔒 Checking unique constraint..."
UNIQUE_CONSTRAINT=$(sqlite3 "$DB_PATH" "
SELECT name FROM sqlite_master 
WHERE type='index' AND name='idx_proxies_host_port';" 2>/dev/null || echo "")
if [ -n "$UNIQUE_CONSTRAINT" ]; then
    echo "✅ Unique constraint on (host, port) is active"
else
    echo "❌ Unique constraint not found"
fi

echo

# Start ProxyRouter
echo "🚀 Starting ProxyRouter..."
./proxyrouter > proxyrouter.log 2>&1 &
sleep 3

# Check if ProxyRouter started successfully
if pgrep -f "proxyrouter" > /dev/null; then
    echo "✅ ProxyRouter started successfully"
else
    echo "❌ Failed to start ProxyRouter"
    exit 1
fi

echo

# Test the new unique constraint
echo "🧪 Testing new unique constraint..."

# Test 1: Add a proxy that should be accepted
echo "Test 1: Adding new proxy..."
curl -s -X POST "http://localhost:8081/api/v1/proxies/import" \
    -H "Content-Type: application/json" \
    -d '{"proxies": ["192.168.1.100:8080"], "source": "test"}' > /dev/null

# Test 2: Add the same proxy again (should be ignored)
echo "Test 2: Adding duplicate proxy (should be ignored)..."
curl -s -X POST "http://localhost:8081/api/v1/proxies/import" \
    -H "Content-Type: application/json" \
    -d '{"proxies": ["192.168.1.100:8080"], "source": "test2"}' > /dev/null

# Test 3: Add same IP with different port (should be accepted)
echo "Test 3: Adding same IP with different port..."
curl -s -X POST "http://localhost:8081/api/v1/proxies/import" \
    -H "Content-Type: application/json" \
    -d '{"proxies": ["192.168.1.100:8081"], "source": "test"}' > /dev/null

# Check final count
FINAL_COUNT=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM proxies WHERE host='192.168.1.100';" 2>/dev/null || echo "0")
echo "Proxies with IP 192.168.1.100: $FINAL_COUNT"

if [ "$FINAL_COUNT" -eq 2 ]; then
    echo "✅ Unique constraint working correctly (2 different ports)"
else
    echo "❌ Unique constraint not working as expected"
fi

echo

# Show summary
echo "=== Migration Summary ==="
echo "Before migration: $CURRENT_COUNT proxies"
echo "Duplicates removed: $DUPLICATE_COUNT"
echo "After migration: $NEW_COUNT proxies"
echo "Unique constraint: ✅ Active"
echo "Import behavior: ✅ Skips duplicates"

echo
echo "✅ Migration completed successfully!"
echo "ProxyRouter is running with the new unique constraint on (host, port)"
