# ClaraTeach Build Guide

> **See also:** [Operations Guide](./operations/OPERATIONS_GUIDE.md) for deployment, credentials, and PostgreSQL setup.

## Architecture Overview

```
┌─────────────────────────────────────────┐
│  Portal + API Server                    │  Runs on: macOS, Linux, or cloud
│  ├── Frontend (React)                   │
│  └── Backend (Go)                       │
│      ├── provisioner/gcp.go             │  → Creates GCP VMs (Docker runtime)
│      ├── provisioner/gcp_firecracker.go │  → Creates GCP VMs + calls Agent API
│      └── provisioner/firecracker.go     │  → Local dev only (Linux required)
└─────────────────┬───────────────────────┘
                  │ Creates VM via GCP API
                  ▼
┌─────────────────────────────────────────┐
│  GCP VM (Linux, from snapshot)          │  Created per workshop
│  ┌───────────────────────────────────┐  │
│  │  Agent (cmd/agent)                │  │  Manages Firecracker MicroVMs
│  │  └── orchestrator/firecracker.go  │  │  Uses firecracker-go-sdk, netlink
│  └───────────────────────────────────┘  │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐    │
│  │ MicroVM │ │ MicroVM │ │ MicroVM │    │  One per learner seat
│  │ Seat 1  │ │ Seat 2  │ │ Seat N  │    │
│  └─────────┘ └─────────┘ └─────────┘    │
└─────────────────────────────────────────┘
```

## Platform-Specific Build Constraints

The codebase uses Go build tags to handle Linux-only dependencies:

| File | Build Tag | Purpose |
|------|-----------|---------|
| `orchestrator/firecracker.go` | `//go:build linux` | Firecracker SDK, netlink (Linux kernel APIs) |
| `provisioner/firecracker.go` | `//go:build linux` | Local Firecracker provisioner for dev/testing |
| `provisioner/firecracker_stub.go` | `//go:build !linux` | Stub for macOS/Windows (returns errors) |

### Linux-Only Dependencies

These packages require Linux kernel features (KVM, netlink, namespaces):

- `github.com/firecracker-microvm/firecracker-go-sdk` - Firecracker VM management
- `github.com/vishvananda/netlink` - Linux network interface management
- `github.com/vishvananda/netns` - Linux network namespaces
- `github.com/containernetworking/cni` - Container Network Interface
- `github.com/containernetworking/plugins` - CNI plugins

## Building

### API Server (Portal Backend)

Runs on **macOS or Linux**:

```bash
# From backend directory
cd backend

# Build for current platform
go build ./cmd/server/

# Or run directly
go run ./cmd/server/
```

On macOS, the local Firecracker provisioner is excluded (stub returns errors).
The GCP provisioners work normally since they only use GCP APIs.

### Agent (Runs on GCP VM)

Must be built **on Linux** or cross-compiled for Linux:

```bash
# Build agent for Linux (from any platform)
GOOS=linux GOARCH=amd64 go build -o agent ./cmd/agent/

# Or build on Linux directly
go build ./cmd/agent/
```

The agent binary is typically:
1. Pre-installed in a GCP VM snapshot (recommended)
2. Or deployed via startup script when VMs are created

### Creating the Agent Snapshot

1. Create a base GCP VM with nested virtualization enabled:
   ```bash
   gcloud compute instances create clara-base \
     --zone=us-central1-b \
     --machine-type=n2-standard-8 \
     --min-cpu-platform="Intel Cascade Lake" \
     --enable-nested-virtualization \
     --image-family=ubuntu-2204-lts \
     --image-project=ubuntu-os-cloud
   ```

2. SSH in and install dependencies:
   ```bash
   # Install Firecracker
   curl -L https://github.com/firecracker-microvm/firecracker/releases/download/v1.5.0/firecracker-v1.5.0-x86_64.tgz | tar xz
   sudo mv release-v1.5.0-x86_64/firecracker-v1.5.0-x86_64 /usr/local/bin/firecracker

   # Copy agent binary and rootfs images
   # ... (see docs/VM-SETUP.md for full instructions)
   ```

3. Create snapshot:
   ```bash
   gcloud compute snapshots create clara-agent-snapshot \
     --source-disk=clara-base \
     --zone=us-central1-b
   ```

4. Configure the API server to use this snapshot:
   ```bash
   export FC_SNAPSHOT_NAME=clara-agent-snapshot
   ```

## Runtime Types

When creating a workshop, you can choose the runtime:

| Runtime | Provisioner | Description |
|---------|-------------|-------------|
| `docker` | `gcp.go` | Creates GCP VM running Docker containers |
| `firecracker` | `gcp_firecracker.go` | Creates GCP VM from snapshot, agent manages MicroVMs |

The local Firecracker provisioner (`provisioner/firecracker.go`) is only for
development/testing on Linux machines with KVM support.
