#!/bin/bash
set -e

# Configuration
PROJECT="${GCP_PROJECT:-clarateach}"
ZONE="${GCP_ZONE:-us-central1-b}"
SNAPSHOT_NAME="${SNAPSHOT_NAME:-clarateach-agent-$(date +%Y%m%d-%H%M%S)}"
TEMP_VM_NAME="clarateach-snapshot-builder-$$"
MACHINE_TYPE="n2-standard-4"

# Paths to local artifacts (must exist before running)
AGENT_BINARY="${AGENT_BINARY:-./agent}"
ROOTFS_IMAGE="${ROOTFS_IMAGE:-./rootfs.ext4}"
KERNEL_IMAGE="${KERNEL_IMAGE:-./vmlinux}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() { echo -e "${GREEN}[INFO]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# Cleanup function
cleanup() {
    if [[ -n "$TEMP_VM_NAME" ]]; then
        log "Cleaning up temporary VM: $TEMP_VM_NAME"
        gcloud compute instances delete "$TEMP_VM_NAME" \
            --project="$PROJECT" \
            --zone="$ZONE" \
            --quiet 2>/dev/null || true
    fi
}

# Check prerequisites
check_prerequisites() {
    log "Checking prerequisites..."

    [[ -f "$AGENT_BINARY" ]] || error "Agent binary not found: $AGENT_BINARY"
    [[ -f "$ROOTFS_IMAGE" ]] || error "Rootfs image not found: $ROOTFS_IMAGE"
    [[ -f "$KERNEL_IMAGE" ]] || error "Kernel image not found: $KERNEL_IMAGE"

    command -v gcloud &>/dev/null || error "gcloud CLI not installed"

    log "All prerequisites met"
}

# Build agent if needed
build_agent() {
    if [[ ! -f "$AGENT_BINARY" ]] || [[ "$BUILD_AGENT" == "true" ]]; then
        log "Building agent binary for Linux..."
        GOOS=linux GOARCH=amd64 go build -o "$AGENT_BINARY" ./cmd/agent/
    fi
}

# Create temporary VM
create_temp_vm() {
    log "Creating temporary VM: $TEMP_VM_NAME"

    gcloud compute instances create "$TEMP_VM_NAME" \
        --project="$PROJECT" \
        --zone="$ZONE" \
        --machine-type="$MACHINE_TYPE" \
        --min-cpu-platform="Intel Cascade Lake" \
        --enable-nested-virtualization \
        --image-family=ubuntu-2204-lts \
        --image-project=ubuntu-os-cloud \
        --boot-disk-size=50GB \
        --boot-disk-type=pd-ssd \
        --tags=clarateach-agent \
        --metadata=startup-script='#!/bin/bash
echo "VM started, waiting for setup..."'

    log "Waiting for VM to be ready..."
    sleep 30

    # Wait for SSH to be available
    for i in {1..30}; do
        if gcloud compute ssh "$TEMP_VM_NAME" --project="$PROJECT" --zone="$ZONE" --command="echo ready" &>/dev/null; then
            log "VM is ready"
            return 0
        fi
        sleep 10
    done
    error "VM failed to become ready"
}

# Upload artifacts
upload_artifacts() {
    log "Uploading artifacts to VM..."

    gcloud compute scp "$AGENT_BINARY" "$TEMP_VM_NAME":/tmp/agent \
        --project="$PROJECT" --zone="$ZONE"

    gcloud compute scp "$ROOTFS_IMAGE" "$TEMP_VM_NAME":/tmp/rootfs.ext4 \
        --project="$PROJECT" --zone="$ZONE"

    gcloud compute scp "$KERNEL_IMAGE" "$TEMP_VM_NAME":/tmp/vmlinux \
        --project="$PROJECT" --zone="$ZONE"

    log "Artifacts uploaded"
}

