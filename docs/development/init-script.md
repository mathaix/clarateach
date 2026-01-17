# MicroVM Init Script

This document describes the init script used by Firecracker MicroVMs in ClaraTeach.

## Overview

The init script (`/sbin/init`) is the first process run when a MicroVM boots. It:
1. Mounts essential filesystems
2. Configures networking
3. Sets environment variables
4. Starts the workspace server

## Critical Requirements

### 1. NO Strict Error Handling

**DO NOT use `set -e` or `set -euo pipefail`**

```bash
# BAD - Will cause silent failures
#!/bin/bash
set -euo pipefail

# GOOD - Handles errors gracefully
#!/bin/bash
exec 2>&1  # Redirect stderr to stdout for logging
```

**Why**: Minor errors (like missing commands or already-configured interfaces) will cause the entire init process to exit, leaving the MicroVM with no running services.

### 2. Required Environment Variables

These MUST be set for the workspace server to function correctly:

```bash
export MICROVM_MODE=true      # CRITICAL - Changes route registration
export AUTH_DISABLED=true     # Auth handled at backend level
export HOME=/home/learner
export WORKSPACE_DIR=/workspace
export NODE_ENV=production
export TERM=xterm-256color
export PATH="/home/learner/.local/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
```

### 3. MICROVM_MODE Explained

| MICROVM_MODE | Routes Registered | Used When |
|--------------|-------------------|-----------|
| `false`/unset | `/vm/:seat/files` | Docker/container deployment |
| `true` | `/files` | Firecracker MicroVM |

The agent proxy expects routes at `/files` and `/terminal` (without seat prefix), so `MICROVM_MODE=true` is **mandatory**.

## Reference Implementation

```bash
#!/bin/bash
# ClaraTeach MicroVM Init Script
# Note: No set -e to prevent early exit on minor errors

exec 2>&1

echo "ClaraTeach MicroVM starting..."

# Mount essential filesystems
mount -t proc none /proc 2>/dev/null || true
mount -t sysfs none /sys 2>/dev/null || true
mount -t devtmpfs none /dev 2>/dev/null || true
mkdir -p /dev/pts 2>/dev/null || true
mount -t devpts devpts /dev/pts 2>/dev/null || true
mount -t tmpfs tmpfs /run 2>/dev/null || true
mount -t tmpfs tmpfs /tmp 2>/dev/null || true

echo "Filesystems mounted"

# Set hostname
hostname clarateach-vm 2>/dev/null || true

# Configure networking from kernel cmdline
CMDLINE=$(cat /proc/cmdline 2>/dev/null || echo "")
echo "Kernel cmdline: $CMDLINE"

if echo "$CMDLINE" | grep -q "ip="; then
    IP_CONFIG=$(echo "$CMDLINE" | grep -oE 'ip=[^ ]+' | cut -d= -f2)
    CLIENT_IP=$(echo "$IP_CONFIG" | cut -d: -f1)
    GATEWAY_IP=$(echo "$IP_CONFIG" | cut -d: -f3)

    echo "Configuring network: IP=$CLIENT_IP, Gateway=$GATEWAY_IP"

    ip link set lo up 2>/dev/null || true
    ip link set eth0 up 2>/dev/null || true
    ip addr add "${CLIENT_IP}/24" dev eth0 2>/dev/null || true
    ip route add default via "$GATEWAY_IP" 2>/dev/null || true
fi

# Configure DNS
echo "nameserver 8.8.8.8" > /etc/resolv.conf 2>/dev/null || true

# Export environment - CRITICAL FOR WORKSPACE SERVER
export HOME=/home/learner
export PATH="/home/learner/.local/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
export WORKSPACE_DIR=/workspace
export TERM=xterm-256color
export NODE_ENV=production
export MICROVM_MODE=true        # CRITICAL - Required for correct route registration
export AUTH_DISABLED=true       # Auth handled at backend level

echo "Environment configured: MICROVM_MODE=$MICROVM_MODE"

# Create workspace
mkdir -p /workspace 2>/dev/null || true
chown -R learner:learner /workspace 2>/dev/null || true

# Check if server exists and start it
if [ -f /home/learner/server/dist/index.js ]; then
    echo "Starting workspace server..."
    cd /home/learner/server
    exec node dist/index.js
else
    echo "ERROR: Workspace server not found at /home/learner/server/dist/index.js"
    echo "Listing /home/learner:"
    ls -la /home/learner/ 2>/dev/null || echo "Cannot list"
    echo "Entering shell for debugging..."
    exec /bin/bash
fi
```

