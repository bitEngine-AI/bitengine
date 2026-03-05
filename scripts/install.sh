#!/usr/bin/env bash
set -euo pipefail

# Colors
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info() { echo -e "${GREEN}[BitEngine]${NC} $1"; }
warn() { echo -e "${YELLOW}[BitEngine]${NC} $1"; }
error() { echo -e "${RED}[BitEngine]${NC} $1"; exit 1; }

# Check Docker
command -v docker &>/dev/null || error "Docker not found. Install: https://docs.docker.com/get-docker/"
docker info &>/dev/null || error "Docker daemon not running."
info "Docker detected ✓"

# Check Docker Compose
if docker compose version &>/dev/null; then
  COMPOSE="docker compose"
elif command -v docker-compose &>/dev/null; then
  COMPOSE="docker-compose"
else
  error "Docker Compose not found."
fi
info "Docker Compose detected ✓"

# Create install directory
INSTALL_DIR="${BITENGINE_DIR:-$HOME/bitengine}"
mkdir -p "$INSTALL_DIR"
cd "$INSTALL_DIR"
info "Installing to $INSTALL_DIR"

# Generate secrets
JWT_SECRET=$(openssl rand -hex 32)

# Write .env
cat > .env <<EOF
BITENGINE_DATABASE_URL=postgres://bitengine:bitengine@db:5432/bitengine?sslmode=disable
BITENGINE_REDIS_URL=redis://redis:6379/0
BITENGINE_JWT_SECRET=${JWT_SECRET}
BITENGINE_OLLAMA_URL=http://ollama:11434
BITENGINE_LISTEN_ADDR=:9000
BITENGINE_CADDY_ADMIN_URL=http://localhost:2019
BITENGINE_BASE_DOMAIN=bit.local
EOF

# Write docker-compose.yml
cat > docker-compose.yml <<'COMPOSE'
version: "3.8"
services:
  bitengined:
    image: ghcr.io/bitengine-ai/bitengine:latest
    ports:
      - "9000:9000"
    env_file: .env
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - app-data:/data
    depends_on:
      db:
        condition: service_healthy
      redis:
        condition: service_healthy
    restart: unless-stopped

  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: bitengine
      POSTGRES_PASSWORD: bitengine
      POSTGRES_DB: bitengine
    volumes:
      - pg-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U bitengine"]
      interval: 5s
      timeout: 3s
      retries: 5
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 5
    restart: unless-stopped

  ollama:
    image: ollama/ollama:latest
    volumes:
      - ollama-data:/root/.ollama
    restart: unless-stopped

volumes:
  pg-data:
  app-data:
  ollama-data:
COMPOSE

# Start services
info "Starting services..."
$COMPOSE up -d

# Wait for bitengined
info "Waiting for BitEngine to start..."
for i in $(seq 1 30); do
  if curl -sf http://localhost:9000/api/v1/system/status &>/dev/null; then
    break
  fi
  sleep 2
done

# Pull AI models in background
info "Pulling AI models (background)..."
$COMPOSE exec -d ollama ollama pull qwen3:4b

echo ""
info "========================================="
info "  BitEngine is running!"
info "  Open: http://localhost:9000"
info "  First visit will show setup wizard."
info "========================================="
