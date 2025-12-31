#!/usr/bin/env bash
#
# Create a full workshop with multiple learner containers
#
# Usage: ./create-workshop.sh <workshop-id> <seats> [api-key]
#
# Environment variables:
#   WORKSPACE_IMAGE  - Docker image to use (default: clarateach-workspace)
#   JWKS_URL         - URL to fetch JWKS for token validation
#   AUTH_DISABLED    - Set to 'true' to disable auth (default: false)
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NETWORK_PREFIX="clarateach"

# Defaults
WORKSPACE_IMAGE="${WORKSPACE_IMAGE:-clarateach-workspace}"
AUTH_DISABLED="${AUTH_DISABLED:-false}"
JWKS_URL="${JWKS_URL:-}"

usage() {
  echo "Usage: $0 <workshop-id> <seats> [api-key]"
  echo ""
  echo "Arguments:"
  echo "  workshop-id   Unique identifier for the workshop (e.g., 'abc123')"
  echo "  seats         Number of seats to create (1-10)"
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
SEATS="$2"
API_KEY="${3:-}"

# Validate workshop ID
if ! [[ "$WORKSHOP_ID" =~ ^[a-z0-9-]{3,20}$ ]]; then
  echo "Error: workshop-id must be 3-20 lowercase alphanumeric characters or hyphen" >&2
  exit 1
fi

# Validate seats
if ! [[ "$SEATS" =~ ^[1-9][0-9]*$ ]] || [[ "$SEATS" -gt 10 ]]; then
  echo "Error: seats must be a positive integer between 1 and 10" >&2
  exit 1
fi

NETWORK_NAME="${NETWORK_PREFIX}-${WORKSHOP_ID}"

echo "Creating workshop: ${WORKSHOP_ID}"
echo "  Seats: ${SEATS}"
echo "  Image: ${WORKSPACE_IMAGE}"
echo "  Auth:  ${AUTH_DISABLED:+disabled}${AUTH_DISABLED:-enabled}"
echo ""

# Create network first
if ! docker network inspect "$NETWORK_NAME" &>/dev/null; then
  echo "Creating network: ${NETWORK_NAME}"
  docker network create "$NETWORK_NAME" \
    --driver bridge \
    --label "clarateach.type=workshop-network" \
    --label "clarateach.workshop=${WORKSHOP_ID}"
fi

# Create containers
CREATED=0
FAILED=0

for ((seat=1; seat<=SEATS; seat++)); do
  echo ""
  echo "Creating seat ${seat}/${SEATS}..."

  if "$SCRIPT_DIR/create-container.sh" "$WORKSHOP_ID" "$seat" "$API_KEY"; then
    CREATED=$((CREATED + 1))
  else
    echo "Warning: Failed to create seat ${seat}" >&2
    FAILED=$((FAILED + 1))
  fi
done

echo ""
echo "=========================================="
echo "Workshop creation complete!"
echo "  Workshop ID: ${WORKSHOP_ID}"
echo "  Network:     ${NETWORK_NAME}"
echo "  Created:     ${CREATED}/${SEATS} containers"
if [[ "$FAILED" -gt 0 ]]; then
  echo "  Failed:      ${FAILED} containers"
  exit 1
fi
echo ""
echo "List containers with:"
echo "  ./list-containers.sh ${WORKSHOP_ID}"
echo ""
echo "Destroy workshop with:"
echo "  ./destroy-workshop.sh ${WORKSHOP_ID}"
