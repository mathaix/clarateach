#!/usr/bin/env bash
#
# Create a learner container for a specific workshop and seat
#
# Usage: ./create-container.sh <workshop-id> <seat> [api-key]
#
# Environment variables:
#   WORKSPACE_IMAGE  - Docker image to use (default: clarateach-workspace)
#   JWKS_URL         - URL to fetch JWKS for token validation
#   AUTH_DISABLED    - Set to 'true' to disable auth (default: false)
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKSPACE_DIR="$(dirname "$SCRIPT_DIR")"

# Defaults
WORKSPACE_IMAGE="${WORKSPACE_IMAGE:-clarateach-workspace}"
AUTH_DISABLED="${AUTH_DISABLED:-false}"
JWKS_URL="${JWKS_URL:-}"
NETWORK_PREFIX="clarateach"
# For local dev, also connect to Caddy's network
CADDY_NETWORK="${CADDY_NETWORK:-workspace_clarateach-net}"

usage() {
  echo "Usage: $0 <workshop-id> <seat> [api-key]"
  echo ""
  echo "Arguments:"
  echo "  workshop-id   Unique identifier for the workshop (e.g., 'abc123')"
  echo "  seat          Seat number (1-based, e.g., 1, 2, 3)"
  echo "  api-key       Optional Anthropic API key for Claude CLI"
  echo ""
  echo "Environment variables:"
  echo "  WORKSPACE_IMAGE  Docker image to use (default: clarateach-workspace)"
  echo "  JWKS_URL         URL to fetch JWKS for token validation"
  echo "  AUTH_DISABLED    Set to 'true' to disable auth (default: false)"
  exit 1
}

# Validate arguments
if [[ $# -lt 2 ]]; then
  usage
fi

WORKSHOP_ID="$1"
SEAT="$2"
API_KEY="${3:-}"

# Validate workshop ID (lowercase alphanumeric + hyphen, 3-20 chars)
if ! [[ "$WORKSHOP_ID" =~ ^[a-z0-9-]{3,20}$ ]]; then
  echo "Error: workshop-id must be 3-20 lowercase alphanumeric characters or hyphen" >&2
  exit 1
fi

# Validate seat number (1-10)
if ! [[ "$SEAT" =~ ^[1-9][0-9]*$ ]] || [[ "$SEAT" -gt 10 ]]; then
  echo "Error: seat must be an integer between 1 and 10" >&2
  exit 1
fi

# Container and network names
CONTAINER_NAME="clarateach-${WORKSHOP_ID}-${SEAT}"
NETWORK_NAME="${NETWORK_PREFIX}-${WORKSHOP_ID}"
VOLUME_NAME="${CONTAINER_NAME}-data"

# Check if container already exists
if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
  echo "Error: Container ${CONTAINER_NAME} already exists" >&2
  echo "Use destroy-container.sh to remove it first" >&2
  exit 1
fi

# Create network if it doesn't exist
if ! docker network inspect "$NETWORK_NAME" &>/dev/null; then
  echo "Creating network: ${NETWORK_NAME}"
  docker network create "$NETWORK_NAME" \
    --driver bridge \
    --label "clarateach.type=workshop-network" \
    --label "clarateach.workshop=${WORKSHOP_ID}"
fi

# Create volume for workspace persistence
if ! docker volume inspect "$VOLUME_NAME" &>/dev/null; then
  echo "Creating volume: ${VOLUME_NAME}"
  docker volume create "$VOLUME_NAME" \
    --label "clarateach.type=workspace-data" \
    --label "clarateach.workshop=${WORKSHOP_ID}" \
    --label "clarateach.seat=${SEAT}"
fi

# Build environment variables
SEAT_PADDED=$(printf '%02d' "$SEAT")
CONTAINER_ID_ENV="c-${SEAT_PADDED}"

ENV_ARGS=(
  -e "WORKSPACE_DIR=/workspace"
  -e "TERM=xterm-256color"
  -e "TERMINAL_PORT=3001"
  -e "FILES_PORT=3002"
  -e "AUTH_DISABLED=${AUTH_DISABLED}"
  -e "SEAT=${SEAT}"
  -e "CONTAINER_ID=${CONTAINER_ID_ENV}"
)

if [[ -n "$JWKS_URL" ]]; then
  ENV_ARGS+=(-e "JWKS_URL=${JWKS_URL}")
fi

if [[ -n "$API_KEY" ]]; then
  ENV_ARGS+=(-e "ANTHROPIC_API_KEY=${API_KEY}")
fi

# Create the container
echo "Creating container: ${CONTAINER_NAME}"
docker run -d \
  --name "$CONTAINER_NAME" \
  --hostname "seat-${SEAT}" \
  --network "$NETWORK_NAME" \
  --restart unless-stopped \
  --volume "${VOLUME_NAME}:/workspace" \
  --label "clarateach.type=learner-workspace" \
  --label "clarateach.workshop=${WORKSHOP_ID}" \
  --label "clarateach.seat=${SEAT}" \
  "${ENV_ARGS[@]}" \
  "$WORKSPACE_IMAGE" \
  node /home/learner/server/dist/index.js

# Connect to Caddy network if it exists (for local dev routing)
if docker network inspect "$CADDY_NETWORK" &>/dev/null; then
  echo "Connecting to Caddy network: ${CADDY_NETWORK}"
  docker network connect "$CADDY_NETWORK" "$CONTAINER_NAME" --alias "workspace-${SEAT}"
fi

# Get container info
CONTAINER_ID=$(docker inspect --format='{{.Id}}' "$CONTAINER_NAME")
CONTAINER_IP=$(docker inspect --format "{{(index .NetworkSettings.Networks \"${NETWORK_NAME}\").IPAddress}}" "$CONTAINER_NAME")

echo ""
echo "Container created successfully!"
echo "  Name:      ${CONTAINER_NAME}"
echo "  ID:        ${CONTAINER_ID:0:12}"
echo "  Network:   ${NETWORK_NAME}"
echo "  IP:        ${CONTAINER_IP}"
echo "  Volume:    ${VOLUME_NAME}"
echo ""
echo "Terminal server: http://${CONTAINER_IP}:3001"
echo "File server:     http://${CONTAINER_IP}:3002"

# Output JSON for programmatic use
cat <<EOF
{"name":"${CONTAINER_NAME}","id":"${CONTAINER_ID}","ip":"${CONTAINER_IP}","network":"${NETWORK_NAME}","volume":"${VOLUME_NAME}","port":3001}
EOF
