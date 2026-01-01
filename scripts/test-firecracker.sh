#!/bin/bash
set -euo pipefail

# Test script for Firecracker VM creation
# Usage: sudo ./scripts/test-firecracker.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="$(dirname "$SCRIPT_DIR")"
IMAGES_DIR="${IMAGES_DIR:-/var/lib/clarateach/images}"
SOCKET_DIR="${SOCKET_DIR:-/tmp/clarateach}"
AGENT_TOKEN="${AGENT_TOKEN:-test-token}"
PORT="${PORT:-9090}"
NUM_VMS="${NUM_VMS:-3}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log() { echo -e "${GREEN}[+]${NC} $1"; }
warn() { echo -e "${YELLOW}[!]${NC} $1"; }
error() { echo -e "${RED}[x]${NC} $1"; }

cleanup() {
    log "Cleaning up..."
    # Kill agent if we started it
    if [ -n "${AGENT_PID:-}" ]; then
        kill "$AGENT_PID" 2>/dev/null || true
        wait "$AGENT_PID" 2>/dev/null || true
    fi
    # Kill any firecracker processes
    pkill -f "firecracker --api-sock $SOCKET_DIR" 2>/dev/null || true
    # Clean up socket directory
    rm -f "$SOCKET_DIR"/*.sock "$SOCKET_DIR"/rootfs-*.ext4 2>/dev/null || true
}

trap cleanup EXIT

# Check prerequisites
check_prerequisites() {
    log "Checking prerequisites..."

    if [ "$(id -u)" -ne 0 ]; then
        error "This script must be run as root (for KVM access)"
        exit 1
    fi

    if [ ! -e /dev/kvm ]; then
        error "/dev/kvm not found. KVM is required for Firecracker."
        exit 1
    fi

    if ! command -v firecracker &>/dev/null; then
        error "firecracker not found. Run: sudo ../scripts/setup-firecracker.sh"
        exit 1
    fi

    if [ ! -f "$IMAGES_DIR/vmlinux" ]; then
        error "Kernel not found at $IMAGES_DIR/vmlinux"
        exit 1
    fi

    if [ ! -f "$IMAGES_DIR/rootfs.ext4" ]; then
        error "Rootfs not found at $IMAGES_DIR/rootfs.ext4"
        exit 1
    fi

    log "Prerequisites OK"
}

# Build agent
build_agent() {
    log "Building agent..."
    cd "$BACKEND_DIR"
    go build -o agent ./cmd/agent/
    log "Agent built"
}

# Start agent
start_agent() {
    log "Starting agent on port $PORT..."
    mkdir -p "$SOCKET_DIR"

    IMAGES_DIR="$IMAGES_DIR" \
    SOCKET_DIR="$SOCKET_DIR" \
    AGENT_TOKEN="$AGENT_TOKEN" \
    PORT="$PORT" \
    "$BACKEND_DIR/agent" &
    AGENT_PID=$!

    # Wait for agent to be ready
    for i in {1..30}; do
        if curl -s "http://localhost:$PORT/health" &>/dev/null; then
            log "Agent ready (PID: $AGENT_PID)"
            return 0
        fi
        sleep 0.5
    done

    error "Agent failed to start"
    exit 1
}

# Create VMs
create_vms() {
    log "Creating $NUM_VMS VMs..."

    for i in $(seq 1 "$NUM_VMS"); do
        response=$(curl -s -X POST "http://localhost:$PORT/vms" \
            -H "Authorization: Bearer $AGENT_TOKEN" \
            -H "Content-Type: application/json" \
            -d "{\"workshop_id\": \"test\", \"seat_id\": $i}")

        ip=$(echo "$response" | grep -o '"ip":"[^"]*"' | cut -d'"' -f4)
        if [ -n "$ip" ]; then
            log "  VM $i created: $ip"
        else
            error "  VM $i failed: $response"
            return 1
        fi
    done
}

# Test VMs
test_vms() {
    log "Testing VMs..."

    # Wait for VMs to boot
    sleep 2

    # Check firecracker processes
    fc_count=$(pgrep -c -f "firecracker --api-sock" || echo "0")
    if [ "$fc_count" -eq "$NUM_VMS" ]; then
        log "  Firecracker processes: $fc_count OK"
    else
        error "  Expected $NUM_VMS firecracker processes, found $fc_count"
        return 1
    fi

    # Ping each VM
    for i in $(seq 1 "$NUM_VMS"); do
        ip="192.168.100.$((10 + i))"
        if ping -c 1 -W 2 "$ip" &>/dev/null; then
            log "  Ping $ip: OK"
        else
            warn "  Ping $ip: FAILED (VM may not have networking configured)"
        fi
    done

    # Check API listing
    vms=$(curl -s -H "Authorization: Bearer $AGENT_TOKEN" \
        "http://localhost:$PORT/vms?workshop_id=test")
    vm_count=$(echo "$vms" | grep -o '"seat_id"' | wc -l)

    if [ "$vm_count" -eq "$NUM_VMS" ]; then
        log "  API listing: $vm_count VMs OK"
    else
        error "  Expected $NUM_VMS VMs in API, found $vm_count"
        return 1
    fi
}

# Destroy VMs
destroy_vms() {
    log "Destroying VMs..."

    for i in $(seq 1 "$NUM_VMS"); do
        response=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE \
            "http://localhost:$PORT/vms/test/$i" \
            -H "Authorization: Bearer $AGENT_TOKEN")

        if [ "$response" = "204" ]; then
            log "  VM $i destroyed"
        else
            warn "  VM $i destroy returned: $response"
        fi
    done
}

# Main
main() {
    echo "========================================"
    echo "  Firecracker VM Test"
    echo "========================================"
    echo ""

    check_prerequisites
    build_agent
    start_agent
    create_vms
    test_vms
    destroy_vms

    echo ""
    echo "========================================"
    log "All tests passed!"
    echo "========================================"
}

main "$@"
