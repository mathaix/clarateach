# Orchestrator Refactor Plan: "The Path to MicroVMs"

## 1. Executive Summary

**Objective:** Replace the brittle Node.js/Bash orchestration scripts with a robust, type-safe **Go Microservice**.

**Immediate Goal (Phase 1):** Implement a **Docker Provider** in Go. This allows development on macOS and immediate deployment to standard VMs, fixing the current fragility of shell script parsing.

**Long-Term Goal (Phase 2):** Implement a **Firecracker Provider** in Go. This enables secure, hardware-isolated MicroVMs for production on supported infrastructure (GCP N2/Metal), without changing the API contract used by the frontend/portal.

---

## 2. Architecture Overview

The system moves from a "Script Execution" model to a "Service-to-Service" model.

```mermaid
graph TD
    subgraph "Admin Stack"
        P[Portal API (Node.js)]
        O[Orchestrator Service (Go)]
    end

    subgraph "Infrastructure"
        D[Docker Engine]
        F[Firecracker (Future)]
    end

    P -- HTTP/REST --> O
    O -- Docker SDK --> D
    O -- CNI/KVM --> F
```

### Key Design Decisions

1.  **Language:** **Go**. Chosen for its native Docker SDK, superior networking libraries (`netlink`), and industry standard status for container/VM tooling.
2.  **Communication:** **HTTP/REST**. Chosen for simplicity of integration with the existing Node.js Portal. (gRPC is overkill for the current scale).
3.  **State Management:** **Stateless** (mostly). The Orchestrator will rely on the underlying infrastructure (Docker labels) as the source of truth, just like the current architecture.

---

## 3. The "Provider" Interface

This is the core abstraction that makes the transition to MicroVMs possible. The rest of the application will not know whether a "Container" or a "MicroVM" is running.

```go
package provider

import "context"

type InstanceConfig struct {
    WorkshopID string
    SeatID     int
    Image      string // Docker image or Rootfs path
    ApiKey     string // Optional secret to inject
}

type Instance struct {
    ID        string
    IP        string
    Status    string // "running", "stopped", "error"
    Endpoint  string // e.g., "http://192.168.1.5:3000"
}

// The Contract
type Provider interface {
    // Provision a new instance (Container or VM)
    Create(ctx context.Context, cfg InstanceConfig) (Instance, error)

    // Destroy an instance
    Destroy(ctx context.Context, workshopID string, seatID int) error

    // List all instances for a workshop
    List(ctx context.Context, workshopID string) ([]Instance, error)
}
```

---

## 4. Phase 1 Implementation: Docker Provider

This implementation replaces the current `.sh` scripts.

### Technical Specs
*   **SDK:** `github.com/docker/docker/client`
*   **Networking:** Uses standard Docker Bridge networking.
*   **Labels:** Uses Docker labels to track `WorkshopID` and `SeatID` (same as current scripts, ensuring backward compatibility).

### Logic Flow (Create Instance)
1.  **Check:** Does a container for `ws-123-seat-01` already exist?
2.  **Network:** Ensure Docker network `clarateach-ws-123` exists.
3.  **Config:** Prepare `ContainerConfig` (Image, Env Vars) and `HostConfig` (Mounts, Resources).
4.  **Run:** `cli.ContainerCreate` -> `cli.ContainerStart`.
5.  **Inspect:** Query the container to get its assigned IP address.
6.  **Return:** Return struct with IP and Status.

---

## 5. API Specification (Internal)

The Orchestrator will expose a simple internal API on `localhost:8080`.

| Method | Path | Payload | Description |
| :--- | :--- | :--- | :--- |
| `POST` | `/workshops/:id/seats` | `{ "seat": 1, "api_key": "..." }` | Provision a seat. |
| `DELETE` | `/workshops/:id/seats/:seat` | - | Destroy a seat. |
| `GET` | `/workshops/:id/seats` | - | List all seats and their IPs. |
| `DELETE` | `/workshops/:id` | - | Destroy entire workshop (all seats + network). |

---

## 6. Implementation Plan

### Step 1: Scaffolding (Day 1)
*   Create `orchestrator/` directory.
*   Initialize `go.mod`.
*   Set up basic HTTP server structure (using `net/http` and `chi` router).
*   Define the `Provider` interface.

### Step 2: Docker Provider Core (Day 1-2)
*   Implement `docker_provider.go`.
*   Add logic for `Create`, `Destroy`, `List`.
*   **Verification:** Write a Go test that spins up a real container (requires local Docker).

### Step 3: API Layer (Day 2)
*   Connect HTTP handlers to the Provider methods.
*   Add simple error handling and JSON logging.

### Step 4: Portal Integration (Day 3)
*   Modify `portal/src/services/orchestrator.ts`.
*   Replace `exec('create-container.sh')` with `axios.post('http://orchestrator:8080/...')`.
*   Update `docker-compose.yml` to include the new `orchestrator` service.

### Step 5: Cleanup
*   Delete `workspace/orchestrator/*.sh` scripts.
*   Update documentation.

---

## 7. Future: Phase 2 (Firecracker)

When moving to Phase 2, we will create `firecracker_provider.go`.

### Key Differences to Handle
1.  **Rootfs Generation:** We will need a **Converter Pipeline**.
    *   **Tool:** We will implement a custom Go utility or use a tool like `firebuild` or `rootfs-builder`.
    *   **Process:** `docker create` -> `docker export` -> `mkfs.ext4` (create empty image) -> `mount` -> `untar` (extract filesystem) -> `unmount`.
    *   This pipeline converts our Docker image (`clarateach-workspace`) into a bootable `rootfs.ext4` file.
2.  **Networking:** The Go service will need to run as `root` (or with `NET_ADMIN` capabilities) to create TAP devices.
3.  **CNI:** We will likely use a CNI plugin (or write simple netlink code) to manage IPs.

This plan ensures that the work done today (Phase 1) is a direct investment in the future architecture (Phase 2).
