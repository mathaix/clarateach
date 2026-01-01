#!/bin/bash
# test-e2e-local.sh - End-to-end test for Firecracker MicroVMs on local worker
# Run this script on a KVM-enabled VM (e.g., clara2) with the agent running
set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
AGENT_URL="${AGENT_URL:-http://localhost:9090}"
AGENT_TOKEN="${AGENT_TOKEN:-}"
WORKSHOP_ID="${WORKSHOP_ID:-e2e-test-$(date +%s)}"
SEATS="${SEATS:-3}"
TIMEOUT="${TIMEOUT:-60}"

# Stats
TESTS_PASSED=0
TESTS_FAILED=0

log() { echo -e "${GREEN}[✓]${NC} $1"; }
warn() { echo -e "${YELLOW}[!]${NC} $1"; }
fail() { echo -e "${RED}[✗]${NC} $1"; TESTS_FAILED=$((TESTS_FAILED + 1)); }
pass() { TESTS_PASSED=$((TESTS_PASSED + 1)); log "$1"; }

# Build auth header if token is set
AUTH_HEADER=""
if [[ -n "$AGENT_TOKEN" ]]; then
    AUTH_HEADER="-H Authorization: Bearer $AGENT_TOKEN"
fi

cleanup() {
    echo ""
    echo "=== Cleanup ==="
    for seat in $(seq 1 "$SEATS"); do
        echo "Destroying VM: $WORKSHOP_ID seat $seat"
        curl -s -X DELETE "$AGENT_URL/vms/$WORKSHOP_ID/$seat" $AUTH_HEADER || true
    done
    log "Cleanup complete"
}

# Set trap for cleanup on exit
trap cleanup EXIT

echo "=============================================="
echo "ClaraTeach E2E Test - Local Firecracker"
echo "=============================================="
echo "Agent URL: $AGENT_URL"
echo "Workshop ID: $WORKSHOP_ID"
echo "Seats: $SEATS"
echo ""

# Test 1: Agent health check
echo "=== Test 1: Agent Health Check ==="
HEALTH=$(curl -s "$AGENT_URL/health")
if echo "$HEALTH" | grep -q '"status":"healthy"'; then
    WORKER_ID=$(echo "$HEALTH" | grep -o '"worker_id":"[^"]*"' | cut -d'"' -f4)
    pass "Agent is healthy (worker: $WORKER_ID)"
else
    fail "Agent health check failed: $HEALTH"
    exit 1
fi

# Test 2: Create MicroVMs
echo ""
echo "=== Test 2: Create MicroVMs ==="
for seat in $(seq 1 "$SEATS"); do
    echo "Creating VM: $WORKSHOP_ID seat $seat"
    RESULT=$(curl -s -X POST "$AGENT_URL/vms" \
        -H "Content-Type: application/json" \
        $AUTH_HEADER \
        -d "{\"workshop_id\": \"$WORKSHOP_ID\", \"seat_id\": $seat}")

    if echo "$RESULT" | grep -q '"status":"running"'; then
        IP=$(echo "$RESULT" | grep -o '"ip":"[^"]*"' | cut -d'"' -f4)
        pass "Created VM seat $seat with IP $IP"
    else
        fail "Failed to create VM seat $seat: $RESULT"
    fi
done

# Give VMs time to boot
echo ""
echo "Waiting 5s for VMs to initialize..."
sleep 5

# Test 3: List VMs
echo ""
echo "=== Test 3: List VMs ==="
VMS=$(curl -s "$AGENT_URL/vms?workshop_id=$WORKSHOP_ID" $AUTH_HEADER)
VM_COUNT=$(echo "$VMS" | grep -o '"workshop_id"' | wc -l)
if [[ "$VM_COUNT" -eq "$SEATS" ]]; then
    pass "Listed $VM_COUNT VMs for workshop $WORKSHOP_ID"
else
    fail "Expected $SEATS VMs, got $VM_COUNT"
fi

# Test 4: Ping MicroVMs
echo ""
echo "=== Test 4: Ping MicroVMs ==="
for seat in $(seq 1 "$SEATS"); do
    IP="192.168.100.$((10 + seat))"
    if ping -c 1 -W 2 "$IP" > /dev/null 2>&1; then
        pass "Pinged $IP (seat $seat)"
    else
        fail "Cannot ping $IP (seat $seat)"
    fi
done

# Test 5: Proxy health check
echo ""
echo "=== Test 5: Proxy Health Check ==="
for seat in $(seq 1 "$SEATS"); do
    HEALTH=$(curl -s "$AGENT_URL/proxy/$WORKSHOP_ID/$seat/health" $AUTH_HEADER 2>/dev/null || echo "error")
    if echo "$HEALTH" | grep -q '"vm_ip"'; then
        STATUS=$(echo "$HEALTH" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
        pass "Proxy health for seat $seat: $STATUS"
    else
        warn "Proxy health check failed for seat $seat (expected - MicroVM services not running)"
    fi
done

# Test 6: Get individual VM
echo ""
echo "=== Test 6: Get Individual VM ==="
for seat in $(seq 1 "$SEATS"); do
    VM=$(curl -s "$AGENT_URL/vms/$WORKSHOP_ID/$seat" $AUTH_HEADER)
    if echo "$VM" | grep -q '"status":"running"'; then
        pass "Got VM details for seat $seat"
    else
        fail "Failed to get VM seat $seat: $VM"
    fi
done

# Summary
echo ""
echo "=============================================="
echo "Test Summary"
echo "=============================================="
echo -e "Passed: ${GREEN}$TESTS_PASSED${NC}"
echo -e "Failed: ${RED}$TESTS_FAILED${NC}"
echo ""

if [[ "$TESTS_FAILED" -eq 0 ]]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed${NC}"
    exit 1
fi
