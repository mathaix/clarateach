# ClaraTeach Backend

Go-based backend for ClaraTeach. Supports both Docker containers and Firecracker MicroVMs for isolated learner environments.

## Binaries

| Binary | Purpose | Runs On |
|--------|---------|---------|
| `cmd/server/` | Control Plane API | Cloud Run / Controller VM |
| `cmd/agent/` | Worker Agent (Firecracker) | Worker VMs (KVM-enabled) |
| `cmd/rootfs-builder/` | Build rootfs images | Build machine |
| `cmd/vmctl/` | GCP VM management CLI | Developer machine |

### Build All

```bash
cd backend
go build ./...
```

---

## cmd/server (Control Plane)

The main API server that handles workshops, sessions, and orchestration.

```bash
# Build
go build -o server ./cmd/server/

# Run
./server
```

**Environment Variables:**
| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | HTTP listen port | `8080` |
| `DATABASE_URL` | SQLite database path | `./clarateach.db` |
| `GCP_PROJECT` | GCP project ID | - |
| `GCP_ZONE` | GCP zone | `us-central1-a` |
| `WORKER_AGENTS` | JSON array of worker configs (distributed mode) | - |

**Distributed Mode (Firecracker):**
```bash
WORKER_AGENTS='[{"address":"10.0.0.10:9090","token":"secret1"}]' ./server
```

---

## cmd/agent (Worker Agent)

Runs on KVM-enabled VMs to manage local Firecracker MicroVMs. Exposes HTTP API on port 9090.

```bash
# Build
go build -o agent ./cmd/agent/

# Run
./agent
```

**Environment Variables:**
| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | HTTP listen port | `9090` |
| `AGENT_TOKEN` | Auth token (or via GCP metadata) | - |
| `IMAGES_DIR` | Kernel/rootfs directory | `/var/lib/clarateach/images` |
| `SOCKET_DIR` | Firecracker socket directory | `/tmp/clarateach` |
| `BRIDGE_NAME` | Network bridge name | `clarateach0` |
| `BRIDGE_IP` | Bridge IP CIDR | `192.168.100.1/24` |
| `CAPACITY` | Max VMs per worker | `50` |

**API Endpoints:**
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/health` | No | Health check |
| GET | `/info` | Yes | Worker info |
| POST | `/vms` | Yes | Create VM |
| GET | `/vms` | Yes | List VMs |
| GET | `/vms/{workshopID}/{seatID}` | Yes | Get VM |
| DELETE | `/vms/{workshopID}/{seatID}` | Yes | Destroy VM |

---

## cmd/rootfs-builder

Builds ext4 rootfs images for Firecracker from Docker images.

```bash
# Build
go build -o rootfs-builder ./cmd/rootfs-builder/

# Create rootfs from existing Docker image
sudo ./rootfs-builder --image clarateach-workspace --output ./rootfs.ext4

# Build from Dockerfile
sudo ./rootfs-builder --dockerfile ./workspace/Dockerfile --output ./rootfs.ext4 --size 4G
```

**Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `--image` | Docker image name | (required) |
| `--dockerfile` | Path to Dockerfile | - |
| `--output` | Output path for rootfs.ext4 | (required) |
| `--size` | Rootfs size | `2G` |
| `--init-script` | Custom init script path | - |
| `--verbose` | Enable verbose logging | `false` |

**Requirements:**
- Docker
- `mkfs.ext4` (e2fsprogs)
- `sudo` (for mount/umount)

---

## cmd/vmctl

CLI for managing GCP VMs directly (for testing/debugging).

```bash
# Build
go build -o vmctl ./cmd/vmctl/

# Create VM
./vmctl create --workshop ws-123 --seats 5

# List VMs
./vmctl list

# Delete VM
./vmctl delete --workshop ws-123
```

**Environment Variables:**
| Variable | Description |
|----------|-------------|
| `GCP_PROJECT` | GCP project ID |
| `GCP_ZONE` | GCP zone (default: us-central1-a) |
| `GCP_REGISTRY` | Container registry URL |

---

## Architecture

### Single-Box Mode (Development)
```
[ Browser ] --> [ server:8080 ] --> [ Docker / Local Firecracker ]
```

### Distributed Mode (Production)
```
[ Browser ] --> [ Cloud Run: server ] --> [ Worker VMs: agent:9090 ]
                                                    |
                                                    v
                                          [ Firecracker MicroVMs ]
