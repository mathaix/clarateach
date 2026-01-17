# Update GCP Snapshot Procedure

This document describes how to update the GCP snapshot used for provisioning Firecracker agent VMs.

## When to Update

Update the snapshot when:
- Workspace server code changes
- Init script modifications
- Agent server updates
- Kernel or rootfs changes
- Security patches needed

## Prerequisites

- GCP project access with Compute Engine permissions
- SSH access to an existing agent VM (e.g., `clara2`)
- `gcloud` CLI configured
- Go toolchain (for building agent/rootfs tools)

## Quick Reference

```bash
# Current snapshot (check start-backend.sh)
FC_SNAPSHOT_NAME=clarateach-agent-20260116-204056

# Agent VM for building
AGENT_VM=clara2
GCP_ZONE=us-central1-b
```

## Procedure

### Step 1: Prepare the Agent VM

Start the agent VM if not running:

```bash
gcloud compute instances start clara2 --zone=us-central1-b
```

SSH into the VM:

```bash
gcloud compute ssh clara2 --zone=us-central1-b
```

### Step 2: Update Code (if needed)

#### Option A: Update Workspace Server

If workspace server changed, rebuild the rootfs:

```bash
# On your local machine, build and push Docker image
cd workspace
docker build -t us-central1-docker.pkg.dev/clarateach/clarateach/clarateach-workspace:latest .
docker push us-central1-docker.pkg.dev/clarateach/clarateach/clarateach-workspace:latest

# On the agent VM
cd /home/mantiz/ClaraTeach/backend
sudo go run ./cmd/rootfs-builder/ \
  --docker-image=clarateach-workspace \
  --output=/var/lib/clarateach/images/rootfs.ext4 \
  --size=2G
```

#### Option B: Update Init Script Only

If only the init script changed:

```bash
# On the agent VM
sudo mkdir -p /mnt/rootfs
sudo mount /var/lib/clarateach/images/rootfs.ext4 /mnt/rootfs

# Edit the init script
sudo nano /mnt/rootfs/sbin/init

# Or copy a new one
sudo cp /path/to/new/init /mnt/rootfs/sbin/init
sudo chmod 755 /mnt/rootfs/sbin/init

sudo umount /mnt/rootfs
```

#### Option C: Update Agent Server

```bash
# On your local machine
cd backend
GOOS=linux GOARCH=amd64 go build -o agent ./cmd/agent/
scp agent clara2:/home/mantiz/ClaraTeach/backend/

# On the agent VM
sudo systemctl restart clarateach-agent
```

### Step 3: Test the Changes

Before creating a snapshot, verify everything works:

```bash
# Test MicroVM boot
curl http://localhost:9090/health

# Create a test MicroVM
curl -X POST http://localhost:9090/vms \
  -H "Content-Type: application/json" \
  -d '{"workshop_id": "test-ws", "seat_id": 1}'

# Test endpoints
curl http://192.168.100.11:3001/health
curl http://192.168.100.11:3002/files

# Cleanup
curl -X DELETE http://localhost:9090/vms/test-ws/1
```

### Step 4: Prepare for Snapshot

Stop services and clean up:

```bash
# Stop agent service
sudo systemctl stop clarateach-agent

# Clean up any running MicroVMs
sudo pkill -f firecracker || true

# Clean up tap interfaces
for tap in $(ip link show | grep tap | cut -d: -f2 | tr -d ' '); do
  sudo ip link delete $tap 2>/dev/null || true
done

# Clear logs
sudo truncate -s 0 /var/log/clarateach-agent.log

# Sync filesystem
sudo sync
```

### Step 5: Create the Snapshot

From your local machine (not the VM):

```bash
# Generate snapshot name with timestamp
SNAPSHOT_NAME="clarateach-agent-$(date +%Y%m%d-%H%M%S)"

# Stop the VM
gcloud compute instances stop clara2 --zone=us-central1-b

# Create snapshot from the boot disk
gcloud compute snapshots create $SNAPSHOT_NAME \
  --source-disk=clara2 \
  --source-disk-zone=us-central1-b \
  --description="ClaraTeach agent snapshot $(date)"

# Verify snapshot created
gcloud compute snapshots describe $SNAPSHOT_NAME

# Start the VM again
gcloud compute instances start clara2 --zone=us-central1-b

echo "Snapshot created: $SNAPSHOT_NAME"
```

### Step 6: Update Configuration

Update the backend to use the new snapshot:

```bash
# Edit start-backend.sh
vi scripts/start-backend.sh

# Change the FC_SNAPSHOT_NAME line:
export FC_SNAPSHOT_NAME="${FC_SNAPSHOT_NAME:-clarateach-agent-YYYYMMDD-HHMMSS}"
```

Or set via environment variable:

```bash
export FC_SNAPSHOT_NAME=clarateach-agent-YYYYMMDD-HHMMSS
./scripts/start-backend.sh
```

### Step 7: Verify New Workshops Work

1. Create a new workshop in the UI
2. Wait for VM provisioning
3. Register and launch workspace
4. Verify terminal and file explorer work

## Rollback Procedure

If the new snapshot has issues:

```bash
# List available snapshots
gcloud compute snapshots list --filter="name~clarateach-agent"

# Update config to use previous snapshot
export FC_SNAPSHOT_NAME=clarateach-agent-PREVIOUS-TIMESTAMP
./scripts/start-backend.sh
```

## Snapshot Management

### List Snapshots

```bash
gcloud compute snapshots list --filter="name~clarateach-agent" \
  --format="table(name,creationTimestamp,diskSizeGb,status)"
```

### Delete Old Snapshots

Keep at least the last 3 snapshots for rollback:

```bash
# Delete a specific snapshot
gcloud compute snapshots delete clarateach-agent-OLD-TIMESTAMP
```

### Snapshot Naming Convention

Format: `clarateach-agent-YYYYMMDD-HHMMSS`

Example: `clarateach-agent-20260116-204056`

## Troubleshooting

### Snapshot Creation Fails

```bash
# Check disk status
gcloud compute disks describe clara2 --zone=us-central1-b

# Ensure VM is stopped
gcloud compute instances describe clara2 --zone=us-central1-b --format="get(status)"
```

### New VMs Don't Boot

1. Check the snapshot was created from a working state
2. Verify init script is correct
3. Check agent logs on new VM

### Workspace Server Not Starting

SSH into a newly provisioned VM and check:

```bash
# Check if Firecracker is running
ps aux | grep firecracker

# Check MicroVM console output
cat /var/log/firecracker-*.log

# Manually test MicroVM
curl http://192.168.100.11:3001/health
```

## Automation (Future)

Consider automating this process with:
- CI/CD pipeline triggered on code changes
- Packer for reproducible image builds
- Terraform for snapshot management
