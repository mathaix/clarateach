#!/bin/bash
set -euo pipefail

# Firecracker setup script for ClaraTeach
# Downloads and installs Firecracker binary and Linux kernel

FIRECRACKER_VERSION="${FIRECRACKER_VERSION:-v1.10.1}"
KERNEL_VERSION="6.1.102"
IMAGES_DIR="/var/lib/clarateach/images"

echo "=== ClaraTeach Firecracker Setup ==="
echo "Firecracker version: ${FIRECRACKER_VERSION}"
echo "Kernel version: ${KERNEL_VERSION}"
echo "Images directory: ${IMAGES_DIR}"
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root (sudo)"
    exit 1
fi

# Check architecture
ARCH=$(uname -m)
if [ "$ARCH" != "x86_64" ] && [ "$ARCH" != "aarch64" ]; then
    echo "Error: Unsupported architecture: $ARCH"
    echo "Firecracker only supports x86_64 and aarch64"
    exit 1
fi

# Map architecture names
if [ "$ARCH" = "x86_64" ]; then
    FC_ARCH="x86_64"
else
    FC_ARCH="aarch64"
fi

# Create images directory
mkdir -p "${IMAGES_DIR}"

# --- Download Firecracker ---
echo "Downloading Firecracker ${FIRECRACKER_VERSION}..."
FC_URL="https://github.com/firecracker-microvm/firecracker/releases/download/${FIRECRACKER_VERSION}/firecracker-${FIRECRACKER_VERSION}-${FC_ARCH}.tgz"
TMP_DIR=$(mktemp -d)
trap "rm -rf ${TMP_DIR}" EXIT

curl -fsSL "${FC_URL}" -o "${TMP_DIR}/firecracker.tgz"
tar -xzf "${TMP_DIR}/firecracker.tgz" -C "${TMP_DIR}"

# Install binaries
cp "${TMP_DIR}/release-${FIRECRACKER_VERSION}-${FC_ARCH}/firecracker-${FIRECRACKER_VERSION}-${FC_ARCH}" /usr/local/bin/firecracker
cp "${TMP_DIR}/release-${FIRECRACKER_VERSION}-${FC_ARCH}/jailer-${FIRECRACKER_VERSION}-${FC_ARCH}" /usr/local/bin/jailer
chmod +x /usr/local/bin/firecracker /usr/local/bin/jailer

echo "Installed firecracker and jailer to /usr/local/bin/"
firecracker --version

# --- Download Kernel ---
echo ""
echo "Downloading Linux kernel ${KERNEL_VERSION}..."
KERNEL_URL="https://s3.amazonaws.com/spec.ccfc.min/firecracker-ci/v1.11/${FC_ARCH}/vmlinux-${KERNEL_VERSION}"
curl -fsSL "${KERNEL_URL}" -o "${IMAGES_DIR}/vmlinux"
chmod 644 "${IMAGES_DIR}/vmlinux"
echo "Installed kernel to ${IMAGES_DIR}/vmlinux"

# --- Check KVM ---
echo ""
echo "Checking KVM support..."
if [ ! -e /dev/kvm ]; then
    echo "Warning: /dev/kvm not found. Firecracker requires KVM."
    echo "If running on a VM, enable nested virtualization."
    echo "On GCP: gcloud compute instances update INSTANCE --enable-nested-virtualization"
else
    echo "KVM is available at /dev/kvm"
    # Ensure current user can access KVM
    if [ -n "${SUDO_USER:-}" ]; then
        usermod -aG kvm "${SUDO_USER}" 2>/dev/null || true
        echo "Added ${SUDO_USER} to kvm group"
    fi
fi

# --- Summary ---
echo ""
echo "=== Setup Complete ==="
echo ""
echo "Installed:"
echo "  - /usr/local/bin/firecracker"
echo "  - /usr/local/bin/jailer"
echo "  - ${IMAGES_DIR}/vmlinux"
echo ""
echo "Next steps:"
echo "  1. Build rootfs:  sudo ./scripts/build-rootfs.sh"
echo "  2. Verify KVM:    ls -la /dev/kvm"
echo "  3. Test:          firecracker --help"
echo ""
