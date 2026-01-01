package rootfs

// DefaultInitScript is the default init script injected into rootfs images.
// It handles:
// - Mounting essential filesystems (proc, sys, dev, etc.)
// - Network configuration from kernel cmdline
// - DNS configuration
// - Starting the workspace server
const DefaultInitScript = `#!/bin/bash
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
# Firecracker provides eth0 with static IP via kernel params
# Format: ip=<client-ip>:<server-ip>:<gw-ip>:<netmask>:<hostname>:<device>:<autoconf>
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
    # Fallback: bring up interfaces with defaults
    ip link set lo up
    ip link set eth0 up
    ip addr add 172.16.0.2/24 dev eth0 || true
    ip route add default via 172.16.0.1 || true
fi

# Configure DNS (use Google DNS as fallback)
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
chown learner:learner /workspace 2>/dev/null || true

# Log startup
echo "ClaraTeach VM initialized"
echo "IP: $(ip -4 addr show eth0 2>/dev/null | grep -oP '(?<=inet\s)\d+(\.\d+){3}' || echo 'not configured')"

# Execute the workspace server as learner user
cd /home/learner/server 2>/dev/null || cd /home/learner || cd /
exec su -s /bin/bash learner -c "exec node dist/index.js" 2>/dev/null || exec /bin/bash
`
