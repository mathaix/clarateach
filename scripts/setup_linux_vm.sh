#!/bin/bash
set -e

# ClaraTeach Linux VM Setup Script
# This script installs Go, Docker, Firecracker, and KVM dependencies.
# Intended for Ubuntu 22.04 or 24.04 LTS.

echo ">>> Updating System..."
sudo apt-get update && sudo apt-get upgrade -y

echo ">>> Installing Core Tools..."
sudo apt-get install -y \
    curl \
    wget \
    git \
    build-essential \
    jq \
    unzip \
    pkg-config \
    qemu-kvm \
    libvirt-daemon-system \
    libvirt-clients \
    bridge-utils

echo ">>> Installing Go (1.24)..."
GO_VERSION="1.24.0"
wget "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz"
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf "go${GO_VERSION}.linux-amd64.tar.gz"
rm "go${GO_VERSION}.linux-amd64.tar.gz"

# Add Go to PATH for current session and future sessions
export PATH=$PATH:/usr/local/go/bin
if ! grep -q "/usr/local/go/bin" ~/.bashrc; then
    echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
fi

echo ">>> Installing Docker..."
if ! command -v docker &> /dev/null; then
    curl -fsSL https://get.docker.com | sh
    sudo usermod -aG docker $USER
else
    echo "Docker already installed."
fi

echo ">>> Verifying KVM support..."
if [ -c /dev/kvm ]; then
    echo "KVM is available (/dev/kvm found)."
    sudo adduser $USER kvm
else
    echo "CRITICAL WARNING: /dev/kvm not found!"
    echo "If you are on GCP, ensure you used --enable-nested-virtualization."
fi

echo ">>> Installing Firecracker (Latest)..."
release_url="https://github.com/firecracker-microvm/firecracker/releases"
latest=$(basename $(curl -fsSL -o /dev/null -w %{url_effective} ${release_url}/latest))
arch=$(uname -m)
curl -L ${release_url}/download/${latest}/firecracker-${latest}-${arch}.tgz | tar -xz
mv release-${latest}-${arch}/firecracker-${latest}-${arch} firecracker
mv release-${latest}-${arch}/jailer-${latest}-${arch} jailer
sudo mv firecracker /usr/local/bin/firecracker
sudo mv jailer /usr/local/bin/jailer
rm -rf release-${latest}-${arch}

echo ">>> Installing CNI Plugins..."
sudo mkdir -p /opt/cni/bin
curl -L https://github.com/containernetworking/plugins/releases/download/v1.4.0/cni-plugins-linux-amd64-v1.4.0.tgz | sudo tar -C /opt/cni/bin -xz

echo ">>> Setup Complete!"
echo "--------------------------------------------------------"
echo "IMPORTANT: Please log out and log back in (or run 'newgrp docker' and 'newgrp kvm')"
echo "to apply group membership changes for Docker and KVM."
echo "Then run: source ~/.bashrc"
echo "--------------------------------------------------------"
