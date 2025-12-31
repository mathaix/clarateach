# GCP VM Provisioning - Scope & Options Analysis

## Goal
Provision GCE VMs on-demand when a workshop is created, run multiple Docker containers (learner workspaces) on each VM, and tear down VMs when workshops end.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                     ClaraTeach Portal                           │
│                   (Cloud Run / App Engine)                      │
└─────────────────────────┬───────────────────────────────────────┘
                          │ Creates Workshop
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                   VM Provisioner                                │
│            (Go code in backend/internal/provisioner)            │
│                                                                 │
│   Options:                                                      │
│   1. Google Cloud SDK (compute.InstancesClient)                 │
│   2. Terraform/OpenTofu (via exec or go-terraform)              │
│   3. Pulumi (pulumi-gcp Go SDK)                                │
└─────────────────────────┬───────────────────────────────────────┘
                          │ Provisions
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                    GCE VM Instance                              │
│                  (e2-standard-4 or similar)                     │
│                                                                 │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐               │
│  │ Container 1 │ │ Container 2 │ │ Container N │  (10-20 seats)│
│  │   Seat 1    │ │   Seat 2    │ │   Seat N    │               │
│  │  :3001/3002 │ │  :3011/3012 │ │  :30N1/30N2 │               │
│  └─────────────┘ └─────────────┘ └─────────────┘               │
│                                                                 │
│  ┌─────────────────────────────────────────────┐               │
│  │              Caddy Reverse Proxy            │               │
│  │         (routes /vm/{seat}/* to container)  │               │
│  └─────────────────────────────────────────────┘               │
└─────────────────────────────────────────────────────────────────┘
```

---

## Option Comparison

### Option A: Google Cloud SDK (Direct API Calls)

**What it is:** Use Google's official Go SDK (`cloud.google.com/go/compute`) to directly call GCE APIs.

**Pros:**
- No external dependencies (just Go code)
- Fine-grained control over every API call
- Lowest latency for provisioning
- Easier to debug (no IaC state to manage)
- Native Go error handling
- Can integrate directly into existing Go backend

**Cons:**
- More code to write for each resource (VM, firewall, disk, etc.)
- No built-in state management (need to track in database)
- Harder to reproduce infrastructure manually
- No drift detection

**Code Example:**
```go
import (
    compute "cloud.google.com/go/compute/apiv1"
    computepb "cloud.google.com/go/compute/apiv1/computepb"
)

func (p *GCPProvider) CreateVM(ctx context.Context, workshopID string) error {
    client, _ := compute.NewInstancesRESTClient(ctx)
    defer client.Close()

    op, err := client.Insert(ctx, &computepb.InsertInstanceRequest{
        Project: p.project,
        Zone:    p.zone,
        InstanceResource: &computepb.Instance{
            Name:        proto.String(fmt.Sprintf("clarateach-%s", workshopID)),
            MachineType: proto.String("zones/us-central1-a/machineTypes/e2-standard-4"),
            // ... disk, network, startup script
        },
    })
    return op.Wait(ctx)
}
```

**Complexity:** Medium
**Best for:** Dynamic, API-driven provisioning with custom logic

---

### Option B: Terraform / OpenTofu

**What it is:** Define infrastructure as `.tf` files, execute via CLI or go-terraform library.

**Terraform:** HashiCorp's IaC tool (BSL license since 1.6)
**OpenTofu:** Open-source fork (MPL 2.0 license)

**Pros:**
- Declarative infrastructure definition
- Built-in state management
- Drift detection and reconciliation
- Large ecosystem of providers
- Easy to version control and review
- Can be used standalone for debugging

**Cons:**
- External binary dependency (terraform/tofu CLI)
- State file management complexity (need remote backend)
- Slower provisioning (plan → apply cycle)
- Harder to integrate dynamic values from Go
- Adds operational complexity

**Integration Approaches:**
1. **Exec approach:** Shell out to `terraform apply -var workshop_id=xxx`
2. **go-terraform:** Use `github.com/hashicorp/terraform-exec` library
3. **CDK for Terraform:** Generate Terraform from Go code (overkill)

**Code Example (go-terraform):**
```go
import "github.com/hashicorp/terraform-exec/tfexec"

func (p *TerraformProvider) CreateVM(ctx context.Context, workshopID string) error {
    tf, _ := tfexec.NewTerraform("/path/to/tf/workshop", "/usr/local/bin/terraform")

    err := tf.Init(ctx, tfexec.Upgrade(true))
    if err != nil { return err }

    return tf.Apply(ctx,
        tfexec.Var(fmt.Sprintf("workshop_id=%s", workshopID)),
        tfexec.Var(fmt.Sprintf("seats=%d", seats)),
    )
}
```

**Complexity:** High
**Best for:** Complex, multi-resource infrastructure with team collaboration

---

### Option C: Pulumi (Go SDK)

**What it is:** Infrastructure as actual Go code (not config files).

**Pros:**
- Native Go code (real functions, loops, conditionals)
- Type-safe resource definitions
- Built-in state management (Pulumi Cloud or self-hosted)
- Can share logic between app and infra code
- Better IDE support than HCL
- Supports preview before deploy

**Cons:**
- Requires Pulumi CLI and account (or self-hosted backend)
- Learning curve for Pulumi concepts (stacks, outputs)
- Heavier runtime dependency
- State management adds complexity
- Commercial product (free tier available)

**Code Example:**
```go
import (
    "github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/compute"
    "github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
    pulumi.Run(func(ctx *pulumi.Context) error {
        instance, err := compute.NewInstance(ctx, "clarateach-vm", &compute.InstanceArgs{
            MachineType: pulumi.String("e2-standard-4"),
            Zone:        pulumi.String("us-central1-a"),
            BootDisk: &compute.InstanceBootDiskArgs{
                InitializeParams: &compute.InstanceBootDiskInitializeParamsArgs{
                    Image: pulumi.String("cos-cloud/cos-stable"),
                },
            },
            // ...
        })
        return err
    })
}
```

**Complexity:** Medium-High
**Best for:** Teams already using Pulumi, complex conditional logic

---

## Recommendation

### For ClaraTeach: **Option A (Google Cloud SDK)**

**Rationale:**

1. **Simplicity**: We only need to provision a single VM per workshop. No complex multi-resource dependencies.

2. **Speed**: Direct API calls are faster than IaC plan/apply cycles. Workshop creation should be quick.

3. **Dynamic**: Workshop creation is driven by API calls, not declarative config. SDK fits this model better.

4. **No State Management**: Track VM IDs in our SQLite/Postgres database. No need for separate Terraform state.

5. **Go Native**: Fits naturally into existing Go backend. No external CLI dependencies.

6. **Debugging**: Easier to debug API calls than IaC state issues.

**When to reconsider:**
- If we need complex networking (VPCs, subnets, NAT) → Consider Terraform
- If infrastructure changes need review/approval → Consider Terraform/Pulumi
- If multiple team members manage infra → Consider Terraform

---

## Implementation Plan

### Phase 1: GCP Provider Interface

```go
// backend/internal/provisioner/gcp.go

type GCPProvider struct {
    project    string
    zone       string
    client     *compute.InstancesClient
    imageURL   string  // Container-Optimized OS image
}

type VMConfig struct {
    WorkshopID   string
    MachineType  string  // e2-standard-4
    DiskSizeGB   int     // 50
    Seats        int     // determines memory/CPU
    StartupScript string
}

type VMInstance struct {
    ID         string  // GCE instance ID
    Name       string  // clarateach-ws-xxxxx
    ExternalIP string
    InternalIP string
    Status     string
    Zone       string
}

type Provisioner interface {
    CreateVM(ctx context.Context, cfg VMConfig) (*VMInstance, error)
    DeleteVM(ctx context.Context, workshopID string) error
    GetVM(ctx context.Context, workshopID string) (*VMInstance, error)
    WaitForReady(ctx context.Context, workshopID string, timeout time.Duration) error
}
```

### Phase 2: Startup Script

The VM runs Container-Optimized OS (COS) and uses cloud-init or a startup script to:
1. Pull the workspace Docker image
2. Start Caddy reverse proxy
3. Start N containers (one per seat)
4. Report readiness back to portal

```bash
#!/bin/bash
# Startup script for workshop VM

WORKSHOP_ID=$(curl -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/attributes/workshop_id)
SEATS=$(curl -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/attributes/seats)
IMAGE=$(curl -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/attributes/image)

# Pull workspace image
docker pull gcr.io/PROJECT/$IMAGE

# Start containers for each seat
for i in $(seq 1 $SEATS); do
    docker run -d \
        --name seat-$i \
        --restart unless-stopped \
        -p $((3000 + i*10 + 1)):3001 \
        -p $((3000 + i*10 + 2)):3002 \
        -e SEAT=$i \
        gcr.io/PROJECT/$IMAGE
done

# Start Caddy
docker run -d \
    --name caddy \
    --network host \
    -v /etc/caddy/Caddyfile:/etc/caddy/Caddyfile \
    caddy:latest
```

### Phase 3: DNS & SSL

Options:
1. **Wildcard DNS**: `*.clarateach.io` → Cloud Load Balancer → VMs
2. **Per-workshop DNS**: Create A record for `ws-xxxx.clarateach.io` pointing to VM IP
3. **Cloud DNS API**: Programmatically create/delete DNS records

### Phase 4: Health Checks & Cleanup

- VM health check endpoint (`/health`)
- Automatic cleanup of VMs after workshop ends (TTL-based)
- Graceful shutdown with data persistence option

---

## Cost Estimate

| Resource | Spec | Monthly Cost |
|----------|------|--------------|
| e2-standard-4 | 4 vCPU, 16GB RAM | ~$97/month |
| e2-standard-8 | 8 vCPU, 32GB RAM | ~$194/month |
| Boot disk | 50GB SSD | ~$8.50/month |
| Egress | Variable | ~$0.12/GB |

**Per-workshop cost** (assuming 4-hour workshop on e2-standard-4):
- Compute: $0.134/hr × 4hr = $0.54
- Disk: ~$0.01
- **Total: ~$0.55 per workshop**

---

## Files to Create

```
backend/
├── internal/
│   ├── provisioner/
│   │   ├── gcp.go           # GCP SDK implementation
│   │   ├── gcp_test.go      # Integration tests (needs GCP project)
│   │   ├── provisioner.go   # Interface definition
│   │   └── mock.go          # Mock for local dev
│   └── orchestrator/
│       ├── docker.go        # Existing (local containers)
│       └── vm.go            # NEW: Orchestrates containers ON a VM
```

---

## Timeline Estimate

| Task | Effort |
|------|--------|
| GCP Provider implementation | 1-2 days |
| Startup script & cloud-init | 0.5 days |
| DNS integration | 0.5 days |
| Health checks & monitoring | 0.5 days |
| Testing & debugging | 1-2 days |
| **Total** | **4-6 days** |

---

## Decisions Made

### 1. Image Registry: **Artifact Registry** ✓

GCR (Container Registry) is deprecated. Artifact Registry is the replacement with:
- Native IAM per-repository (cleaner than bucket-based)
- Better vulnerability scanning
- Multi-format support (Docker, npm, Maven, etc.)
- Any GCP region (not just us/eu/asia)

**Registry URL format:**
```
REGION-docker.pkg.dev/PROJECT_ID/clarateach/workspace:latest
```

**Setup:**
```bash
# Create repository
gcloud artifacts repositories create clarateach \
  --repository-format=docker \
  --location=us-central1 \
  --description="ClaraTeach container images"

# Configure Docker auth
gcloud auth configure-docker us-central1-docker.pkg.dev

# Push image
docker tag clarateach-workspace \
  us-central1-docker.pkg.dev/PROJECT_ID/clarateach/workspace:latest
docker push us-central1-docker.pkg.dev/PROJECT_ID/clarateach/workspace:latest
```

### 2. Authentication: **Workload Identity (Keyless)** ✓

No JSON key files to manage or rotate.

| Environment | Method |
|-------------|--------|
| Local Dev | `gcloud auth application-default login` |
| Production (Cloud Run) | Workload Identity (attach service account) |

**Setup:**
```bash
# Create service account
gcloud iam service-accounts create clarateach-provisioner \
  --display-name="ClaraTeach VM Provisioner"

# Grant compute permissions
gcloud projects add-iam-policy-binding PROJECT_ID \
  --member="serviceAccount:clarateach-provisioner@PROJECT.iam.gserviceaccount.com" \
  --role="roles/compute.instanceAdmin.v1"

# Attach to Cloud Run (production)
gcloud run services update clarateach-backend \
  --service-account=clarateach-provisioner@PROJECT.iam.gserviceaccount.com

# Local dev - just run once
gcloud auth application-default login
```

The Go SDK auto-detects credentials from the environment.

### 3. Spot VMs: **Yes for Testing** ✓

Use Spot VMs (preemptible) for development/testing to reduce costs by ~60-90%.

| Environment | VM Type | Cost |
|-------------|---------|------|
| Production | Standard (on-demand) | ~$0.134/hr (e2-standard-4) |
| Testing/Dev | Spot (preemptible) | ~$0.04/hr (70% cheaper) |

**Trade-offs:**
- Spot VMs can be terminated with 30s notice if GCP needs capacity
- Max 24-hour runtime (auto-terminated)
- Fine for workshops (typically 2-4 hours)
- Not recommended for production workshops (learners would lose work)

**Implementation:**
```go
// In VMConfig
Spot bool  // true = use spot/preemptible VM

// In provisioner
Scheduling: &computepb.Scheduling{
    Preemptible: proto.Bool(cfg.Spot),
    // Or for newer Spot VMs:
    ProvisioningModel: proto.String("SPOT"),
}
```

### 4. Multi-region: **Single Region (us-central1)** ✓

Start with `us-central1-a`. Can expand to other regions later if needed for latency.

### 5. VM Sizing: **Fixed (e2-standard-4)** ✓

Start with fixed sizing. Can add dynamic sizing later based on seat count.

| Seats | Machine Type | vCPU | RAM |
|-------|--------------|------|-----|
| 1-10 | e2-standard-4 | 4 | 16GB |
| 11-20 | e2-standard-8 | 8 | 32GB |

For now: Always use `e2-standard-4`. Revisit when class sizes vary.

---

## All Decisions Summary

| Decision | Choice |
|----------|--------|
| IaC Approach | Google Cloud SDK (direct API) |
| Image Registry | Artifact Registry |
| Authentication | Workload Identity (keyless) |
| Spot VMs | Yes for testing, standard for production |
| Region | us-central1-a (single) |
| VM Size | Fixed e2-standard-4 |

---

## Implementation Details (Completed)

### Container Creation on VM

Containers are created at **VM boot time** via the startup script embedded in the GCP provisioner (`backend/internal/provisioner/gcp.go:308-495`).

**Startup Script Flow:**
1. VM boots with Container-Optimized OS (COS)
2. Waits for Docker daemon to be ready
3. Authenticates with Artifact Registry
4. Pulls workspace image and support images (Neko, Caddy)
5. Creates containers for ALL seats at once (not on-demand):
   ```bash
   for i in $(seq 1 $SEATS); do
       docker run -d --name "seat-$i" \
           -p "$((3000 + i*10 + 1)):3001" \  # Terminal
           -p "$((3000 + i*10 + 2)):3002" \  # Files API
           ...
   done
   ```
6. Starts Caddy reverse proxy on port 80
7. Performs health check

**Port Mapping:**
| Seat | Terminal Port | Files Port | Browser Port |
|------|---------------|------------|--------------|
| 1    | 3011          | 3012       | 3013         |
| 2    | 3021          | 3022       | 3023         |
| 3    | 3031          | 3032       | 3033         |
| N    | 30N1          | 30N2       | 30N3         |

### Workshop Lifecycle

```
┌──────────────────────────────────────────────────────────────────┐
│ 1. CREATE WORKSHOP                                               │
│    POST /api/workshops                                           │
│    └─> Creates workshop record in database (status: "created")   │
└──────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌──────────────────────────────────────────────────────────────────┐
│ 2. START WORKSHOP (if GCP_PROJECT is set)                        │
│    POST /api/workshops/{id}/start                                │
│    ├─> Generates SSH keypair (Ed25519)                           │
│    ├─> Creates GCE VM via Compute API                            │
│    ├─> VM runs startup script (creates containers)               │
│    ├─> Stores VM info + SSH private key in database              │
│    └─> Updates workshop status to "running"                      │
└──────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌──────────────────────────────────────────────────────────────────┐
│ 3. STUDENT JOINS                                                 │
│    POST /api/join {code: "ABC123", name: "Student"}              │
│    ├─> Looks up VM external IP from database                     │
│    ├─> Assigns seat to student                                   │
│    └─> Returns endpoint: http://{VM_IP}                          │
│                                                                  │
│    (No container creation here - already running on VM)          │
└──────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌──────────────────────────────────────────────────────────────────┐
│ 4. STUDENT USES WORKSPACE                                        │
│    Frontend connects to:                                         │
│    ├─> ws://{VM_IP}/vm/{seat}/terminal  (WebSocket)             │
│    ├─> http://{VM_IP}/vm/{seat}/files   (REST API)              │
│    └─> http://{VM_IP}/vm/{seat}/browser (Neko WebSocket)        │
└──────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌──────────────────────────────────────────────────────────────────┐
│ 5. DELETE WORKSHOP                                               │
│    DELETE /api/workshops/{id}                                    │
│    ├─> Calls provisioner.DeleteVM()                              │
│    ├─> GCE VM is terminated and deleted                          │
│    └─> Workshop + sessions removed from database                 │
└──────────────────────────────────────────────────────────────────┘
```

### Networking Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Student Browser                               │
│                                                                      │
│   Workspace Page (React)                                             │
│   ├── Terminal Component ──────► ws://35.x.x.x/vm/1/terminal        │
│   ├── Editor Component ────────► http://35.x.x.x/vm/1/files         │
│   └── Browser Component ───────► ws://35.x.x.x/vm/1/browser         │
└─────────────────────────────────────────────────────────────────────┘
                                     │
                                     │ (Internet)
                                     ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         GCE VM (35.x.x.x)                           │
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │                    Caddy (Port 80)                          │    │
│  │                                                              │    │
│  │  /health              → respond "OK"                        │    │
│  │  /vm/1/terminal*      → localhost:3011 (seat-1)            │    │
│  │  /vm/1/files*         → localhost:3012 (seat-1)            │    │
│  │  /vm/1/browser*       → localhost:3013 (seat-1-neko)       │    │
│  │  /vm/2/terminal*      → localhost:3021 (seat-2)            │    │
│  │  ...                                                        │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                              │                                       │
│       ┌──────────────────────┼──────────────────────┐               │
│       │                      │                      │               │
│       ▼                      ▼                      ▼               │
│  ┌──────────┐          ┌──────────┐          ┌──────────┐          │
│  │ seat-1   │          │ seat-2   │          │ seat-N   │          │
│  │ :3011/12 │          │ :3021/22 │          │ :30N1/N2 │          │
│  │ (ws srv) │          │ (ws srv) │          │ (ws srv) │          │
│  └──────────┘          └──────────┘          └──────────┘          │
│       │                      │                      │               │
│       ▼                      ▼                      ▼               │
│  ┌──────────┐          ┌──────────┐          ┌──────────┐          │
│  │seat-1    │          │seat-2    │          │seat-N    │          │
│  │-neko     │          │-neko     │          │-neko     │          │
│  │:3013     │          │:3023     │          │:30N3     │          │
│  │(browser) │          │(browser) │          │(browser) │          │
│  └──────────┘          └──────────┘          └──────────┘          │
│                                                                      │
│  Docker Network: clarateach-net                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### Firewall Requirements

The VM is tagged with `http-server` and `https-server` which should use GCP's default firewall rules. If these don't exist, create them:

```bash
# Allow HTTP (port 80) from anywhere
gcloud compute firewall-rules create allow-http \
  --network=default \
  --allow=tcp:80 \
  --target-tags=http-server \
  --source-ranges=0.0.0.0/0

# Allow HTTPS (port 443) from anywhere
gcloud compute firewall-rules create allow-https \
  --network=default \
  --allow=tcp:443 \
  --target-tags=https-server \
  --source-ranges=0.0.0.0/0
```

### SSH Debugging Access

Each workshop VM has SSH access for debugging:

1. **Download SSH key** from Admin Portal or:
   ```bash
   curl http://localhost:8080/api/admin/vms/{workshop_id}/ssh-key > key.pem
   chmod 600 key.pem
   ```

2. **SSH into VM:**
   ```bash
   ssh -i key.pem clarateach@{VM_EXTERNAL_IP}
   ```

3. **Check containers:**
   ```bash
   docker ps -a
   docker logs seat-1
   docker logs caddy
   cat /var/log/clarateach-startup.log
   ```

### Scripts

| Script | Purpose |
|--------|---------|
| `scripts/push-image.sh` | Build and push workspace image to Artifact Registry |

### Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `GCP_PROJECT` | GCP project ID (enables GCP mode) | `clarateach` |
| `GCP_ZONE` | GCE zone (default: us-central1-a) | `us-central1-a` |
| `GCP_REGISTRY` | Artifact Registry URL | `us-central1-docker.pkg.dev/clarateach/clarateach` |
| `GCP_USE_SPOT` | Use Spot VMs (cheaper, preemptible) | `true` |

### Local vs GCP Mode

| Feature | Local (Docker) | GCP (VM) |
|---------|----------------|----------|
| Container creation | On student join | At workshop start |
| Container location | Local Docker daemon | Inside GCE VM |
| Networking | Docker network + local ports | VM external IP + Caddy |
| Cleanup | Container deleted | VM terminated |
| Use case | Development | Production |
