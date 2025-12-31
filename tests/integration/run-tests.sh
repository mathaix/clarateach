#!/usr/bin/env bash
#
# Run ClaraTeach integration tests
#
# Prerequisites:
#   - Services must be running: ./scripts/stack.sh start
#   - Docker must be running
#
# Usage:
#   ./run-tests.sh              # Run all tests
#   ./run-tests.sh --watch      # Run in watch mode
#   ./run-tests.sh <pattern>    # Run specific test file

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$(dirname "$SCRIPT_DIR")")"

cd "$SCRIPT_DIR"

# Load configuration from .env file (optional)
ENV_FILE="$ROOT_DIR/.env"
if [ -f "$ENV_FILE" ]; then
  # shellcheck disable=SC1090
  source "$ENV_FILE"
fi

PORTAL_PORT="${PORTAL_PORT:-3000}"
CADDY_PORT="${CADDY_PORT:-8000}"
PORTAL_URL="${PORTAL_URL:-http://localhost:$PORTAL_PORT}"
WORKSPACE_URL="${WORKSPACE_URL:-http://localhost:$CADDY_PORT}"
export PORTAL_URL WORKSPACE_URL

# Check if services are running
echo "Checking if services are running..."

if ! curl -sf "$PORTAL_URL/api/health" > /dev/null 2>&1; then
  echo "Error: Portal API is not running at $PORTAL_URL"
  echo "Start services with: ./scripts/stack.sh start"
  exit 1
fi

if ! curl -sf "$WORKSPACE_URL/health" > /dev/null 2>&1; then
  echo "Error: Caddy proxy is not running at $WORKSPACE_URL"
  echo "Start services with: ./scripts/stack.sh start"
  exit 1
fi

echo "Services are running!"
echo ""

# Install dependencies if needed
if [ ! -d "node_modules" ]; then
  echo "Installing dependencies..."
  npm install
  echo ""
fi

# Run tests
echo "Running integration tests..."
echo ""

if [ "${1:-}" = "--watch" ]; then
  npx vitest --watch
elif [ -n "${1:-}" ]; then
  npx vitest run "$1"
else
  npx vitest run
fi
