#!/bin/bash

set -euo pipefail

# Configuration
IMAGES_DIR="/var/lib/clarateach/images"
ROOTFS_FILE="rootfs.ext4"
ROOTFS_SIZE="2G" # 2GB rootfs
IMAGE_NAME="clarateach-workspace" # Name of your Docker image
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKSPACE_DIR="${SCRIPT_DIR}/../workspace"

# --- Sanity Checks ---
command -v docker >/dev/null 2>&1 || { echo >&2 "Docker is not installed. Aborting."; exit 1; }
# mkfs.ext4 is in /usr/sbin which may not be in PATH
MKFS_EXT4=$(command -v mkfs.ext4 2>/dev/null || echo "/usr/sbin/mkfs.ext4")
[ -x "$MKFS_EXT4" ] || { echo >&2 "mkfs.ext4 is not installed (e.g., e2fsprogs). Aborting."; exit 1; }
command -v bsdtar >/dev/null 2>&1 || { echo >&2 "bsdtar (libarchive-tools) is not installed. Aborting."; exit 1; }

# --- Create Temporary Directory ---
TMP_DIR=$(mktemp -d)
echo "Created temporary directory: ${TMP_DIR}"

# Ensure cleanup on exit
cleanup() {
    echo "Cleaning up..."
    sudo umount "${TMP_DIR}/mnt" || true
    rm -rf "${TMP_DIR}"
    echo "Cleanup complete."
}
trap cleanup EXIT

# --- Ensure images directory exists ---
mkdir -p "${IMAGES_DIR}"

# --- Build Docker Image if it doesn't exist ---
if [[ "$(docker images -q ${IMAGE_NAME} 2> /dev/null)" == "" ]]; then
    echo "Docker image ${IMAGE_NAME} not found. Building from ${WORKSPACE_DIR}..."
    docker build -t ${IMAGE_NAME} "${WORKSPACE_DIR}"
else
    echo "Docker image ${IMAGE_NAME} found."
fi

# --- Create RootFS Disk Image ---
echo "Creating empty ext4 file: ${ROOTFS_FILE} (size: ${ROOTFS_SIZE})..."
truncate -s "${ROOTFS_SIZE}" "${TMP_DIR}/${ROOTFS_FILE}"
"$MKFS_EXT4" -F "${TMP_DIR}/${ROOTFS_FILE}"

# --- Mount RootFS ---
mkdir -p "${TMP_DIR}/mnt"
sudo mount "${TMP_DIR}/${ROOTFS_FILE}" "${TMP_DIR}/mnt"
echo "Mounted ${ROOTFS_FILE} to ${TMP_DIR}/mnt"

# --- Export Docker Container Filesystem ---
echo "Exporting Docker image ${IMAGE_NAME} to rootfs..."
CONTAINER_ID=$(docker create ${IMAGE_NAME})
sudo docker export "${CONTAINER_ID}" | sudo bsdtar -xf - -C "${TMP_DIR}/mnt"
docker rm "${CONTAINER_ID}" >/dev/null

# --- Inject init system for Firecracker ---
# Create a proper init script that sets up networking and runs the workspace server
INIT_SCRIPT="/sbin/init"
cat << 'INITEOF' | sudo tee "${TMP_DIR}/mnt/${INIT_SCRIPT}"
#!/bin/bash
set -euo pipefail

# Mount essential filesystems
mount -t proc none /proc
mount -t sysfs none /sys
mount -t devtmpfs none /dev
mkdir -p /dev/pts
mount -t devpts devpts /dev/pts
mount -t tmpfs tmpfs /run
mount -t tmpfs tmpfs /tmp

# Set hostname (will be overwritten by metadata if available)
hostname clarateach-vm

# Configure networking
# Firecracker provides eth0 with DHCP or static IP via kernel params
# Check for IP from kernel cmdline (format: ip=<client-ip>:<server-ip>:<gw-ip>:<netmask>:<hostname>:<device>:<autoconf>)
CMDLINE=$(cat /proc/cmdline)
if echo "$CMDLINE" | grep -q "ip="; then
    # Extract IP configuration from kernel cmdline
    IP_CONFIG=$(echo "$CMDLINE" | grep -oE 'ip=[^ ]+' | cut -d= -f2)
    CLIENT_IP=$(echo "$IP_CONFIG" | cut -d: -f1)
    GATEWAY_IP=$(echo "$IP_CONFIG" | cut -d: -f3)
    NETMASK=$(echo "$IP_CONFIG" | cut -d: -f4)

    if [ -n "$CLIENT_IP" ] && [ -n "$NETMASK" ]; then
        echo "Configuring network: $CLIENT_IP/$NETMASK gateway $GATEWAY_IP"
        ip link set lo up
        ip link set eth0 up
        ip addr add "${CLIENT_IP}/${NETMASK}" dev eth0
        if [ -n "$GATEWAY_IP" ]; then
            ip route add default via "$GATEWAY_IP"
        fi
    fi
else
    # Fallback: bring up interfaces and try DHCP or use defaults
    ip link set lo up
    ip link set eth0 up
    # Simple static fallback for testing (can be overridden)
    ip addr add 172.16.0.2/24 dev eth0 || true
    ip route add default via 172.16.0.1 || true
fi

# Configure DNS (use Google DNS as fallback, can be customized)
echo "nameserver 8.8.8.8" > /etc/resolv.conf
echo "nameserver 8.8.4.4" >> /etc/resolv.conf

# Export environment variables for the workspace
export HOME=/home/learner
export PATH="/home/learner/.local/bin:/home/learner/.npm-global/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
export WORKSPACE_DIR=/workspace
export TERM=xterm-256color
export NODE_ENV=production

# Create workspace directory if it doesn't exist
mkdir -p /workspace
chown learner:learner /workspace

# Log startup
echo "ClaraTeach VM initialized"
echo "IP: $(ip -4 addr show eth0 | grep -oP '(?<=inet\s)\d+(\.\d+){3}' || echo 'not configured')"

# Execute the workspace server as learner user
cd /home/learner/server
exec su -s /bin/bash learner -c "exec node dist/index.js"
INITEOF
sudo chmod +x "${TMP_DIR}/mnt/${INIT_SCRIPT}"
echo "Injected Firecracker init script to ${TMP_DIR}/mnt/${INIT_SCRIPT}"

# --- Ensure required system utilities exist ---
# Copy busybox utilities if 'ip' command is missing (for minimal images)
if [ ! -f "${TMP_DIR}/mnt/sbin/ip" ] && [ ! -f "${TMP_DIR}/mnt/usr/sbin/ip" ]; then
    echo "Warning: 'ip' command not found in rootfs. Installing iproute2..."
    # We need to install it via docker
    docker run --rm -v "${TMP_DIR}/mnt:/mnt" ${IMAGE_NAME} bash -c "apt-get update && apt-get install -y iproute2 && cp /sbin/ip /mnt/sbin/ || cp /usr/sbin/ip /mnt/sbin/" 2>/dev/null || true
fi

# --- Finalize ---
sudo umount "${TMP_DIR}/mnt"
echo "Unmounted ${ROOTFS_FILE}."
mv "${TMP_DIR}/${ROOTFS_FILE}" "${IMAGES_DIR}/${ROOTFS_FILE}"
echo "Moved ${ROOTFS_FILE} to ${IMAGES_DIR}/"

echo ""
echo "Rootfs build complete!"
echo "Output: ${IMAGES_DIR}/${ROOTFS_FILE}"
