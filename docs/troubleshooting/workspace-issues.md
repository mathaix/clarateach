# Troubleshooting Workspace Issues

This guide helps diagnose and fix common issues with ClaraTeach workspaces.

## Quick Diagnostic Commands

```bash
# Check backend logs
tail -f /tmp/clarateach-backend.log

# Check agent VM status
gcloud compute instances describe clara2 --zone=us-central1-b --format="get(status)"

# Test agent health (replace IP)
curl http://AGENT_IP:9090/health

# Test MicroVM directly (on agent VM)
curl http://192.168.100.11:3001/health
curl http://192.168.100.11:3002/files
```

## Issue: Terminal Shows "Connection Closed"

### Symptoms
- Terminal panel shows "Connection closed" immediately after connecting
- No prompt appears
- WebSocket connection fails

### Diagnosis Steps

1. **Check if MicroVM is running** (on agent VM):
   ```bash
   ps aux | grep firecracker
   ```

2. **Check MicroVM health**:
   ```bash
   curl http://192.168.100.11:3001/health
   ```

3. **Check workspace server logs**:
   ```bash
   # Connect to Firecracker console (if available)
   cat /var/log/firecracker-*.log
   ```

### Common Causes & Solutions

| Cause | Solution |
|-------|----------|
| `MICROVM_MODE` not set | Add `export MICROVM_MODE=true` to init script |
| Workspace server crashed | Check init script, ensure Node.js is installed |
| Wrong port | Verify terminal expects port 3001 |
| Network issue | Check tap interface and IP configuration |

## Issue: 404 on File Explorer

### Symptoms
- File explorer shows error or empty
- Network tab shows 404 on `/files` endpoint
- Terminal may work but files don't

### Diagnosis Steps

1. **Test files endpoint directly**:
   ```bash
   # From agent VM
   curl http://192.168.100.11:3002/files
   ```

2. **Check route registration**:
   ```bash
   # Expected response with MICROVM_MODE=true:
   # {"files":[]}

   # Response without MICROVM_MODE (wrong routes):
   # 404 Not Found
   ```

### Solution

Ensure init script has:
```bash
export MICROVM_MODE=true
```

Then update rootfs and create new snapshot.

## Issue: MicroVM Exits Immediately

### Symptoms
- Firecracker process starts then exits
- Exit status is 0 (clean exit)
- No services running

### Diagnosis Steps

1. **Check Firecracker logs**:
   ```bash
   cat /var/log/firecracker-ws-xxxxx-1.log
   ```

2. **Check init script for strict error handling**:
   ```bash
   # Mount rootfs and check
   sudo mount /var/lib/clarateach/images/rootfs.ext4 /mnt/rootfs
   head -5 /mnt/rootfs/sbin/init
   ```

### Common Causes & Solutions

| Cause | Solution |
|-------|----------|
| `set -e` in init script | Remove strict error handling |
| Missing command (e.g., `ip`) | Install `iproute2` in Docker image |
| Missing workspace server | Rebuild rootfs with correct Docker image |

### Fix

Use robust init script without `set -e`:
```bash
#!/bin/bash
# No set -e
exec 2>&1
mount -t proc none /proc 2>/dev/null || true
# ... rest of script with || true for each command
```

## Issue: Workshop Stuck in "Provisioning"

### Symptoms
- Workshop status shows "provisioning" indefinitely
- No VM created in GCP
- Backend logs show errors

### Diagnosis Steps

1. **Check backend logs**:
   ```bash
   tail -50 /tmp/clarateach-backend.log | grep -i error
   ```

2. **Check GCP quotas**:
   ```bash
   gcloud compute regions describe us-central1 --format="get(quotas)"
   ```

3. **Check snapshot exists**:
   ```bash
   gcloud compute snapshots describe clarateach-agent-YYYYMMDD-HHMMSS
   ```

### Common Causes & Solutions

| Cause | Solution |
|-------|----------|
| Snapshot not found | Update `FC_SNAPSHOT_NAME` to valid snapshot |
| GCP quota exceeded | Request quota increase or delete old VMs |
| Network/API error | Check GCP credentials and permissions |

## Issue: Browser Preview Shows 404

### Symptoms
- Browser preview panel shows "404 page not found"
- Terminal and files work fine

### Explanation

This is **expected behavior** when no web server is running in the workspace. The browser preview connects to a preview port (usually 8080) inside the MicroVM.

### Solution

Start a web server in the terminal:
```bash
# Simple Python server
python3 -m http.server 8080

# Or Node.js
npx serve -p 8080
```

## Issue: Slow VM Provisioning

### Symptoms
- Workshop takes 60+ seconds to provision
- User sees long loading time

### Diagnosis

Check provisioning time in logs:
```bash
grep "VM created" /tmp/clarateach-backend.log
# Example: VM created: clarateach-fc-ws-xxxxx (IP: x.x.x.x) in 75465ms
```

### Optimization Options

1. **Use smaller disk size** in VM config
2. **Pre-warm VMs** (keep standby pool)
3. **Use regional persistent disk** for faster boot

## Issue: Cannot SSH to Agent VM

### Symptoms
- `gcloud compute ssh` fails
- VM appears running but unreachable

### Diagnosis Steps

1. **Check VM status**:
   ```bash
   gcloud compute instances describe clara2 --zone=us-central1-b
   ```

2. **Check firewall rules**:
   ```bash
   gcloud compute firewall-rules list --filter="name~clarateach"
   ```

3. **Check serial console**:
   ```bash
   gcloud compute instances get-serial-port-output clara2 --zone=us-central1-b
   ```

### Solutions

- Restart VM: `gcloud compute instances reset clara2 --zone=us-central1-b`
- Check SSH keys: `gcloud compute config-ssh`

## Issue: Files Not Persisting

### Symptoms
- Files created in workspace disappear
- Changes lost after refresh

### Explanation

Each MicroVM has an ephemeral filesystem. Files are lost when:
- MicroVM is restarted
- Workshop is stopped
- VM is terminated

### Future Solution

Implement persistent storage via:
- Mounted GCS buckets
- Network-attached storage
- Periodic backup to cloud storage

## Debugging Checklist

When workspace issues occur, check in order:

1. **Backend running?**
   ```bash
   curl http://localhost:8080/api/health
   ```

2. **Agent VM running?**
   ```bash
   gcloud compute instances describe clara2 --zone=us-central1-b --format="get(status)"
   ```

3. **Agent server responding?**
   ```bash
   curl http://AGENT_IP:9090/health
   ```

4. **MicroVM running?** (on agent VM)
   ```bash
   ps aux | grep firecracker
   ```

5. **Workspace server responding?** (on agent VM)
   ```bash
   curl http://192.168.100.11:3001/health
   curl http://192.168.100.11:3002/files
   ```

6. **Correct environment?** (on agent VM)
   ```bash
   # Check init script
   sudo mount /var/lib/clarateach/images/rootfs.ext4 /mnt/rootfs
   grep MICROVM_MODE /mnt/rootfs/sbin/init
   sudo umount /mnt/rootfs
   ```

## Getting Help

If issues persist:

1. Collect logs:
   - Backend: `/tmp/clarateach-backend.log`
   - Agent: `/var/log/clarateach-agent.log`
   - Firecracker: `/var/log/firecracker-*.log`

2. Note the workshop ID and seat ID

3. Check the timestamp when issue occurred

4. Review recent changes to code or configuration