## Kernel Command Line Parameters

The init script reads network configuration from kernel cmdline passed by Firecracker:

```
ip=192.168.100.11::192.168.100.1:255.255.255.0::eth0:off
```

Format: `ip=<client-ip>:<server-ip>:<gateway>:<netmask>:<hostname>:<device>:<autoconf>`

## Updating the Init Script

### Method 1: Via Code (Recommended)

The init script template is in `backend/internal/rootfs/initscript.go`:

```go
const DefaultInitScript = `#!/bin/bash
# ... script content ...
`
```

After modifying, rebuild rootfs:

```bash
cd backend
go run ./cmd/rootfs-builder/ \
  --docker-image=clarateach-workspace \
  --output=/var/lib/clarateach/images/rootfs.ext4
```

### Method 2: Direct Modification (Quick Fix)

For quick fixes on an existing rootfs:

```bash
# Mount the rootfs
sudo mkdir -p /mnt/rootfs
sudo mount /var/lib/clarateach/images/rootfs.ext4 /mnt/rootfs

# Edit the init script
sudo nano /mnt/rootfs/sbin/init

# Ensure executable
sudo chmod 755 /mnt/rootfs/sbin/init

# Unmount
sudo umount /mnt/rootfs
```

## Testing the Init Script

### 1. Unit Test (Without Booting)

Check script syntax:

```bash
bash -n /path/to/init-script
```

### 2. Integration Test

Boot a MicroVM and check:

```bash
# Check if server responds
curl http://192.168.100.11:3001/health
# Expected: {"status":"ok","workspace":"/workspace"}

curl http://192.168.100.11:3002/files
# Expected: {"files":[...]}
```

### 3. Debug Mode

If the workspace server isn't found, the script falls back to a shell. You can then SSH into the agent VM and connect to the MicroVM console for debugging.

## Common Issues

### Issue: MicroVM Exits Immediately (status=0)

**Cause**: Script using `set -e` and a command failed

**Solution**: Remove strict error handling, use `|| true` for non-critical commands

### Issue: 404 on /files Endpoint

**Cause**: `MICROVM_MODE=true` not set

**Solution**: Add `export MICROVM_MODE=true` before starting workspace server

### Issue: Terminal Shows "Connection Closed"

**Cause**: Workspace server not running or wrong routes

**Solution**:
1. Check MICROVM_MODE is set
2. Verify workspace server process is running
3. Check init script didn't exit early

### Issue: Network Not Working in MicroVM

**Cause**: Network configuration failed

**Solution**:
1. Check kernel cmdline includes `ip=` parameter
2. Verify `iproute2` package is installed in rootfs
3. Check gateway is reachable

## Filesystem Layout

The init script expects this filesystem structure in the rootfs:

```
/
├── sbin/
│   └── init              # This script (must be executable)
├── home/
│   └── learner/
│       ├── server/
│       │   └── dist/
│       │       └── index.js  # Workspace server
│       └── .local/
│           └── bin/      # User binaries
├── workspace/            # Learner's working directory
├── proc/                 # Mounted at boot
├── sys/                  # Mounted at boot
├── dev/                  # Mounted at boot
├── tmp/                  # Mounted at boot
└── etc/
    └── resolv.conf       # Created at boot
```

## Security Considerations

- `AUTH_DISABLED=true` is safe because authentication is handled at the backend/agent level
- MicroVMs are isolated via Firecracker's security model
- Each learner gets their own MicroVM with separate filesystem
- Network isolation via unique tap interfaces
