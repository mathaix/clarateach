# Distributed Architecture: Control Plane + Worker Agent

## Problem

The current implementation assumes single-box architecture where the Backend API and Firecracker VMs run on the same host. For production, we need:
- **Control Plane** (Go Backend API) -> Cloud Run / Controller VM
- **Worker Fleet** (Firecracker VMs) -> Multiple KVM-enabled worker VMs

## Solution: Worker Agent Architecture

```
┌──────────────────────────┐
│   Control Plane (API)    │  <- Cloud Run / Controller VM
│   - Scheduling decisions │
│   - User-facing API      │
│   - Stores agent tokens  │
└───────────┬──────────────┘
            │ HTTP/REST (internal VPC, port 9090)
            │ Authorization: Bearer <AGENT_TOKEN>
    ┌───────┴───────┬───────────────┐
    ▼               ▼               ▼
┌────────┐    ┌────────┐      ┌────────┐
│Worker 1│    │Worker 2│      │Worker N│
│ Agent  │    │ Agent  │      │ Agent  │
│:9090   │    │:9090   │      │:9090   │
│ FC VMs │    │ FC VMs │      │ FC VMs │
└────────┘    └────────┘      └────────┘
```

## Deployment Model

Worker VMs are provisioned via GCP with a startup script:
```bash
#!/bin/bash
set -euo pipefail

# Download binaries and images
gsutil cp gs://my-bucket/clarateach-agent /usr/local/bin/
gsutil cp gs://my-bucket/rootfs.ext4 /var/lib/clarateach/images/
gsutil cp gs://my-bucket/vmlinux /var/lib/clarateach/images/
chmod +x /usr/local/bin/clarateach-agent

# Create systemd unit file
cat > /etc/systemd/system/clarateach-agent.service << 'EOF'
[Unit]
Description=ClaraTeach Worker Agent
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/clarateach-agent
Restart=always
RestartSec=5
Environment=PORT=9090
Environment=IMAGES_DIR=/var/lib/clarateach/images
Environment=SOCKET_DIR=/tmp/clarateach

# Security hardening
NoNewPrivileges=false
ProtectSystem=false

[Install]
WantedBy=multi-user.target
EOF

# Enable and start service
systemctl daemon-reload
systemctl enable clarateach-agent
systemctl start clarateach-agent
```

---

## Security

### 1. Network Layer (VPC Firewall)

**Option A: Control Plane on GCE (uses network tags)**
```hcl
resource "google_compute_firewall" "agent-internal-gce" {
  name    = "allow-agent-internal"
  network = "default"

  allow {
    protocol = "tcp"
    ports    = ["9090"]
  }

  source_tags = ["control-plane"]
  target_tags = ["worker-agent"]
}
```

**Option B: Control Plane on Cloud Run (uses VPC Connector)**

Cloud Run doesn't have network tags. You must use a VPC Connector and allow its subnet:

```hcl
# VPC Connector for Cloud Run
resource "google_vpc_access_connector" "connector" {
  name          = "clarateach-connector"
  region        = "us-central1"
  ip_cidr_range = "10.8.0.0/28"  # /28 required for connectors
  network       = "default"
}

# Firewall: Allow VPC Connector subnet to reach agents
resource "google_compute_firewall" "agent-internal-cloudrun" {
  name    = "allow-agent-from-connector"
  network = "default"

  allow {
    protocol = "tcp"
    ports    = ["9090"]
  }

  # VPC Connector IP range
  source_ranges = ["10.8.0.0/28"]
  target_tags   = ["worker-agent"]
}
```

Then configure Cloud Run to use the connector:
```hcl
resource "google_cloud_run_service" "api" {
  # ...
  template {
    metadata {
      annotations = {
        "run.googleapis.com/vpc-access-connector" = google_vpc_access_connector.connector.id
        "run.googleapis.com/vpc-access-egress"    = "all-traffic"
      }
    }
  }
}
```

