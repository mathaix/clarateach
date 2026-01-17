# ClaraTeach Backend

Go-based backend for ClaraTeach. Supports both Docker containers and Firecracker MicroVMs for isolated learner environments.

---

## Developer Onboarding

### Prerequisites

| Dependency | Required For | Install |
|------------|--------------|---------|
| **Go 1.21+** | All | [golang.org/dl](https://golang.org/dl/) |
| **Docker** | Docker mode, building rootfs | [docs.docker.com](https://docs.docker.com/get-docker/) |
| **gcloud CLI** | GCP deployment | [cloud.google.com/sdk](https://cloud.google.com/sdk/docs/install) |
| **KVM** | Firecracker mode | `sudo apt install qemu-kvm` (Linux only) |
| **Firecracker** | Firecracker mode | `sudo ./scripts/setup-firecracker.sh` |
| **e2fsprogs** | Building rootfs | `sudo apt install e2fsprogs` |

### Clone and Build

```bash
git clone https://github.com/yourorg/clarateach.git
cd clarateach/backend

# Build all binaries
go build ./...

# Or build individually
go build -o server ./cmd/server/
go build -o agent ./cmd/agent/
go build -o rootfs-builder ./cmd/rootfs-builder/
```

---

## Testing Flows

There are three ways to run ClaraTeach, depending on your environment and needs:

| Flow | Use Case | Requirements |
|------|----------|--------------|
| **Docker Mode** | Quick local dev, no KVM needed | Docker |
| **Firecracker Local** | Test MicroVMs on KVM-enabled machine | KVM, Firecracker, rootfs |
| **Firecracker Distributed** | Production-like, multi-worker | GCP VMs with nested virtualization |

---

### Flow 1: Docker Mode (Simplest)

Best for quick development without Firecracker. Uses Docker containers for isolation.

**Requirements:** Docker running locally

```bash
cd backend

# Start the server
go run ./cmd/server/

# Server runs on http://localhost:8080
```

Test it:
```bash
curl http://localhost:8080/api/workshops
```

---

### Flow 2: Firecracker Local (Single Machine)

Test Firecracker MicroVMs on a KVM-enabled Linux machine (or GCP VM with nested virtualization).

**Requirements:**
- Linux with `/dev/kvm` available
- Root access (for networking/KVM)

#### Step 1: Install Firecracker

```bash
sudo ./scripts/setup-firecracker.sh
```

This downloads:
- `firecracker` binary → `/usr/local/bin/firecracker`
- Linux kernel → `/var/lib/clarateach/images/vmlinux`

#### Step 2: Build the Workspace Image

```bash
# Build the Docker image first
cd ../workspace
docker build -t clarateach-workspace .
cd ../backend

# Convert to Firecracker rootfs
go build -o rootfs-builder ./cmd/rootfs-builder/
sudo ./rootfs-builder \
  --image clarateach-workspace \
  --output /var/lib/clarateach/images/rootfs.ext4
```

#### Step 3: Run the Test Script

```bash
sudo ./scripts/test-firecracker.sh
```

This will:
1. Build the agent
2. Start the agent on port 9090
3. Create 3 test VMs (192.168.100.11-13)
4. Ping each VM
5. Clean up

**Expected output:**
```
========================================
  Firecracker VM Test
========================================

[+] Checking prerequisites...
[+] Prerequisites OK
[+] Building agent...
[+] Agent built
[+] Starting agent on port 9090...
[+] Agent ready (PID: 12345)
[+] Creating 3 VMs...
[+]   VM 1 created: 192.168.100.11
[+]   VM 2 created: 192.168.100.12
[+]   VM 3 created: 192.168.100.13
[+] Testing VMs...
[+]   Firecracker processes: 3 OK
[+]   Ping 192.168.100.11: OK
[+]   Ping 192.168.100.12: OK
[+]   Ping 192.168.100.13: OK
[+]   API listing: 3 VMs OK
[+] Destroying VMs...
[+]   VM 1 destroyed
[+]   VM 2 destroyed
[+]   VM 3 destroyed

========================================
[+] All tests passed!
========================================
```

#### Step 4: Manual Testing (Optional)

If you want to manually test the agent API:

```bash
# Terminal 1: Start the agent
sudo AGENT_TOKEN=dev-token ./agent

# Terminal 2: Create a VM
curl -X POST http://localhost:9090/vms \
  -H "Authorization: Bearer dev-token" \
  -H "Content-Type: application/json" \
  -d '{"workshop_id": "test", "seat_id": 1}'

# List VMs
curl -H "Authorization: Bearer dev-token" \
  "http://localhost:9090/vms?workshop_id=test"

# Ping the VM
ping 192.168.100.11

# Destroy the VM
curl -X DELETE http://localhost:9090/vms/test/1 \
  -H "Authorization: Bearer dev-token"
```

---

### Flow 3: Firecracker Distributed (Production-like)

Run the control plane separately from worker agents. This is the production architecture.

```
┌──────────────────────────────────┐
│  Control Plane (Cloud Run)       │
│  server:8080                     │
└──────────────┬───────────────────┘
               │ HTTP (port 9090)
    ┌──────────┼──────────┐
    ▼          ▼          ▼
┌────────┐ ┌────────┐ ┌────────┐
│Worker 1│ │Worker 2│ │Worker N│
│agent   │ │agent   │ │agent   │
│        │ │        │ │        │
│ FC VMs │ │ FC VMs │ │ FC VMs │
└────────┘ └────────┘ └────────┘
```

#### Step 1: Set Up Worker VM on GCP

Firecracker requires KVM. On GCP, create a VM with nested virtualization:

```bash
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

> **Note:** Spot instances are ~60-90% cheaper. `--instance-termination-action=STOP` preserves the disk if preempted.

#### Step 2: Set Up the Worker

SSH into the worker and install dependencies:

```bash
gcloud compute ssh clara-worker --zone=us-central1-b --project=clarateach

# On the worker VM:
sudo apt update && sudo apt install -y git golang-go docker.io e2fsprogs

# Clone and build
git clone https://github.com/yourorg/clarateach.git
cd clarateach/backend

# Install Firecracker
sudo ./scripts/setup-firecracker.sh

# Build rootfs (or copy from another machine)
cd ../workspace && docker build -t clarateach-workspace .
cd ../backend
go build -o rootfs-builder ./cmd/rootfs-builder/
sudo ./rootfs-builder \
  --image clarateach-workspace \
  --output /var/lib/clarateach/images/rootfs.ext4

# Build and start the agent
go build -o agent ./cmd/agent/
sudo AGENT_TOKEN=secret123 ./agent
```

#### Step 3: Run the Control Plane

On your local machine or Cloud Run:

```bash
cd backend
WORKER_AGENTS='[{"address":"WORKER_IP:9090","token":"secret123"}]' \
  go run ./cmd/server/
```

Replace `WORKER_IP` with the worker's internal IP.

#### Step 4: Test End-to-End

```bash
# Create a workshop
curl -X POST http://localhost:8080/api/workshops \
  -H "Content-Type: application/json" \
  -d '{"name": "Test Workshop", "seats": 3, "runtime": "firecracker"}'

# The server will create VMs on the worker agent
```

---

## Troubleshooting

### "KVM not available"

```bash
# Check if KVM exists
ls -la /dev/kvm

# If missing on GCP, you need nested virtualization (see GCP Worker VM Setup)
# If missing on local Linux, install KVM:
sudo apt install qemu-kvm
sudo usermod -aG kvm $USER
# Log out and back in
```

### "Firecracker not found"

```bash
sudo ./scripts/setup-firecracker.sh
```

### "rootfs.ext4 not found"

```bash
# Build the workspace Docker image first
cd ../workspace && docker build -t clarateach-workspace .

# Then build rootfs
cd ../backend
sudo ./rootfs-builder \
  --image clarateach-workspace \
  --output /var/lib/clarateach/images/rootfs.ext4
```

### VMs not pingable

Check the bridge and NAT setup:
```bash
# Bridge should exist
ip link show clarateach0

# IP forwarding should be enabled
cat /proc/sys/net/ipv4/ip_forward  # Should be 1

# Check iptables NAT rules
sudo iptables -t nat -L POSTROUTING
```

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

## Binaries Reference

| Binary | Purpose | Runs On |
|--------|---------|---------|
| `cmd/server/` | Control Plane API | Cloud Run / Controller VM |
| `cmd/agent/` | Worker Agent (Firecracker) | Worker VMs (KVM-enabled) |
| `cmd/rootfs-builder/` | Build rootfs images | Build machine |
| `cmd/vmctl/` | GCP VM management CLI | Developer machine |

---

## cmd/server (Control Plane)

The main API server that handles workshops, sessions, and orchestration.

```bash
go build -o server ./cmd/server/
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

---

## cmd/agent (Worker Agent)

Runs on KVM-enabled VMs to manage local Firecracker MicroVMs.

```bash
go build -o agent ./cmd/agent/
sudo AGENT_TOKEN=secret ./agent
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
go build -o rootfs-builder ./cmd/rootfs-builder/

# From existing Docker image
sudo ./rootfs-builder --image clarateach-workspace --output ./rootfs.ext4

# From Dockerfile
sudo ./rootfs-builder --dockerfile ./Dockerfile --output ./rootfs.ext4 --size 4G
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

---

## Scripts

| Script | Purpose |
|--------|---------|
| `scripts/setup-firecracker.sh` | Download Firecracker binary + Linux kernel |
| `scripts/test-firecracker.sh` | End-to-end test of VM creation |
| `scripts/run_backend_docker.sh` | Run backend in Docker |
| `scripts/run_backend_local.sh` | Run backend locally |

---

## GCP Worker VM Setup

### Create a New Worker VM

```bash
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

### Recreate Existing VM with Nested Virtualization

You cannot enable nested virtualization on an existing VM. Recreate it:

```bash
# 1. Snapshot the disk
gcloud compute disks snapshot clara2 \
  --zone=us-central1-b \
  --snapshot-names=clara2-snapshot \
  --project=clarateach

# 2. Delete the VM
gcloud compute instances delete clara2 \
  --zone=us-central1-b \
  --project=clarateach \
  --quiet

# 3. Create new VM from snapshot
gcloud compute instances create clara2 \
  --project=clarateach \
  --zone=us-central1-b \
  --machine-type=n2-standard-8 \
  --enable-nested-virtualization \
  --create-disk=boot=yes,source-snapshot=clara2-snapshot,size=50GB,auto-delete=yes \
  --provisioning-model=SPOT \
  --instance-termination-action=STOP
```

### Verify KVM

```bash
gcloud compute ssh clara2 --zone=us-central1-b --project=clarateach
ls -la /dev/kvm  # Should exist
```

### Requirements

- **Machine type:** N1, N2, or C2 (not E2 or N2D)
- **Disk:** 50GB recommended (rootfs is 2GB per VM)

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
