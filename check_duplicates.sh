#!/bin/bash

# Script to check for duplicate IP:port combinations in the database

set -e

echo "=== Checking for Duplicate IP:Port Combinations ==="
echo "Date: $(date)"
echo

# Check if database exists
DB_PATH="data/router.db"
if [ ! -f "$DB_PATH" ]; then
    echo "❌ Database not found at $DB_PATH"
    echo "Please start ProxyRouter first to create the database"
    exit 1
fi

echo "📊 Database found at $DB_PATH"

# Check total proxy count
echo "📈 Total proxies in database..."
TOTAL_COUNT=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM proxies;" 2>/dev/null || echo "0")
echo "Total proxies: $TOTAL_COUNT"

echo

# Check for duplicates
echo "🔍 Checking for duplicate IP:port combinations..."
DUPLICATE_QUERY="
SELECT 
    host, 
    port, 
    COUNT(*) as count,
    GROUP_CONCAT(scheme || ':' || source || ':' || id) as details
FROM proxies 
GROUP BY host, port 
HAVING count > 1
ORDER BY count DESC, host, port
LIMIT 20;
"

DUPLICATE_COUNT=$(sqlite3 "$DB_PATH" "
SELECT COUNT(*) FROM (
    SELECT host, port, COUNT(*) as cnt 
    FROM proxies 
    GROUP BY host, port 
    HAVING cnt > 1
);" 2>/dev/null || echo "0")

echo "Duplicate IP:port combinations found: $DUPLICATE_COUNT"

if [ "$DUPLICATE_COUNT" -gt 0 ]; then
    echo
    echo "📋 Sample duplicates (showing first 20):"
    echo "IP:Port | Count | Details (scheme:source:id)"
    echo "--------|-------|-------------------------"
    sqlite3 "$DB_PATH" "$DUPLICATE_QUERY" 2>/dev/null | while IFS='|' read -r host port count details; do
        printf "%-15s | %-5s | %s\n" "$host:$port" "$count" "$details"
    done
else
    echo "✅ No duplicates found!"
fi

echo

# Check unique constraint status
echo "🔒 Current unique constraint status..."
CURRENT_CONSTRAINT=$(sqlite3 "$DB_PATH" "
SELECT sql FROM sqlite_master 
WHERE type='table' AND name='proxies';" 2>/dev/null | grep -i "unique" || echo "No unique constraint found")

if echo "$CURRENT_CONSTRAINT" | grep -q "scheme.*host.*port"; then
    echo "Current constraint: (scheme, host, port)"
elif echo "$CURRENT_CONSTRAINT" | grep -q "host.*port"; then
    echo "Current constraint: (host, port) ✅"
else
    echo "No unique constraint found"
fi

echo

# Show some statistics
echo "📊 Statistics:"
echo "Unique IP addresses: $(sqlite3 "$DB_PATH" "SELECT COUNT(DISTINCT host) FROM proxies;" 2>/dev/null || echo "0")"
echo "Unique ports: $(sqlite3 "$DB_PATH" "SELECT COUNT(DISTINCT port) FROM proxies;" 2>/dev/null || echo "0")"
echo "Unique IP:port combinations: $(sqlite3 "$DB_PATH" "SELECT COUNT(DISTINCT host || ':' || port) FROM proxies;" 2>/dev/null || echo "0")"

echo

if [ "$DUPLICATE_COUNT" -gt 0 ]; then
    echo "⚠️  Duplicates found! Run the migration to clean them up:"
    echo "   ./run_migration.sh"
else
    echo "✅ Database is clean - no duplicates found"
fi
