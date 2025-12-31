#!/usr/bin/env bash
#
# Integration tests for container orchestration scripts
#
# Prerequisites:
#   - Docker running
#   - clarateach-workspace image built (or use WORKSPACE_IMAGE env var)
#
# Usage: ./test-orchestrator.sh
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_WORKSHOP_ID="test$(date +%s)"
TEST_PASSED=0
TEST_FAILED=0

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_pass() {
  echo -e "${GREEN}[PASS]${NC} $1"
  TEST_PASSED=$((TEST_PASSED + 1))
}

log_fail() {
  echo -e "${RED}[FAIL]${NC} $1"
  TEST_FAILED=$((TEST_FAILED + 1))
}

log_info() {
  echo -e "${YELLOW}[INFO]${NC} $1"
}

cleanup() {
  log_info "Cleaning up test containers..."
  "$SCRIPT_DIR/destroy-workshop.sh" "$TEST_WORKSHOP_ID" 2>/dev/null || true
}

# Cleanup on exit
trap cleanup EXIT

# Check prerequisites
check_prerequisites() {
  log_info "Checking prerequisites..."

  if ! command -v docker &>/dev/null; then
    echo "Error: Docker is not installed" >&2
    exit 1
  fi

  if ! docker info &>/dev/null; then
    echo "Error: Docker is not running" >&2
    exit 1
  fi

  # Check if workspace image exists
  IMAGE="${WORKSPACE_IMAGE:-clarateach-workspace}"
  if ! docker image inspect "$IMAGE" &>/dev/null; then
    log_info "Building workspace image..."
    (cd "$(dirname "$SCRIPT_DIR")" && docker build -t "$IMAGE" .)
  fi

  log_pass "Prerequisites check"
}

# Test: Create single container
test_create_container() {
  log_info "Testing create-container.sh..."

  # Create container
  if ! AUTH_DISABLED=true "$SCRIPT_DIR/create-container.sh" "$TEST_WORKSHOP_ID" 1 >/dev/null; then
    log_fail "create-container.sh failed"
    return 1
  fi

  # Verify container exists
  CONTAINER_NAME="clarateach-${TEST_WORKSHOP_ID}-1"
  if docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    log_pass "Container created and running"
  else
    log_fail "Container not found or not running"
    return 1
  fi

  # Verify network exists
  NETWORK_NAME="clarateach-${TEST_WORKSHOP_ID}"
  if docker network inspect "$NETWORK_NAME" &>/dev/null; then
    log_pass "Network created"
  else
    log_fail "Network not found"
    return 1
  fi

  # Verify volume exists
  VOLUME_NAME="${CONTAINER_NAME}-data"
  if docker volume inspect "$VOLUME_NAME" &>/dev/null; then
    log_pass "Volume created"
  else
    log_fail "Volume not found"
    return 1
  fi

  # Verify labels
  WORKSHOP_LABEL=$(docker inspect --format='{{index .Config.Labels "clarateach.workshop"}}' "$CONTAINER_NAME")
  SEAT_LABEL=$(docker inspect --format='{{index .Config.Labels "clarateach.seat"}}' "$CONTAINER_NAME")

  if [[ "$WORKSHOP_LABEL" == "$TEST_WORKSHOP_ID" ]] && [[ "$SEAT_LABEL" == "1" ]]; then
    log_pass "Container labels correct"
  else
    log_fail "Container labels incorrect (workshop=${WORKSHOP_LABEL}, seat=${SEAT_LABEL})"
    return 1
  fi

  # Test health endpoint (wait for startup)
  log_info "Waiting for server to start..."
  CONTAINER_IP=$(docker inspect --format "{{(index .NetworkSettings.Networks \"${NETWORK_NAME}\").IPAddress}}" "$CONTAINER_NAME")

  HEALTHY=false
  for i in {1..30}; do
    if docker exec "$CONTAINER_NAME" curl -sf "http://localhost:3002/health" >/dev/null 2>&1; then
      HEALTHY=true
      break
    fi
    sleep 1
  done

  if [[ "$HEALTHY" == "true" ]]; then
    log_pass "Server health check passed"
  else
    log_fail "Server health check failed"
    return 1
  fi
}

# Test: List containers
test_list_containers() {
  log_info "Testing list-containers.sh..."

  # List all containers
  OUTPUT=$("$SCRIPT_DIR/list-containers.sh" 2>/dev/null)
  if echo "$OUTPUT" | grep -q "${TEST_WORKSHOP_ID}"; then
    log_pass "list-containers.sh shows test container"
  else
    log_fail "list-containers.sh doesn't show test container"
    return 1
  fi

  # List with filter
  OUTPUT=$("$SCRIPT_DIR/list-containers.sh" "$TEST_WORKSHOP_ID" 2>/dev/null)
  if echo "$OUTPUT" | grep -qi "seat"; then
    log_pass "list-containers.sh filter works"
  else
    log_fail "list-containers.sh filter doesn't work"
    return 1
  fi

  # Test JSON output
  OUTPUT=$("$SCRIPT_DIR/list-containers.sh" "$TEST_WORKSHOP_ID" --json 2>/dev/null)
  if echo "$OUTPUT" | grep -q '"workshop"'; then
    log_pass "list-containers.sh JSON output works"
  else
    log_fail "list-containers.sh JSON output doesn't work"
    return 1
  fi
}

# Test: Create duplicate container (should fail)
test_duplicate_container() {
  log_info "Testing duplicate container prevention..."

  if AUTH_DISABLED=true "$SCRIPT_DIR/create-container.sh" "$TEST_WORKSHOP_ID" 1 2>/dev/null; then
    log_fail "Duplicate container creation should have failed"
    return 1
  else
    log_pass "Duplicate container creation blocked"
  fi
}

