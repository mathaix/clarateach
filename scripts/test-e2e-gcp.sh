#!/bin/bash
# test-e2e-gcp.sh - Full end-to-end test for ClaraTeach Firecracker flow
# Tests: Backend API → GCP VM → Agent → MicroVMs → User Interface
set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

# Configuration
BACKEND_URL="${BACKEND_URL:-http://localhost:8080}"
WORKSHOP_NAME="${WORKSHOP_NAME:-E2E Test $(date +%H%M%S)}"
SEATS="${SEATS:-2}"
TIMEOUT="${TIMEOUT:-300}"  # 5 minutes for GCP VM to be ready

log() { echo -e "${GREEN}[✓]${NC} $1"; }
warn() { echo -e "${YELLOW}[!]${NC} $1"; }
fail() { echo -e "${RED}[✗]${NC} $1"; }
info() { echo -e "${CYAN}[i]${NC} $1"; }

WORKSHOP_ID=""
AGENT_IP=""

cleanup() {
    echo ""
    echo "=== Cleanup ==="
    if [[ -n "$WORKSHOP_ID" ]]; then
        info "Deleting workshop $WORKSHOP_ID..."
        curl -s -X DELETE "$BACKEND_URL/api/workshops/$WORKSHOP_ID" || true
        log "Cleanup request sent"
    fi
}

trap cleanup EXIT

echo "=============================================="
echo "ClaraTeach Full E2E Test - GCP Firecracker"
echo "=============================================="
echo "Backend URL: $BACKEND_URL"
echo "Workshop Name: $WORKSHOP_NAME"
echo "Seats: $SEATS"
echo ""

# Test 1: Check backend health
echo "=== Test 1: Backend Health Check ==="
HEALTH=$(curl -s "$BACKEND_URL/health" 2>/dev/null || echo "error")
if echo "$HEALTH" | grep -q "ok\|healthy"; then
    log "Backend is healthy"
else
    fail "Backend not responding at $BACKEND_URL"
    info "Start the backend with: go run ./cmd/server/"
    exit 1
fi

# Test 2: Create workshop with Firecracker runtime
echo ""
echo "=== Test 2: Create Workshop (Firecracker) ==="
RESULT=$(curl -s -X POST "$BACKEND_URL/api/workshops" \
    -H "Content-Type: application/json" \
    -d "{\"name\": \"$WORKSHOP_NAME\", \"seats\": $SEATS, \"runtime_type\": \"firecracker\"}")

if echo "$RESULT" | grep -q '"id"'; then
    WORKSHOP_ID=$(echo "$RESULT" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
    log "Workshop created: $WORKSHOP_ID"
else
    fail "Failed to create workshop: $RESULT"
    exit 1
fi

# Test 3: Wait for workshop to be ready
echo ""
echo "=== Test 3: Wait for Workshop Ready ==="
info "Waiting for GCP VM to be provisioned (this may take 2-3 minutes)..."

START_TIME=$(date +%s)
while true; do
    ELAPSED=$(($(date +%s) - START_TIME))
    if [[ $ELAPSED -gt $TIMEOUT ]]; then
        fail "Timeout waiting for workshop to be ready"
        exit 1
    fi

    STATUS=$(curl -s "$BACKEND_URL/api/workshops/$WORKSHOP_ID")
    WORKSHOP_STATUS=$(echo "$STATUS" | grep -o '"status":"[^"]*"' | cut -d'"' -f4 || echo "unknown")

    if [[ "$WORKSHOP_STATUS" == "running" ]]; then
        AGENT_IP=$(echo "$STATUS" | grep -o '"agent_ip":"[^"]*"' | cut -d'"' -f4 || echo "")
        log "Workshop is running! Agent IP: $AGENT_IP"
        break
    elif [[ "$WORKSHOP_STATUS" == "failed" ]]; then
        fail "Workshop creation failed"
        echo "$STATUS" | head -c 500
        exit 1
    fi

    printf "\r  Status: %-15s Elapsed: %ds" "$WORKSHOP_STATUS" "$ELAPSED"
    sleep 5
done
echo ""

# Test 4: Check agent health
echo ""
echo "=== Test 4: Agent Health Check ==="
if [[ -z "$AGENT_IP" ]]; then
    warn "No agent IP found, skipping agent tests"
else
    AGENT_URL="http://$AGENT_IP:9090"
    AGENT_HEALTH=$(curl -s --connect-timeout 10 "$AGENT_URL/health" 2>/dev/null || echo "error")
    if echo "$AGENT_HEALTH" | grep -q '"status":"healthy"'; then
        log "Agent is healthy"
    else
        warn "Agent not responding: $AGENT_HEALTH"
    fi
fi

# Test 5: List MicroVMs
echo ""
echo "=== Test 5: List MicroVMs ==="
if [[ -n "$AGENT_IP" ]]; then
    VMS=$(curl -s "$AGENT_URL/vms" 2>/dev/null || echo "[]")
    VM_COUNT=$(echo "$VMS" | grep -o '"seat_id"' | wc -l || echo "0")
    if [[ "$VM_COUNT" -ge "$SEATS" ]]; then
        log "Found $VM_COUNT MicroVMs"
    else
        warn "Expected $SEATS MicroVMs, found $VM_COUNT"
    fi
fi

# Test 6: Check proxy health for each seat
echo ""
echo "=== Test 6: MicroVM Services Health ==="
if [[ -n "$AGENT_IP" ]]; then
    for seat in $(seq 1 "$SEATS"); do
        PROXY_HEALTH=$(curl -s "$AGENT_URL/proxy/$WORKSHOP_ID/$seat/health" 2>/dev/null || echo "{}")
        STATUS=$(echo "$PROXY_HEALTH" | grep -o '"status":"[^"]*"' | cut -d'"' -f4 || echo "unknown")
        if [[ "$STATUS" == "healthy" ]]; then
            log "Seat $seat: healthy (terminal + files)"
        else
            warn "Seat $seat: $STATUS"
        fi
    done
fi

# Summary
echo ""
echo "=============================================="
echo "Workshop Ready!"
echo "=============================================="
echo ""
echo "Workshop ID: $WORKSHOP_ID"
echo "Agent URL: http://$AGENT_IP:9090"
echo ""
echo "Access URLs:"
for seat in $(seq 1 "$SEATS"); do
    VM_IP="192.168.100.$((10 + seat))"
    echo "  Seat $seat:"
    echo "    - Terminal WebSocket: ws://$AGENT_IP:9090/proxy/$WORKSHOP_ID/$seat/terminal"
    echo "    - Files API: http://$AGENT_IP:9090/proxy/$WORKSHOP_ID/$seat/files/"
    echo "    - Direct (internal): http://$VM_IP:3001 / http://$VM_IP:3002"
done
echo ""
echo "Press Enter to cleanup and exit..."
read -r

echo ""
log "Test complete!"