```

---

## Quick Start

### Docker Mode (Default)
```bash
cd backend
go run ./cmd/server/
```

### Firecracker Mode (Local)
```bash
# 1. Setup Firecracker (one-time)
sudo ../scripts/setup-firecracker.sh

# 2. Build rootfs
sudo ./rootfs-builder --image clarateach-workspace --output /var/lib/clarateach/images/rootfs.ext4

# 3. Run server
go run ./cmd/server/
```

### Firecracker Mode (Distributed)
```bash
# On Worker VMs:
./agent

# On Control Plane:
WORKER_AGENTS='[{"address":"worker1:9090","token":"secret"}]' ./server
```

---

## GCP Worker VM Setup

Firecracker requires KVM. On GCP, you must create VMs with **nested virtualization** enabled.

### Create a New Worker VM

```bash
# Using spot instance for cost savings (~60-90% cheaper)
gcloud compute instances create clara-worker \
  --project=clarateach \
  --zone=us-central1-b \
  --machine-type=n2-standard-8 \
  --enable-nested-virtualization \
  --image-family=debian-12 \
  --image-project=debian-cloud \
  --boot-disk-size=50GB \
  --provisioning-model=SPOT \
  --instance-termination-action=STOP
```

> **Note:** Spot instances can be preempted by GCP. Use `--instance-termination-action=STOP` to stop (not delete) the VM when preempted, preserving the disk.

### Migrate Existing VM to Nested Virtualization

You cannot enable nested virtualization on an existing VM. You must recreate it:

```bash
# 1. Create snapshot of current disk
gcloud compute disks snapshot clara2 \
  --zone=us-central1-b \
  --snapshot-names=clara2-snapshot \
  --project=clarateach

# 2. Delete the VM
gcloud compute instances delete clara2 \
  --zone=us-central1-b \
  --project=clarateach \
  --quiet

# 3. Create new VM with nested virtualization from snapshot (spot instance)
gcloud compute instances create clara2 \
  --project=clarateach \
  --zone=us-central1-b \
  --machine-type=n2-standard-8 \
  --enable-nested-virtualization \
  --create-disk=boot=yes,source-snapshot=clara2-snapshot,size=50GB,auto-delete=yes \
  --provisioning-model=SPOT \
  --instance-termination-action=STOP
```

### Verify KVM is Available

```bash
# SSH into the VM
gcloud compute ssh clara2 --zone=us-central1-b --project=clarateach

# Check for /dev/kvm
ls -la /dev/kvm
```

### Requirements

- **Machine type:** N1, N2, or C2 (not E2 or N2D)
- **Disk:** 50GB recommended (rootfs is 2GB per VM)

---

## Scripts

| Script | Purpose | Status |
|--------|---------|--------|
| `scripts/setup-firecracker.sh` | Download Firecracker + kernel | Active |
| `scripts/test-firecracker.sh` | Test Firecracker VM creation | Active |
| `scripts/build-rootfs.sh` | Build rootfs (shell version) | Legacy (use `rootfs-builder`) |
| `scripts/run_backend_docker.sh` | Run backend in Docker | Active |
| `scripts/run_backend_local.sh` | Run backend locally | Active |

### Testing Firecracker

Run the test script to verify Firecracker VM creation works:

```bash
sudo ./scripts/test-firecracker.sh
```

This will:
1. Build the agent
2. Start the agent on port 9090
3. Create 3 test VMs
4. Verify they're running (processes, ping, API)
5. Destroy the VMs and clean up

**Options:**
```bash
# Create more VMs
sudo NUM_VMS=10 ./scripts/test-firecracker.sh

# Use different port
sudo PORT=9091 ./scripts/test-firecracker.sh
```

---

## API Overview

### Workshops
- `GET /api/workshops` - List workshops
- `POST /api/workshops` - Create workshop
- `DELETE /api/workshops/{id}` - Delete workshop

### Sessions
- `POST /api/join` - Join workshop (returns JWT + endpoint)
- `GET /api/session/{code}` - Get session by code

### Admin
- `GET /api/admin/overview` - Dashboard overview
- `GET /api/admin/vms` - List all VMs
