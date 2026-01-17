#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"

# Load .env if exists
if [ -f "$ROOT_DIR/.env" ]; then
  set -a
  source "$ROOT_DIR/.env"
  set +a
fi

# Defaults
export GCP_PROJECT="${GCP_PROJECT:-clarateach}"
export GCP_ZONE="${GCP_ZONE:-us-central1-b}"
export GCP_REGISTRY="${GCP_REGISTRY:-us-central1-docker.pkg.dev/clarateach/clarateach}"
export AUTH_DISABLED="${AUTH_DISABLED:-true}"
export PORT="${PORT:-8080}"

# Firecracker provisioner config
export FC_SNAPSHOT_NAME="${FC_SNAPSHOT_NAME:-clarateach-agent-20260116-204056}"

echo "Starting backend on port $PORT..."
echo "  GCP_PROJECT=$GCP_PROJECT"
echo "  GCP_ZONE=$GCP_ZONE"
echo "  FC_SNAPSHOT_NAME=$FC_SNAPSHOT_NAME"
echo "  AUTH_DISABLED=$AUTH_DISABLED"
echo ""

cd "$BACKEND_DIR"
go run ./cmd/server/
