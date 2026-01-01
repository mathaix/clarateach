#!/bin/bash
# prepare-snapshot.sh - Clean up VM before taking a GCP snapshot
# Run this on clara2 before stopping the VM for snapshotting
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() { echo -e "${GREEN}[✓]${NC} $1"; }
warn() { echo -e "${YELLOW}[!]${NC} $1"; }
info() { echo -e "    $1"; }

echo "=============================================="
echo "ClaraTeach VM Snapshot Preparation"
echo "=============================================="
echo ""

# Check if running as root or with sudo
if [[ $EUID -ne 0 ]]; then
    echo "This script must be run as root or with sudo"
    exit 1
fi

# Show disk usage before
echo "=== Disk Usage Before ==="
df -h / | tail -1
BEFORE=$(df / | tail -1 | awk '{print $3}')
echo ""

# 1. Stop services that might be writing files
echo "=== Stopping Services ==="
systemctl stop clarateach-agent 2>/dev/null || true
log "Stopped clarateach-agent"

# 2. Clean apt cache
echo ""
echo "=== Cleaning APT Cache ==="
apt-get clean
apt-get autoclean
apt-get autoremove -y 2>/dev/null || true
rm -rf /var/lib/apt/lists/*
log "Cleaned apt cache"

# 3. Clean Docker
echo ""
echo "=== Cleaning Docker ==="
if command -v docker &> /dev/null; then
    # Stop all containers
    docker ps -q | xargs -r docker stop 2>/dev/null || true
    # Remove stopped containers
    docker container prune -f 2>/dev/null || true
    # Remove unused images (keep clarateach-workspace)
    docker image prune -f 2>/dev/null || true
    # Remove build cache
    docker builder prune -f 2>/dev/null || true
    # Remove unused volumes
    docker volume prune -f 2>/dev/null || true
    log "Cleaned Docker"
    info "Remaining images:"
    docker images --format "  {{.Repository}}:{{.Tag}} ({{.Size}})"
else
    warn "Docker not installed"
fi

# 4. Clean temporary files
echo ""
echo "=== Cleaning Temporary Files ==="
rm -rf /tmp/* 2>/dev/null || true
rm -rf /var/tmp/* 2>/dev/null || true
rm -rf /root/.cache/* 2>/dev/null || true
rm -rf /home/*/.cache/* 2>/dev/null || true
log "Cleaned temporary files"

# 5. Clean logs
echo ""
echo "=== Cleaning Logs ==="
journalctl --vacuum-time=1d 2>/dev/null || true
find /var/log -type f -name "*.gz" -delete 2>/dev/null || true
find /var/log -type f -name "*.log.*" -delete 2>/dev/null || true
find /var/log -type f -name "*.1" -delete 2>/dev/null || true
# Truncate current logs instead of deleting
find /var/log -type f -name "*.log" -exec truncate -s 0 {} \; 2>/dev/null || true
truncate -s 0 /var/log/wtmp 2>/dev/null || true
truncate -s 0 /var/log/lastlog 2>/dev/null || true
log "Cleaned logs"

# 6. Clean Go cache (if exists)
echo ""
echo "=== Cleaning Go Cache ==="
rm -rf /root/go/pkg/mod/cache 2>/dev/null || true
rm -rf /home/*/go/pkg/mod/cache 2>/dev/null || true
log "Cleaned Go cache"

# 7. Clean build artifacts in clarateach directory
echo ""
echo "=== Cleaning Build Artifacts ==="
CLARATEACH_DIR="/home/mathewma/clarateach/backend"
if [[ -d "$CLARATEACH_DIR" ]]; then
    cd "$CLARATEACH_DIR"
    rm -f agent agent-linux rootfs-builder 2>/dev/null || true
    rm -rf /tmp/rootfs-build-* 2>/dev/null || true
    log "Cleaned build artifacts"
fi

# 8. Clean Firecracker sockets and temp files
echo ""
echo "=== Cleaning Firecracker Temp Files ==="
rm -rf /tmp/clarateach/* 2>/dev/null || true
mkdir -p /tmp/clarateach
chmod 755 /tmp/clarateach
log "Cleaned Firecracker temp files"

# 9. Clean SSH known hosts and bash history
echo ""
echo "=== Cleaning User Data ==="
rm -f /root/.bash_history 2>/dev/null || true
rm -f /home/*/.bash_history 2>/dev/null || true
rm -f /root/.ssh/known_hosts 2>/dev/null || true
rm -f /home/*/.ssh/known_hosts 2>/dev/null || true
log "Cleaned user data"

# 10. Verify essential files exist
echo ""
echo "=== Verifying Essential Files ==="
ESSENTIAL_FILES=(
    "/usr/local/bin/agent"
    "/usr/local/bin/firecracker"
    "/var/lib/clarateach/images/vmlinux"
    "/var/lib/clarateach/images/rootfs.ext4"
    "/etc/systemd/system/clarateach-agent.service"
)

for file in "${ESSENTIAL_FILES[@]}"; do
    if [[ -f "$file" ]]; then
        SIZE=$(du -h "$file" 2>/dev/null | cut -f1)
        log "$file ($SIZE)"
    else
        warn "MISSING: $file"
    fi
done

# 11. Show disk usage after
echo ""
echo "=== Disk Usage After ==="
sync
df -h / | tail -1
AFTER=$(df / | tail -1 | awk '{print $3}')

echo ""
echo "=============================================="
echo "Cleanup Complete!"
echo "=============================================="
echo ""
echo "Before: $BEFORE"
echo "After:  $AFTER"
echo ""
echo "Next steps:"
echo "  1. Review the output above"
echo "  2. Stop the VM:  sudo shutdown -h now"
echo "  3. Create snapshot from GCP Console or:"
echo ""
echo "     gcloud compute disks snapshot clara2-disk \\"
echo "       --project=clarateach \\"
echo "       --zone=us-central1-b \\"
echo "       --snapshot-names=clara2-snapshot-$(date +%Y%m%d)"
echo ""
