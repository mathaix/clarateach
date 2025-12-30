#!/usr/bin/env bash
#
# List all learner containers, optionally filtered by workshop
#
# Usage: ./list-containers.sh [workshop-id] [--json]
#
# Options:
#   --json    Output in JSON format
#
set -euo pipefail

usage() {
  echo "Usage: $0 [workshop-id] [--json]"
  echo ""
  echo "Arguments:"
  echo "  workshop-id   Optional: filter by workshop ID"
  echo ""
  echo "Options:"
  echo "  --json        Output in JSON format"
  exit 1
}

# Parse arguments
WORKSHOP_ID=""
JSON_OUTPUT=false

for arg in "$@"; do
  case "$arg" in
    --json)
      JSON_OUTPUT=true
      ;;
    --help|-h)
      usage
      ;;
    *)
      if [[ -z "$WORKSHOP_ID" ]]; then
        WORKSHOP_ID="$arg"
      fi
      ;;
  esac
done

# Build filter arguments
FILTER_ARGS=(--filter "label=clarateach.type=learner-workspace")
if [[ -n "$WORKSHOP_ID" ]]; then
  FILTER_ARGS+=(--filter "label=clarateach.workshop=${WORKSHOP_ID}")
fi

# Get containers
CONTAINERS=$(docker ps -a "${FILTER_ARGS[@]}" --format '{{.Names}}|{{.ID}}|{{.Status}}|{{.Labels}}' 2>/dev/null || echo "")

if [[ -z "$CONTAINERS" ]]; then
  if [[ "$JSON_OUTPUT" == "true" ]]; then
    echo "[]"
  else
    echo "No containers found"
  fi
  exit 0
fi

# Helper function to extract label value from label string
extract_label() {
  local labels="$1"
  local key="$2"
  echo "$labels" | sed -n "s/.*${key}=\([^,]*\).*/\1/p"
}

if [[ "$JSON_OUTPUT" == "true" ]]; then
  # JSON output
  echo "["
  FIRST=true
  while IFS='|' read -r name id status labels; do
    # Extract labels
    workshop=$(extract_label "$labels" "clarateach.workshop")
    seat=$(extract_label "$labels" "clarateach.seat")
    workshop="${workshop:-unknown}"
    seat="${seat:-0}"
    network="clarateach-${workshop}"

    # Get IP address from workshop network (avoid concatenated IPs across networks)
    ip=$(docker inspect --format "{{range \$k,\$v := .NetworkSettings.Networks}}{{if eq \$k \"${network}\"}}{{\$v.IPAddress}}{{end}}{{end}}" "$name" 2>/dev/null || echo "")
    if [[ -z "$ip" ]]; then
      ip=$(docker inspect --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$name" 2>/dev/null || echo "")
    fi

    # Determine state
    if [[ "$status" == *"Up"* ]]; then
      state="running"
    elif [[ "$status" == *"Exited"* ]]; then
      state="stopped"
    else
      state="unknown"
    fi

    if [[ "$FIRST" == "true" ]]; then
      FIRST=false
    else
      echo ","
    fi

    cat <<EOF
  {
    "name": "${name}",
    "id": "${id}",
    "workshop": "${workshop}",
    "seat": ${seat},
    "state": "${state}",
    "ip": "${ip}",
    "status": "${status}"
  }
EOF
  done <<< "$CONTAINERS"
  echo ""
  echo "]"
else
  # Table output
  printf "%-35s %-12s %-10s %-5s %-15s %s\n" "NAME" "ID" "WORKSHOP" "SEAT" "IP" "STATUS"
  printf "%s\n" "$(printf '%.0s-' {1..100})"

  while IFS='|' read -r name id status labels; do
    workshop=$(extract_label "$labels" "clarateach.workshop")
    seat=$(extract_label "$labels" "clarateach.seat")
    workshop="${workshop:-unknown}"
    seat="${seat:-0}"
    network="clarateach-${workshop}"
    ip=$(docker inspect --format "{{range \$k,\$v := .NetworkSettings.Networks}}{{if eq \$k \"${network}\"}}{{\$v.IPAddress}}{{end}}{{end}}" "$name" 2>/dev/null || echo "")
    if [[ -z "$ip" ]]; then
      ip=$(docker inspect --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$name" 2>/dev/null || echo "-")
    fi

    # Truncate status for display
    short_status="${status:0:20}"

    printf "%-35s %-12s %-10s %-5s %-15s %s\n" "$name" "${id:0:12}" "$workshop" "$seat" "$ip" "$short_status"
  done <<< "$CONTAINERS"
fi