# Setup VM
setup_vm() {
    log "Setting up VM..."

    gcloud compute ssh "$TEMP_VM_NAME" --project="$PROJECT" --zone="$ZONE" --command='
set -e

echo "=== Installing dependencies ==="
sudo apt-get update
sudo apt-get install -y curl iptables iproute2

echo "=== Installing Cloudflared (for Quick Tunnel) ==="
curl -sL https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64.deb -o /tmp/cloudflared.deb
sudo dpkg -i /tmp/cloudflared.deb
rm /tmp/cloudflared.deb

echo "=== Installing Firecracker ==="
FC_VERSION="v1.5.0"
curl -sL "https://github.com/firecracker-microvm/firecracker/releases/download/${FC_VERSION}/firecracker-${FC_VERSION}-x86_64.tgz" | tar xz
sudo mv release-${FC_VERSION}-x86_64/firecracker-${FC_VERSION}-x86_64 /usr/local/bin/firecracker
sudo chmod +x /usr/local/bin/firecracker
rm -rf release-${FC_VERSION}-x86_64

echo "=== Installing Agent ==="
sudo mv /tmp/agent /usr/local/bin/agent
sudo chmod +x /usr/local/bin/agent

echo "=== Setting up images directory ==="
sudo mkdir -p /var/lib/clarateach/images
sudo mv /tmp/rootfs.ext4 /var/lib/clarateach/images/
sudo mv /tmp/vmlinux /var/lib/clarateach/images/

echo "=== Setting up MicroVM networking ==="
# Create bridge for MicroVMs
sudo ip link add name fcbr0 type bridge 2>/dev/null || true
sudo ip addr add 192.168.100.1/24 dev fcbr0 2>/dev/null || true
sudo ip link set fcbr0 up

# Enable IP forwarding
echo "net.ipv4.ip_forward=1" | sudo tee /etc/sysctl.d/99-firecracker.conf
sudo sysctl -p /etc/sysctl.d/99-firecracker.conf

# NAT for MicroVM internet access
sudo iptables -t nat -A POSTROUTING -s 192.168.100.0/24 -o ens4 -j MASQUERADE
sudo iptables -A FORWARD -i fcbr0 -o ens4 -j ACCEPT
sudo iptables -A FORWARD -i ens4 -o fcbr0 -m state --state RELATED,ESTABLISHED -j ACCEPT

# Save iptables rules
sudo apt-get install -y iptables-persistent
sudo netfilter-persistent save

echo "=== Creating systemd service ==="
# Note: The agent handles everything in Go code:
# - Bridge network setup (replaces setup-microvm-network.sh)
# - Tunnel management (spawns cloudflared, registers URL)
# No external scripts needed.
sudo tee /etc/systemd/system/clarateach-agent.service > /dev/null << '\''EOF'\''
[Unit]
Description=ClaraTeach Worker Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
Environment=IMAGES_DIR=/var/lib/clarateach/images
Environment=SOCKET_DIR=/tmp/clarateach
ExecStart=/usr/local/bin/agent
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable clarateach-agent

echo "=== Cleaning up ==="
sudo apt-get clean
sudo rm -rf /var/lib/apt/lists/*
sudo rm -rf /tmp/*
sudo journalctl --vacuum-time=1d

echo "=== Setup complete ==="
'

    log "VM setup complete"
}

# Stop VM and create snapshot
create_snapshot() {
    log "Stopping VM for snapshot..."
    gcloud compute instances stop "$TEMP_VM_NAME" \
        --project="$PROJECT" --zone="$ZONE"

    # Wait for VM to stop
    sleep 30

    log "Creating snapshot: $SNAPSHOT_NAME"
    gcloud compute snapshots create "$SNAPSHOT_NAME" \
        --project="$PROJECT" \
        --source-disk="$TEMP_VM_NAME" \
        --source-disk-zone="$ZONE" \
        --description="ClaraTeach agent snapshot created $(date)"

    log "Snapshot created: $SNAPSHOT_NAME"
}

# Main
main() {
    trap cleanup EXIT

    echo "=========================================="
    echo "ClaraTeach Agent Snapshot Creator"
    echo "=========================================="
    echo "Project:  $PROJECT"
    echo "Zone:     $ZONE"
    echo "Snapshot: $SNAPSHOT_NAME"
    echo "=========================================="
    echo ""

    check_prerequisites
    create_temp_vm
    upload_artifacts
    setup_vm
    create_snapshot

    echo ""
    echo "=========================================="
    echo -e "${GREEN}SUCCESS!${NC}"
    echo "=========================================="
    echo "Snapshot: $SNAPSHOT_NAME"
    echo ""
    echo "To use this snapshot, set:"
    echo "  export FC_SNAPSHOT_NAME=$SNAPSHOT_NAME"
    echo ""
    echo "The temporary VM will be deleted automatically."
    echo "=========================================="
}

main "$@"
