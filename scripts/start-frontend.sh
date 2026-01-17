#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FRONTEND_DIR="$ROOT_DIR/frontend"

# Load .env if exists
if [ -f "$ROOT_DIR/.env" ]; then
  set -a
  source "$ROOT_DIR/.env"
  set +a
fi

export FRONTEND_PORT="${FRONTEND_PORT:-5173}"

# Install dependencies if needed
if [ ! -d "$FRONTEND_DIR/node_modules" ]; then
  echo "Installing frontend dependencies..."
  cd "$FRONTEND_DIR"
  npm install
fi

echo "Starting frontend on port $FRONTEND_PORT..."
echo "  Access locally: http://localhost:$FRONTEND_PORT"
echo "  Access externally: http://$(curl -s ifconfig.me 2>/dev/null || echo '<external-ip>'):$FRONTEND_PORT"
echo ""

cd "$FRONTEND_DIR"
npm run dev -- --host 0.0.0.0
