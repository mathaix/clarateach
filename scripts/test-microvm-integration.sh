#!/usr/bin/env bash
# MicroVM Integration Test Suite
# Run this after updating the rootfs/snapshot to verify everything works
# before doing a full browser test.

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
AGENT_HOST="${AGENT_HOST:-}"
WORKSHOP_ID="${WORKSHOP_ID:-test-integration}"
SEAT_ID="${SEAT_ID:-99}"
ROOTFS_PATH="${ROOTFS_PATH:-/var/lib/clarateach/images/rootfs.ext4}"

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_test() { echo -e "\n${YELLOW}[TEST]${NC} $1"; }

pass() {
    echo -e "  ${GREEN}✓ PASS${NC}: $1"
    ((TESTS_PASSED++))
}

fail() {
    echo -e "  ${RED}✗ FAIL${NC}: $1"
    ((TESTS_FAILED++))
}

# Check if running on GCP VM or locally
detect_environment() {
    if [ -n "$AGENT_HOST" ]; then
        log_info "Using provided AGENT_HOST: $AGENT_HOST"
        return
    fi

    # Try to detect if we're on a GCP VM with agent
    if curl -sf http://localhost:9090/health >/dev/null 2>&1; then
        AGENT_HOST="localhost:9090"
        log_info "Detected local agent at $AGENT_HOST"
    else
        log_error "No agent found. Set AGENT_HOST environment variable."
        log_info "Example: AGENT_HOST=34.70.202.73:9090 $0"
        exit 1
    fi
}

# ============================================================================
# Test 1: Rootfs Validation
# ============================================================================
test_rootfs_validation() {
    log_test "1. Rootfs Validation"

    # This test runs on the GCP VM where the rootfs is located
    # We'll use SSH if AGENT_HOST is remote, or local commands if local

    if [ "$AGENT_HOST" = "localhost:9090" ]; then
        # Local - direct access to rootfs
        MOUNT_DIR=$(mktemp -d)

        if [ ! -f "$ROOTFS_PATH" ]; then
            fail "Rootfs not found at $ROOTFS_PATH"
            return
        fi

        # Mount rootfs (requires sudo)
        if ! sudo mount -o ro "$ROOTFS_PATH" "$MOUNT_DIR" 2>/dev/null; then
            fail "Failed to mount rootfs"
            rmdir "$MOUNT_DIR"
            return
        fi

        # Check /sbin/init exists
        if [ -f "$MOUNT_DIR/sbin/init" ]; then
            pass "/sbin/init exists"
        else
            fail "/sbin/init not found"
        fi

        # Check /sbin/init is executable
        if [ -x "$MOUNT_DIR/sbin/init" ]; then
            pass "/sbin/init is executable"
        else
            fail "/sbin/init is not executable"
        fi

        # Check for MICROVM_MODE=true
        if grep -q "MICROVM_MODE=true" "$MOUNT_DIR/sbin/init" 2>/dev/null; then
            pass "Init script contains MICROVM_MODE=true"
        else
            fail "Init script missing MICROVM_MODE=true"
        fi

        # Check for AUTH_DISABLED=true
        if grep -q "AUTH_DISABLED=true" "$MOUNT_DIR/sbin/init" 2>/dev/null; then
            pass "Init script contains AUTH_DISABLED=true"
        else
            fail "Init script missing AUTH_DISABLED=true"
        fi

        # Check workspace server exists
        if [ -f "$MOUNT_DIR/home/learner/server/dist/index.js" ]; then
            pass "Workspace server dist/index.js exists"
        else
            fail "Workspace server dist/index.js not found"
        fi

        # Check workspace server has MICROVM_MODE support
        if grep -q "MICROVM_MODE" "$MOUNT_DIR/home/learner/server/dist/index.js" 2>/dev/null; then
            pass "Workspace server has MICROVM_MODE support"
        else
            fail "Workspace server missing MICROVM_MODE support"
        fi

        # Cleanup
        sudo umount "$MOUNT_DIR" 2>/dev/null || true
        rmdir "$MOUNT_DIR"
    else
        log_warn "Rootfs validation skipped (remote agent - run on GCP VM for this test)"
    fi
}

# ============================================================================
# Test 2: MicroVM Boot Test
# ============================================================================
test_microvm_boot() {
    log_test "2. MicroVM Boot Test"

    local agent_url="http://$AGENT_HOST"

    # Check agent health first
    if ! curl -sf "$agent_url/health" >/dev/null; then
        fail "Agent not reachable at $agent_url"
        return
    fi
    pass "Agent is healthy"

    # Create a test MicroVM
    log_info "Creating test MicroVM (workshop=$WORKSHOP_ID, seat=$SEAT_ID)..."

    local create_response
    create_response=$(curl -sf -X POST "$agent_url/vms" \
        -H "Content-Type: application/json" \
        -d "{\"workshop_id\": \"$WORKSHOP_ID\", \"seat_id\": $SEAT_ID}" 2>&1) || true

    if echo "$create_response" | grep -q '"ip"'; then
        pass "MicroVM created successfully"
        local vm_ip
        vm_ip=$(echo "$create_response" | grep -oE '"ip":"[^"]+"' | cut -d'"' -f4)
        log_info "MicroVM IP: $vm_ip"

        # Store for later tests
        export TEST_VM_IP="$vm_ip"
    elif echo "$create_response" | grep -q "vm_exists"; then
        log_warn "MicroVM already exists, attempting to get its IP..."
        # Try to get the IP from health endpoint
        TEST_VM_IP="192.168.100.$((10 + SEAT_ID))"
        pass "Using existing MicroVM at $TEST_VM_IP"
    else
        fail "Failed to create MicroVM: $create_response"
        return
    fi

    # Wait for workspace server to start
    log_info "Waiting for workspace server to start..."
    local max_attempts=30
    local attempt=0

    while [ $attempt -lt $max_attempts ]; do
        if curl -sf "http://$TEST_VM_IP:3001/health" >/dev/null 2>&1; then
            pass "Workspace server is responding on port 3001"
            break
        fi
        ((attempt++))
        sleep 1
    done

    if [ $attempt -eq $max_attempts ]; then
        fail "Workspace server did not start within ${max_attempts}s"
    fi
}

