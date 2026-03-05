#!/bin/sh
set -e
echo "⏳ Waiting for PostgreSQL..."
until pg_isready -h postgresql -U bitengine -q; do sleep 1; done
echo "✅ PostgreSQL ready"
echo "📦 Running migrations..."
for f in /app/migrations/*.sql; do
  PGPASSWORD=bitengine psql -h postgresql -U bitengine -d bitengine -f "$f" -q 2>/dev/null || true
done
echo "✅ Migrations done"
echo "🔨 Starting air..."
exec air -c .air.toml
