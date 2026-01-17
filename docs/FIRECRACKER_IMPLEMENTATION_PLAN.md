# Firecracker MicroVM Implementation Plan

## Executive Summary

This document outlines the plan to implement **Firecracker MicroVMs** as a secure, isolated runtime alternative to Docker containers for ClaraTeach. This transition is essential for running untrusted learner code securely.

The implementation builds upon the newly established **Unified Go Backend**. The backend (running on a Linux VM with KVM) will directly orchestrate Firecracker processes alongside Docker containers.

---

## 1. Architecture: The Hybrid Host

The goal is to support both Docker (for development/Mac) and Firecracker (for production/Linux) within the same `orchestrator` package.

### Architecture Diagram (Production)

```mermaid
graph TD
    subgraph "Linux Host (GCP N2 / Bare Metal)"
        GoBackend[Go Backend (Port 8080)]
        Bridge[Linux Bridge: clarateach0]
        
        subgraph "Seat 1 (Firecracker)"
            Tap1[TAP Device: tap1]
            MicroVM1[MicroVM (Guest IP: 192.168.100.2)]
        end
        
        subgraph "Seat 2 (Docker - Optional)"
            Veth2[Veth Pair]
            Container2[Docker Container]
        end
    end

    GoBackend -- Spawns --> MicroVM1
    GoBackend -- Configures --> Bridge
    GoBackend -- Configures --> Tap1
    Bridge -- Connects --> Tap1
```

### The `Provider` Interface

We will leverage the existing interface in `backend/internal/orchestrator/provider.go`:

```go
type Provider interface {
    Create(ctx context.Context, cfg InstanceConfig) (*Instance, error)
    Destroy(ctx context.Context, workshopID string, seatID int) error
    List(ctx context.Context, workshopID string) ([]*Instance, error)
    GetIP(ctx context.Context, workshopID string, seatID int) (string, error)
}
```

We will implement a new `FirecrackerProvider` struct that satisfies this interface.

---

## 2. Implementation Steps

### Phase 1: Infrastructure Preparation (Rootfs & Kernel)

Unlike Docker, Firecracker requires a raw filesystem image (`rootfs.ext4`) and a kernel (`vmlinux`).

1.  **Kernel:** Use the standard AWS-recommended kernel (v5.10 or v6.1).
2.  **Rootfs Pipeline:** Create a script (`scripts/build-rootfs.sh`) to convert our existing Docker image (`clarateach-workspace`) into a bootable rootfs.
    *   **Step A:** Create empty ext4 file (`dd`, `mkfs.ext4`).
    *   **Step B:** `docker export` the workspace container filesystem.
    *   **Step C:** Extract into the mounted ext4 file.
    *   **Step D:** Inject an `init` system (OpenRC) because the Docker image lacks one.
    *   **Step E:** Inject the `workspace-server` (Node.js) service definition.

### Phase 2: Networking Plumbing (The Hard Part)

Firecracker requires manual network setup on the host. The `FirecrackerProvider` must:

1.  **Bridge Creation:** On startup, ensure `clarateach0` bridge exists (`netlink` library).
2.  **TAP Creation:** For each seat, create a TAP device (`tapX`).
3.  **Linkage:** Attach `tapX` to `clarateach0`.
4.  **IPAM:** Manually manage IPs (e.g., `192.168.100.X` for Seat X).
5.  **NAT/Masquerading:** Ensure `iptables` allows traffic from the bridge to the internet (so learners can `npm install`).

### Phase 3: The `FirecrackerProvider`

Implement `backend/internal/orchestrator/firecracker.go`:

```go
func (f *FirecrackerProvider) Create(ctx context.Context, cfg InstanceConfig) (*Instance, error) {
    // 1. Setup Network (Tap, Bridge)
    // 2. Prepare Config (Kernel, Rootfs, BootArgs="ip=192.168...")
    // 3. firecracker.NewMachine(...)
    // 4. machine.Start()
    // 5. Return Instance{IP: "192.168..."}
}
```

### Phase 4: Hybrid Switching

Update `backend/cmd/server/main.go` to choose the provider based on an environment variable or flag:

```go
var orch orchestrator.Provider
if os.Getenv("RUNTIME") == "firecracker" {
    orch = orchestrator.NewFirecrackerProvider(...)
} else {
    orch = orchestrator.NewDockerProvider(...)
}
```

---

## 3. Development Workflow

Since Firecracker requires KVM (Linux), development on macOS requires a two-step approach or a VM.

**Recommended Workflow:**
1.  **Provision:** A GCP `n2-standard-8` instance (Nested Virtualization enabled).
2.  **Remote Dev:** Use VS Code Remote SSH or simply `git pull` on the VM.
3.  **Run:** Execute the backend directly on the Linux VM.

---

## 4. Dependencies

*   `github.com/firecracker-microvm/firecracker-go-sdk`
*   `github.com/vishvananda/netlink` (For networking plumbing)
*   `firecracker` binary (installed on host)
*   `jailer` binary (installed on host, optional for dev)

---

## 5. Security Model

*   **Isolation:** MicroVMs share nothing but the hardware.
*   **Networking:** Host firewall rules prevent MicroVMs from talking to the Backend API (except specific endpoints if needed) or other MicroVMs.
*   **Jailer:** In production, use `jailer` to chroot the Firecracker process and drop privileges.

---

## 6. VM Image Setup & Snapshot Creation

The `GCPFirecrackerProvider` creates VMs from a pre-baked GCP snapshot (`clara2-snapshot`) rather than using startup scripts. This approach provides faster boot times since all components are pre-installed.