# ============================================================================
# Test 3: Route Validation
# ============================================================================
test_route_validation() {
    log_test "3. Route Validation (MICROVM_MODE routes)"

    if [ -z "${TEST_VM_IP:-}" ]; then
        log_warn "Skipping - no VM IP available"
        return
    fi

    local ws_url="http://$TEST_VM_IP:3001"
    local files_url="http://$TEST_VM_IP:3002"

    # Test /health on terminal server
    if curl -sf "$ws_url/health" | grep -q '"status":"ok"'; then
        pass "GET /health returns 200 with status ok"
    else
        fail "GET /health failed"
    fi

    # Test /files endpoint (MICROVM_MODE route)
    local files_response
    files_response=$(curl -sf "$files_url/files" 2>&1) || files_response="FAILED"

    if echo "$files_response" | grep -q '"files"'; then
        pass "GET /files returns 200 with files array (MICROVM_MODE working)"
    else
        fail "GET /files failed: $files_response"
    fi

    # Test /terminal endpoint exists (will fail upgrade but should not 404)
    local terminal_response
    terminal_response=$(curl -s -o /dev/null -w "%{http_code}" "$ws_url/terminal" 2>&1)

    # WebSocket endpoints return various codes, but NOT 404
    if [ "$terminal_response" != "404" ]; then
        pass "GET /terminal does not return 404 (endpoint exists)"
    else
        fail "GET /terminal returns 404 (endpoint missing)"
    fi

    # Test that Docker-mode routes return 404 (they should NOT exist)
    local docker_route_response
    docker_route_response=$(curl -s -o /dev/null -w "%{http_code}" "$files_url/vm/1/files" 2>&1)

    if [ "$docker_route_response" = "404" ]; then
        pass "GET /vm/1/files returns 404 (Docker routes correctly disabled)"
    else
        fail "GET /vm/1/files returns $docker_route_response (should be 404 in MICROVM_MODE)"
    fi
}

# ============================================================================
# Test 4: Agent Proxy Test
# ============================================================================
test_agent_proxy() {
    log_test "4. Agent Proxy Test"

    if [ -z "${TEST_VM_IP:-}" ]; then
        log_warn "Skipping - no VM IP available"
        return
    fi

    local agent_url="http://$AGENT_HOST"
    local proxy_base="$agent_url/proxy/$WORKSHOP_ID/$SEAT_ID"

    # Test /proxy/{ws}/{seat}/files
    local proxy_files_response
    proxy_files_response=$(curl -sf "$proxy_base/files" 2>&1) || proxy_files_response="FAILED"

    if echo "$proxy_files_response" | grep -q '"files"'; then
        pass "Agent proxy /files works correctly"
    else
        fail "Agent proxy /files failed: $proxy_files_response"
    fi

    # Test /proxy/{ws}/{seat}/health (if endpoint exists)
    local proxy_health_response
    proxy_health_response=$(curl -sf "$proxy_base/health" 2>&1) || proxy_health_response="NOT_FOUND"

    if echo "$proxy_health_response" | grep -q '"status"'; then
        pass "Agent proxy /health works correctly"
    else
        log_warn "Agent proxy /health endpoint may not exist"
    fi
}

# ============================================================================
# Test 5: Cleanup
# ============================================================================
test_cleanup() {
    log_test "5. Cleanup"

    local agent_url="http://$AGENT_HOST"

    # Delete test MicroVM
    log_info "Deleting test MicroVM..."

    local delete_response
    delete_response=$(curl -sf -X DELETE "$agent_url/vms/$WORKSHOP_ID/$SEAT_ID" 2>&1) || delete_response="FAILED"

    if echo "$delete_response" | grep -q "deleted\|success" || [ "$delete_response" = "" ]; then
        pass "Test MicroVM deleted"
    else
        log_warn "Could not delete test MicroVM: $delete_response"
    fi
}

# ============================================================================
# Main
# ============================================================================
main() {
    echo "=============================================="
    echo "  MicroVM Integration Test Suite"
    echo "=============================================="
    echo ""

    detect_environment

    echo ""
    echo "Configuration:"
    echo "  AGENT_HOST:   $AGENT_HOST"
    echo "  WORKSHOP_ID:  $WORKSHOP_ID"
    echo "  SEAT_ID:      $SEAT_ID"
    echo "  ROOTFS_PATH:  $ROOTFS_PATH"

    # Run tests
    test_rootfs_validation
    test_microvm_boot
    test_route_validation
    test_agent_proxy
    test_cleanup

    # Summary
    echo ""
    echo "=============================================="
    echo "  Test Summary"
    echo "=============================================="
    echo -e "  ${GREEN}Passed${NC}: $TESTS_PASSED"
    echo -e "  ${RED}Failed${NC}: $TESTS_FAILED"
    echo ""

    if [ $TESTS_FAILED -eq 0 ]; then
        echo -e "${GREEN}All tests passed! Ready for browser testing.${NC}"
        exit 0
    else
        echo -e "${RED}Some tests failed. Fix issues before browser testing.${NC}"
        exit 1
    fi
}

main "$@"
