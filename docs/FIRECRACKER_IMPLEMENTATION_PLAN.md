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