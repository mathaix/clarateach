#!/bin/bash

set -euo pipefail

# Configuration
ROOTFS_FILE="rootfs.ext4"
ROOTFS_SIZE="2G" # 2GB rootfs
IMAGE_NAME="clarateach-workspace" # Name of your Docker image

# --- Sanity Checks ---
command -v docker >/dev/null 2>&1 || { echo >&2 "Docker is not installed. Aborting."; exit 1; }
command -v mkfs.ext4 >/dev/null 2>&1 || { echo >&2 "mkfs.ext4 is not installed (e.g., e2fsprogs). Aborting."; exit 1; }
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

# --- Build Docker Image if it doesn't exist ---
if [[ "$(docker images -q ${IMAGE_NAME} 2> /dev/null)" == "" ]]; then
    echo "Docker image ${IMAGE_NAME} not found. Building from workspace/Dockerfile..."
    docker build -t ${IMAGE_NAME} ../workspace
else
    echo "Docker image ${IMAGE_NAME} found."
fi

# --- Create RootFS Disk Image ---
echo "Creating empty ext4 file: ${ROOTFS_FILE} (size: ${ROOTFS_SIZE})..."
dd if=/dev/zero of="${TMP_DIR}/${ROOTFS_FILE}" bs=1M count=$(echo "${ROOTFS_SIZE}" | sed 's/G/*1024/g' | bc) # Convert G to M
mkfs.ext4 -F "${TMP_DIR}/${ROOTFS_FILE}"

# --- Mount RootFS ---
mkdir -p "${TMP_DIR}/mnt"
sudo mount "${TMP_DIR}/${ROOTFS_FILE}" "${TMP_DIR}/mnt"
echo "Mounted ${ROOTFS_FILE} to ${TMP_DIR}/mnt"

# --- Export Docker Container Filesystem ---
echo "Exporting Docker image ${IMAGE_NAME} to rootfs..."
CONTAINER_ID=$(docker create ${IMAGE_NAME})
sudo docker export "${CONTAINER_ID}" | sudo bsdtar -xf - -C "${TMP_DIR}/mnt"
docker rm "${CONTAINER_ID}" >/dev/null

# --- Inject init system (simplified for now) ---
# The Firecracker plan mentions OpenRC, but for a basic script, we'll make /sbin/init
# a simple script that executes the existing entrypoint.sh
# This needs to be robust for a real system, but serves as a starting point.
INIT_SCRIPT="/sbin/init"
cat << EOF | sudo tee "${TMP_DIR}/mnt/${INIT_SCRIPT}"
#!/bin/bash
set -euo pipefail
# Mount essential filesystems
mount -t proc none /proc
mount -t sysfs none /sys
mount -t devtmpfs none /dev

# Execute the original entrypoint script as the learner user
exec /bin/su -c "/home/learner/entrypoint.sh" learner
EOF
sudo chmod +x "${TMP_DIR}/mnt/${INIT_SCRIPT}"
echo "Injected simplified init script to ${TMP_DIR}/mnt/${INIT_SCRIPT}"

# --- Finalize ---
sudo umount "${TMP_DIR}/mnt"
echo "Unmounted ${ROOTFS_FILE}. Rootfs is ready at ${TMP_DIR}/${ROOTFS_FILE}"
mv "${TMP_DIR}/${ROOTFS_FILE}" ./${ROOTFS_FILE}
echo "Moved ${ROOTFS_FILE} to current directory."

echo "Rootfs build complete!"