### 6.1 Prerequisites

- GCP project with Compute Engine API enabled
- A GCP VM with nested virtualization enabled (n2 or c2 machine type)

### 6.2 Create Base VM

```bash
# Create VM with nested virtualization (required for Firecracker)
gcloud compute instances create clara2 \
  --project=clarateach \
  --zone=us-central1-b \
  --machine-type=n2-standard-8 \
  --image-family=ubuntu-2404-lts-amd64 \
  --image-project=ubuntu-os-cloud \
  --boot-disk-size=50GB \
  --boot-disk-type=pd-balanced \
  --enable-nested-virtualization \
  --tags=clarateach,clarateach-agent
```

### 6.3 Install Components

SSH into the VM and run the setup scripts in order:

```bash
# SSH into the VM
gcloud compute ssh clara2 --zone=us-central1-b

# Clone the repository
git clone https://github.com/YOUR_ORG/clarateach.git
cd clarateach

# 1. Install system dependencies (Go, Docker, KVM, Firecracker)
./scripts/setup_linux_vm.sh

# Log out and back in to apply group changes (docker, kvm)
exit
gcloud compute ssh clara2 --zone=us-central1-b
cd clarateach

# 2. Install Firecracker binaries and kernel
sudo ./scripts/setup-firecracker.sh

# 3. Build the rootfs from Docker workspace image
sudo ./scripts/build-rootfs.sh

# 4. Build and install the agent
cd backend
go build -o agent ./cmd/agent
sudo cp agent /usr/local/bin/agent
sudo chmod +x /usr/local/bin/agent

# 5. Install the systemd service
sudo cp ../scripts/clarateach-agent.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable clarateach-agent
```

### 6.4 Verify Installation

Before creating the snapshot, verify all components are in place:

```bash
# Check essential files
ls -la /usr/local/bin/agent
ls -la /usr/local/bin/firecracker
ls -la /var/lib/clarateach/images/vmlinux
ls -la /var/lib/clarateach/images/rootfs.ext4

# Test agent starts
sudo systemctl start clarateach-agent
sudo systemctl status clarateach-agent
curl http://localhost:9090/health

# Stop agent before snapshot
sudo systemctl stop clarateach-agent
```

### 6.5 Create Snapshot

```bash
# Run cleanup script on the VM
sudo ./scripts/prepare-snapshot.sh

# Shutdown the VM
sudo shutdown -h now

# From your local machine (or Cloud Shell), create the snapshot
gcloud compute disks snapshot clara2 \
  --project=clarateach \
  --zone=us-central1-b \
  --snapshot-names=clara2-snapshot-$(date +%Y%m%d)

# Optionally, update the "latest" snapshot alias
gcloud compute snapshots delete clara2-snapshot --project=clarateach --quiet || true
gcloud compute disks snapshot clara2 \
  --project=clarateach \
  --zone=us-central1-b \
  --snapshot-names=clara2-snapshot
```

### 6.6 Essential Files on Snapshot

The snapshot must contain:

| File | Purpose |
|------|---------|
| `/usr/local/bin/agent` | ClaraTeach agent binary |
| `/usr/local/bin/firecracker` | Firecracker VMM binary |
| `/usr/local/bin/jailer` | Firecracker jailer (optional) |
| `/var/lib/clarateach/images/vmlinux` | Linux kernel for MicroVMs |
| `/var/lib/clarateach/images/rootfs.ext4` | Root filesystem for MicroVMs |
| `/etc/systemd/system/clarateach-agent.service` | Systemd service definition |

### 6.7 How the Provisioner Uses the Snapshot

The `GCPFirecrackerProvider` (`backend/internal/provisioner/gcp_firecracker.go`) creates VMs as follows:

1. **Create VM from snapshot**: The boot disk is initialized from `clara2-snapshot`
   ```go
   InitializeParams: &computepb.AttachedDiskInitializeParams{
       SourceSnapshot: proto.String(fmt.Sprintf("projects/%s/global/snapshots/%s", p.project, p.snapshotName)),
   }
   ```

2. **Pass metadata**: Workshop ID, seat count, and agent token are passed via GCP metadata
   ```go
   metadata := []*computepb.Items{
       {Key: proto.String("workshop_id"), Value: proto.String(cfg.WorkshopID)},
       {Key: proto.String("seats"), Value: proto.String(strconv.Itoa(cfg.Seats))},
       {Key: proto.String("agent-token"), Value: proto.String(p.agentToken)},
   }
   ```

3. **Agent starts automatically**: The systemd service starts the agent on boot

4. **Provisioner polls for health**: Waits for agent's `/health` endpoint to respond

5. **Create MicroVMs**: Calls agent API (`POST /vms`) to create a MicroVM for each seat

### 6.8 Updating the Snapshot

When you need to update components (agent, rootfs, kernel):

```bash
# Start the clara2 VM
gcloud compute instances start clara2 --zone=us-central1-b

# SSH in and make changes
gcloud compute ssh clara2 --zone=us-central1-b

# ... make updates ...

# Run cleanup and create new snapshot (see 6.5)
```

### 6.9 Environment Variables

The provisioner is configured via environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `GCP_PROJECT` | GCP project ID | (required) |
| `GCP_ZONE` | GCE zone | `us-central1-a` |
| `FC_SNAPSHOT_NAME` | Snapshot name | `clara2-snapshot` |
| `FC_MACHINE_TYPE` | Machine type (must support nested virt) | `n2-standard-8` |
| `AGENT_TOKEN` | Token for agent authentication | (optional) |