### 2. Authentication (Agent Token)
- Control Plane generates a random token when creating/registering a Worker VM
- Token is injected into VM via GCP instance metadata: `agent-token`
- Agent reads token on startup from metadata service
- Every request must include: `Authorization: Bearer <AGENT_TOKEN>`

```go
// Agent reads token from GCP metadata on startup
// IMPORTANT: Must include Metadata-Flavor header or request will 403
func getAgentToken() (string, error) {
    req, _ := http.NewRequest("GET",
        "http://metadata.google.internal/computeMetadata/v1/instance/attributes/agent-token", nil)
    req.Header.Set("Metadata-Flavor", "Google")  // Required header

    client := &http.Client{Timeout: 2 * time.Second}
    resp, err := client.Do(req)
    // ...
}

// Agent validates token on each request
func (s *Server) authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        if token != "Bearer "+s.agentToken {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

---

## Three Binaries

The system consists of three separate binaries:

| Binary | Purpose | Runs On |
|--------|---------|---------|
| `cmd/server/` | Control Plane API | Cloud Run / Controller VM |
| `cmd/agent/` | Worker Agent (Firecracker lifecycle) | Worker VMs (KVM-enabled) |
| `cmd/rootfs-builder/` | Build rootfs.ext4 from Docker image | Build machine |

---

## File Changes Summary

### New Files
| File | Purpose |
|------|---------|
| `cmd/agent/main.go` | Worker Agent binary entry point |
| `cmd/rootfs-builder/main.go` | Rootfs builder binary (replaces build-rootfs.sh) |
| `internal/agentapi/server.go` | HTTP server for Worker Agent |
| `internal/agentapi/handlers.go` | VM lifecycle HTTP handlers |
| `internal/agentapi/middleware.go` | Auth middleware (token validation) |
| `internal/rootfs/builder.go` | Rootfs building logic (Docker export -> ext4) |
| `internal/workeragent/client.go` | HTTP client for calling workers (with auth) |
| `internal/workeragent/pool.go` | Worker pool & health checks |
| `internal/workeragent/placement.go` | VM placement logic |

### Modified Files
| File | Change |
|------|--------|
| `internal/provisioner/firecracker.go` | Call Worker Agent API instead of local orchestrator |
| `cmd/server/main.go` | Initialize worker pool when `WORKER_AGENTS` env is set |

### Unchanged Files
| File | Why |
|------|-----|
| `internal/orchestrator/firecracker.go` | Reused as-is by Worker Agent |
| `internal/orchestrator/orchestrator.go` | Interface stays the same |

---

## Implementation Steps

### Step 1: Create Worker Agent API Server
Create `internal/agentapi/server.go` and `handlers.go`:

**Endpoints:**
```
GET  /health                        -> Health check (no auth)
GET  /info                          -> Worker info (capacity, VM count)
POST /vms                           -> Create VM (workshop_id, seat_id)
DELETE /vms/{workshop_id}/{seat_id} -> Destroy VM
GET  /vms/{workshop_id}/{seat_id}   -> Get VM info
GET  /vms                           -> List all VMs (optional ?workshop_id filter)
```

**Request/Response:**
```json
// POST /vms request
{
  "workshop_id": "ws-abc123",
  "seat_id": 1,
  "vcpus": 2,
  "memory_mb": 512
}

// Response
{
  "workshop_id": "ws-abc123",
  "seat_id": 1,
  "ip": "192.168.100.10",
  "status": "running"
}
```

### Step 2: Create Worker Agent Binary
Create `cmd/agent/main.go`:
- Read agent token from GCP metadata (or env var for local testing)
- Initialize `FirecrackerProvider` from existing `internal/orchestrator/`
- Start HTTP server on port 9090 (internal only)
- Apply auth middleware to all routes except `/health`
- Configuration via env vars: `PORT`, `IMAGES_DIR`, `AGENT_TOKEN` (override for local dev)

### Step 3: Create Worker Agent Client
Create `internal/workeragent/client.go`:
```go
type Client struct {
    baseURL    string
    token      string  // Agent token for authentication
    httpClient *http.Client
}