# Test: Create second container
test_create_second_container() {
  log_info "Testing second container creation..."

  if ! AUTH_DISABLED=true "$SCRIPT_DIR/create-container.sh" "$TEST_WORKSHOP_ID" 2 >/dev/null; then
    log_fail "Second container creation failed"
    return 1
  fi

  # Both containers should be on same network
  NETWORK_NAME="clarateach-${TEST_WORKSHOP_ID}"
  CONTAINER_COUNT=$(docker network inspect "$NETWORK_NAME" --format='{{len .Containers}}')

  if [[ "$CONTAINER_COUNT" == "2" ]]; then
    log_pass "Both containers on same network"
  else
    log_fail "Expected 2 containers on network, got $CONTAINER_COUNT"
    return 1
  fi
}

# Test: Destroy single container
test_destroy_container() {
  log_info "Testing destroy-container.sh..."

  CONTAINER_NAME="clarateach-${TEST_WORKSHOP_ID}-2"
  VOLUME_NAME="${CONTAINER_NAME}-data"

  if ! "$SCRIPT_DIR/destroy-container.sh" "$TEST_WORKSHOP_ID" 2 >/dev/null; then
    log_fail "destroy-container.sh failed"
    return 1
  fi

  # Verify container removed
  if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    log_fail "Container still exists after destroy"
    return 1
  else
    log_pass "Container removed"
  fi

  # Verify volume removed
  if docker volume inspect "$VOLUME_NAME" &>/dev/null; then
    log_fail "Volume still exists after destroy"
    return 1
  else
    log_pass "Volume removed"
  fi
}

# Test: Destroy with --keep-data
test_destroy_keep_data() {
  log_info "Testing destroy-container.sh --keep-data..."

  # Create container 3
  AUTH_DISABLED=true "$SCRIPT_DIR/create-container.sh" "$TEST_WORKSHOP_ID" 3 >/dev/null

  CONTAINER_NAME="clarateach-${TEST_WORKSHOP_ID}-3"
  VOLUME_NAME="${CONTAINER_NAME}-data"

  if ! "$SCRIPT_DIR/destroy-container.sh" "$TEST_WORKSHOP_ID" 3 --keep-data >/dev/null; then
    log_fail "destroy-container.sh --keep-data failed"
    return 1
  fi

  # Verify container removed
  if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    log_fail "Container still exists"
    return 1
  else
    log_pass "Container removed"
  fi

  # Verify volume kept
  if docker volume inspect "$VOLUME_NAME" &>/dev/null; then
    log_pass "Volume preserved with --keep-data"
    # Clean up the volume
    docker volume rm "$VOLUME_NAME" >/dev/null
  else
    log_fail "Volume was removed despite --keep-data"
    return 1
  fi
}

# Test: Destroy workshop
test_destroy_workshop() {
  log_info "Testing destroy-workshop.sh..."

  NETWORK_NAME="clarateach-${TEST_WORKSHOP_ID}"

  if ! "$SCRIPT_DIR/destroy-workshop.sh" "$TEST_WORKSHOP_ID" >/dev/null; then
    log_fail "destroy-workshop.sh failed"
    return 1
  fi

  # Verify all containers removed
  REMAINING=$(docker ps -a --filter "label=clarateach.workshop=${TEST_WORKSHOP_ID}" --format '{{.Names}}' | wc -l)
  if [[ "$REMAINING" -eq 0 ]]; then
    log_pass "All containers removed"
  else
    log_fail "Some containers still exist"
    return 1
  fi

  # Verify network removed
  if docker network inspect "$NETWORK_NAME" &>/dev/null; then
    log_fail "Network still exists"
    return 1
  else
    log_pass "Network removed"
  fi
}

# Test: Input validation
test_input_validation() {
  log_info "Testing input validation..."

  # Invalid workshop ID
  if AUTH_DISABLED=true "$SCRIPT_DIR/create-container.sh" "INVALID_ID!" 1 2>/dev/null; then
    log_fail "Should reject invalid workshop ID"
    return 1
  else
    log_pass "Rejects invalid workshop ID"
  fi

  # Invalid seat number
  if AUTH_DISABLED=true "$SCRIPT_DIR/create-container.sh" "validid" 0 2>/dev/null; then
    log_fail "Should reject seat 0"
    return 1
  else
    log_pass "Rejects seat 0"
  fi

  if AUTH_DISABLED=true "$SCRIPT_DIR/create-container.sh" "validid" -1 2>/dev/null; then
    log_fail "Should reject negative seat"
    return 1
  else
    log_pass "Rejects negative seat"
  fi
}

# Run all tests
main() {
  echo "=========================================="
  echo "Container Orchestrator Integration Tests"
  echo "=========================================="
  echo ""
  echo "Test workshop ID: ${TEST_WORKSHOP_ID}"
  echo ""

  check_prerequisites

  echo ""
  echo "Running tests..."
  echo ""

  test_create_container
  test_list_containers
  test_duplicate_container
  test_create_second_container
  test_destroy_container
  test_destroy_keep_data
  test_destroy_workshop
  test_input_validation

  echo ""
  echo "=========================================="
  echo "Test Results"
  echo "=========================================="
  echo -e "Passed: ${GREEN}${TEST_PASSED}${NC}"
  echo -e "Failed: ${RED}${TEST_FAILED}${NC}"
  echo ""

  if [[ "$TEST_FAILED" -gt 0 ]]; then
    exit 1
  fi
}

main "$@"
