#!/usr/bin/env bash
#
# Destroy a learner container and optionally its data volume
#
# Usage: ./destroy-container.sh <workshop-id> <seat> [--keep-data]
#
# Options:
#   --keep-data   Don't delete the workspace data volume
#
set -euo pipefail

NETWORK_PREFIX="clarateach"

usage() {
  echo "Usage: $0 <workshop-id> <seat> [--keep-data]"
  echo ""
  echo "Arguments:"
  echo "  workshop-id   Unique identifier for the workshop"
  echo "  seat          Seat number (1-based)"
  echo ""
  echo "Options:"
  echo "  --keep-data   Don't delete the workspace data volume"
  exit 1
}

# Parse arguments
if [[ $# -lt 2 ]]; then
  usage
fi

WORKSHOP_ID="$1"
SEAT="$2"
KEEP_DATA=false

if [[ "${3:-}" == "--keep-data" ]]; then
  KEEP_DATA=true
fi

# Container and resource names
CONTAINER_NAME="clarateach-${WORKSHOP_ID}-${SEAT}"
VOLUME_NAME="${CONTAINER_NAME}-data"
NETWORK_NAME="${NETWORK_PREFIX}-${WORKSHOP_ID}"

# Check if container exists
if ! docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
  echo "Warning: Container ${CONTAINER_NAME} does not exist" >&2
  exit 0
fi

# Stop and remove container
echo "Stopping container: ${CONTAINER_NAME}"
docker stop "$CONTAINER_NAME" 2>/dev/null || true

echo "Removing container: ${CONTAINER_NAME}"
docker rm "$CONTAINER_NAME"

# Remove volume unless --keep-data
if [[ "$KEEP_DATA" == "false" ]]; then
  if docker volume inspect "$VOLUME_NAME" &>/dev/null; then
    echo "Removing volume: ${VOLUME_NAME}"
    docker volume rm "$VOLUME_NAME"
  fi
else
  echo "Keeping volume: ${VOLUME_NAME}"
fi

# Check if network has any remaining containers
REMAINING=$(docker network inspect "$NETWORK_NAME" --format='{{len .Containers}}' 2>/dev/null || echo "0")

if [[ "$REMAINING" == "0" ]]; then
  echo "Removing empty network: ${NETWORK_NAME}"
  docker network rm "$NETWORK_NAME" 2>/dev/null || true
fi

echo ""
echo "Container destroyed successfully!"
echo "  Name:   ${CONTAINER_NAME}"
echo "  Volume: ${KEEP_DATA:+kept}${KEEP_DATA:-removed}"