func NewClient(address, token string) *Client

// All methods add Authorization: Bearer <token> header
func (c *Client) CreateVM(ctx, VMRequest) (*VMResponse, error)
func (c *Client) DestroyVM(ctx, workshopID, seatID) error
func (c *Client) GetVM(ctx, workshopID, seatID) (*VMResponse, error)
func (c *Client) ListVMs(ctx, workshopID) ([]VMResponse, error)
func (c *Client) Health(ctx) (*HealthResponse, error)  // No auth required
```

### Step 4: Create Worker Pool Manager
Create `internal/workeragent/pool.go`:
```go
type WorkerConfig struct {
    Address string
    Token   string
}

type WorkerPool struct {
    workers map[string]*Worker  // keyed by address
}

func NewWorkerPool(configs []WorkerConfig) *WorkerPool
func (p *WorkerPool) StartHealthChecks(interval time.Duration)
func (p *WorkerPool) GetHealthyWorkers() []*Worker
```

### Step 5: Create Placement Logic
Create `internal/workeragent/placement.go`:
- Round-robin with capacity awareness
- Retry on different worker if one fails

**VM-to-Worker Mapping Persistence:**

Option A (Database - Recommended): Store `worker_id` in `workshop_vms` table
```sql
ALTER TABLE workshop_vms ADD COLUMN worker_id TEXT;
```

Option B (In-memory with recovery): On Control Plane restart, query all workers' `/vms` endpoints to rebuild the mapping.

### Step 6: Modify Firecracker Provisioner
Update `internal/provisioner/firecracker.go`:
```go
// New distributed mode
type FirecrackerProvisioner struct {
    workerPool  *workeragent.WorkerPool
    placer      *workeragent.Placer
    store       store.Store  // For persisting VM-worker mapping
}

func (f *FirecrackerProvisioner) CreateVM(ctx, cfg) {
    worker := f.placer.SelectWorker()
    resp, err := worker.Client.CreateVM(ctx, request)

    // Persist worker assignment in database
    f.store.UpdateVMWorker(ctx, cfg.WorkshopID, cfg.SeatID, worker.ID)

    return resp
}

func (f *FirecrackerProvisioner) DeleteVM(ctx, workshopID string) error {
    // Look up worker from database
    workerID, err := f.store.GetVMWorker(ctx, workshopID, seatID)
    if err != nil {
        return err
    }
    worker := f.workerPool.GetWorker(workerID)
    return worker.Client.DestroyVM(ctx, workshopID, seatID)
}

// ReconcileVMs rebuilds in-memory state from workers on startup
func (f *FirecrackerProvisioner) ReconcileVMs(ctx context.Context) error {
    for _, worker := range f.workerPool.GetHealthyWorkers() {
        vms, err := worker.Client.ListVMs(ctx, "")
        if err != nil {
            continue
        }
        for _, vm := range vms {
            f.store.UpdateVMWorker(ctx, vm.WorkshopID, vm.SeatID, worker.ID)
        }
    }
    return nil
}
```

### Step 7: Update Server Initialization
Modify `cmd/server/main.go`:
```go
if workersJSON := os.Getenv("WORKER_AGENTS"); workersJSON != "" {
    // Parse JSON array of worker configs
    var configs []workeragent.WorkerConfig
    if err := json.Unmarshal([]byte(workersJSON), &configs); err != nil {
        log.Fatalf("Invalid WORKER_AGENTS JSON: %v", err)
    }
    pool := workeragent.NewWorkerPool(configs)
    pool.StartHealthChecks(30 * time.Second)
    fcProv = provisioner.NewFirecrackerProvisioner(pool)
}
```

### Step 8: Create Rootfs Builder Binary
Create `cmd/rootfs-builder/main.go` and `internal/rootfs/builder.go`:

**Purpose:** Replace `scripts/build-rootfs.sh` with a Go binary for better maintainability.

**Workflow:**
1. Build Docker image from `workspace/` directory (if not exists)
2. Create empty ext4 file (configurable size, default 2GB)
3. Mount ext4 file
4. Export Docker container filesystem into mounted ext4
5. Inject init script for Firecracker (networking + workspace server startup)
6. Unmount and move to output directory

**Usage:**
```bash
# Build rootfs from local workspace Dockerfile
rootfs-builder --image clarateach-workspace --output /var/lib/clarateach/images/rootfs.ext4

