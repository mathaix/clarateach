#!/usr/bin/env bash
#
# Destroy all containers for a workshop
#
# Usage: ./destroy-workshop.sh <workshop-id> [--keep-data]
#
# Options:
#   --keep-data   Don't delete the workspace data volumes
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NETWORK_PREFIX="clarateach"

usage() {
  echo "Usage: $0 <workshop-id> [--keep-data]"
  echo ""
  echo "Arguments:"
  echo "  workshop-id   Unique identifier for the workshop"
  echo ""
  echo "Options:"
  echo "  --keep-data   Don't delete the workspace data volumes"
  exit 1
}

if [[ $# -lt 1 ]]; then
  usage
fi

WORKSHOP_ID="$1"
KEEP_DATA_FLAG=""

if [[ "${2:-}" == "--keep-data" ]]; then
  KEEP_DATA_FLAG="--keep-data"
fi

NETWORK_NAME="${NETWORK_PREFIX}-${WORKSHOP_ID}"

# Find all containers for this workshop
CONTAINERS=$(docker ps -a --filter "label=clarateach.workshop=${WORKSHOP_ID}" --format '{{.Names}}' 2>/dev/null || echo "")

if [[ -z "$CONTAINERS" ]]; then
  echo "No containers found for workshop: ${WORKSHOP_ID}"
  exit 0
fi

echo "Destroying all containers for workshop: ${WORKSHOP_ID}"
echo ""

# Destroy each container
while read -r container_name; do
  if [[ -z "$container_name" ]]; then
    continue
  fi

  # Extract seat number from container name (format: clarateach-{workshop}-{seat})
  seat="${container_name##*-}"

  if [[ "$seat" =~ ^[0-9]+$ ]]; then
    echo "Destroying seat ${seat}..."
    "$SCRIPT_DIR/destroy-container.sh" "$WORKSHOP_ID" "$seat" $KEEP_DATA_FLAG
    echo ""
  fi
done <<< "$CONTAINERS"

# Remove network if it still exists
if docker network inspect "$NETWORK_NAME" &>/dev/null; then
  echo "Removing network: ${NETWORK_NAME}"
  docker network rm "$NETWORK_NAME" 2>/dev/null || true
fi

echo ""
echo "Workshop ${WORKSHOP_ID} destroyed successfully!"