# Build from specific Dockerfile
rootfs-builder --dockerfile ./workspace/Dockerfile --output ./rootfs.ext4 --size 4G
```

**Implementation:**
```go
package rootfs

type BuildConfig struct {
    DockerImage    string // Docker image name to export
    DockerfilePath string // Path to Dockerfile (optional, builds image if set)
    OutputPath     string // Output path for rootfs.ext4
    Size           string // Size of rootfs (e.g., "2G", "4G")
    InitScript     string // Custom init script (optional)
}

func Build(ctx context.Context, cfg BuildConfig) error {
    // 1. Build Docker image if Dockerfile provided
    // 2. Create and format ext4 file
    // 3. Mount ext4
    // 4. Docker export to mounted fs
    // 5. Inject init script
    // 6. Unmount
}
```

**Dependencies:**
- Docker CLI (for build/export)
- `mkfs.ext4` (system utility)
- `mount`/`umount` (requires root or sudo)

---

## Configuration

### Control Plane
```bash
# JSON array format (avoids delimiter conflicts with IPv6 or colons in tokens)
WORKER_AGENTS='[
  {"address": "10.0.0.10:9090", "token": "abc123token"},
  {"address": "10.0.0.11:9090", "token": "def456token"}
]'

# Or single line:
WORKER_AGENTS='[{"address":"10.0.0.10:9090","token":"abc123token"},{"address":"10.0.0.11:9090","token":"def456token"}]'
```

### Worker Agent
```bash
PORT=9090
AGENT_TOKEN=abc123token              # Set via GCP metadata in production
IMAGES_DIR=/var/lib/clarateach/images
SOCKET_DIR=/tmp/clarateach
BRIDGE_NAME=clarateach0
BRIDGE_IP=192.168.100.1/24
```

### Rootfs Builder
```bash
# CLI flags (no env vars needed - one-time build tool)
rootfs-builder \
  --image clarateach-workspace \
  --output /var/lib/clarateach/images/rootfs.ext4 \
  --size 2G
```

---

## Error Handling

| Scenario | HTTP Status | Behavior |
|----------|-------------|----------|
| Worker unreachable | N/A | Try different worker |
| Worker at capacity | 503 | Try different worker |
| Auth failed | 401 | Log error, mark worker unhealthy |
| VM already exists | 409 | Return error |
| VM not found | 404 | Return error |
| Server error | 500 | Retry once, then try different worker |

---

## Critical Files

### New Binaries
1. `backend/cmd/agent/main.go` - Worker Agent entry point
2. `backend/cmd/rootfs-builder/main.go` - Rootfs builder entry point

### New Packages
3. `backend/internal/agentapi/` - Worker Agent HTTP server
4. `backend/internal/workeragent/` - Control Plane client for workers
5. `backend/internal/rootfs/` - Rootfs building logic

### Modified Files
6. `backend/internal/provisioner/firecracker.go` - Refactor for distributed mode
7. `backend/cmd/server/main.go` - Add worker pool initialization

### Unchanged (Reused)
8. `backend/internal/orchestrator/firecracker.go` - Reused by Worker Agent as-is
