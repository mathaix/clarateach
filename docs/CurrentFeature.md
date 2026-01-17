# Current Feature: Portal → Workshop → Firecracker MicroVMs

> **Workflow**: See [CurrentFeatureWorkflow.md](CurrentFeatureWorkflow.md) for how this document is managed through the release cycle.

## Goal

Complete the end-to-end flow from portal to user workspace:

1. Admin creates workshop in portal (with Firecracker runtime)
2. Backend provisions GCP VM from snapshot
3. Agent creates MicroVMs for each seat
4. User joins workshop and accesses their workspace

## Current State Summary

| Component | Status | Notes |
|-----------|--------|-------|
| Backend API (`POST /api/workshops`) | ✅ Complete | Accepts `runtime_type`, routes to correct provisioner |
| Provisioner routing | ✅ Complete | `getProvisioner()` selects based on runtime_type |
| `GCPFirecrackerProvider` | ✅ Complete | Creates VM, waits for agent, creates MicroVMs |
| Agent API | ✅ Complete | `POST /vms` creates MicroVMs, proxy routes traffic |
| Session join flow | ✅ Complete | Routes to port 9090 for Firecracker runtime |
| **Frontend workshop creation** | ✅ Complete | Runtime selector dropdown (Docker/Firecracker) |
| **Frontend workspace connection** | ✅ Complete | Terminal, file explorer, editor all working |
| **Workshop deletion** | ✅ Complete | Properly cleans up GCP VMs |

## Architecture

```
┌──────────────────────────────────────────────────────────────────────────┐
│                         Portal (Frontend)                                 │
│                                                                          │
│  Dashboard.tsx: Create Workshop                                          │
│  - name, seats, api_key                                                  │
│  - runtime_type: docker | firecracker  ← MISSING FROM UI                 │
└────────────────────────────────┬─────────────────────────────────────────┘
                                 │
                                 ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                    POST /api/workshops                                    │
│                    { name, seats, api_key, runtime_type }                │
└────────────────────────────────┬─────────────────────────────────────────┘
                                 │
                                 ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                    Backend Server (:8080)                                │
│                                                                          │
│  getProvisioner(runtime_type):                                           │
│  ├─ "docker"      → GCPProvider (Docker containers on VM)               │
│  └─ "firecracker" → GCPFirecrackerProvider (MicroVMs on VM)             │
└────────────────────────────────┬─────────────────────────────────────────┘
                                 │
            ┌────────────────────┴────────────────────┐
            │                                         │
            ▼                                         ▼
┌─────────────────────────┐             ┌─────────────────────────┐
│ Docker Runtime          │             │ Firecracker Runtime     │
│ (GCPProvider)           │             │ (GCPFirecrackerProvider)│
│                         │             │                         │
│ 1. Create VM            │             │ 1. Create VM from       │
│ 2. Run startup script   │             │    clara2-snapshot      │
│ 3. Launch containers    │             │ 2. Wait for agent health│
│ 4. Start Caddy proxy    │             │ 3. POST /vms for seats  │
│                         │             │                         │
│ Access: :8080           │             │ Access: :9090           │
└─────────────────────────┘             └───────────┬─────────────┘
                                                    │
                                                    ▼
                                        ┌─────────────────────────┐
                                        │ GCP VM (clara2-snapshot)│
                                        │ n2-standard-8           │
                                        │ nested virtualization   │
                                        │                         │
                                        │ Agent (:9090)           │
                                        │ ├─ /health              │
                                        │ ├─ POST /vms            │
                                        │ └─ /proxy/{ws}/{seat}/  │
                                        │                         │
                                        │ MicroVMs:               │
                                        │ ├─ Seat 1: 192.168.100.11│
                                        │ ├─ Seat 2: 192.168.100.12│
                                        │ └─ Seat N: 192.168.100.N │
                                        └─────────────────────────┘
```

## Next Phase: Portal Integration

### Tasks

1. **Frontend: Add runtime selector to workshop creation**
   - File: `frontend/src/pages/Dashboard.tsx`
   - Add dropdown: Docker (default) | Firecracker
   - Pass `runtime_type` in API call

2. **Frontend: Verify workspace connection**
   - File: `frontend/src/pages/Workspace.tsx`
   - Ensure Terminal connects to: `ws://{endpoint}/proxy/{ws}/{seat}/terminal`
   - Ensure Files API connects to: `http://{endpoint}/proxy/{ws}/{seat}/files/`

3. **End-to-end test via portal**
   - Create workshop with Firecracker runtime
   - Register a user
   - Join and access workspace

---

## Developing from a Laptop

You can run the backend locally and provision real GCP VMs.

### 1. Authenticate with GCP

```bash
# One-time setup - opens browser for OAuth
gcloud auth application-default login

# Verify it worked
gcloud compute instances list --project=clarateach
```

This creates credentials at `~/.config/gcloud/application_default_credentials.json` which the Go SDK automatically uses.

### 2. Required IAM Permissions

Your Google account needs these roles on the `clarateach` project:

| Role | Purpose |
|------|---------|
| `roles/compute.instanceAdmin.v1` | Create/delete VMs |
| `roles/compute.networkUser` | Use VPC networks |
| `roles/iam.serviceAccountUser` | Attach service accounts to VMs |

Check your permissions:
```bash
gcloud projects get-iam-policy clarateach \
  --flatten="bindings[].members" \
  --filter="bindings.members:$(gcloud config get-value account)"
```

Grant if missing (requires admin):
```bash
gcloud projects add-iam-policy-binding clarateach \
  --member="user:your-email@gmail.com" \
  --role="roles/compute.instanceAdmin.v1"
```

### 3. Run the Backend

```bash
cd backend

# With Firecracker support
GCP_PROJECT=clarateach \
GCP_ZONE=us-central1-b \
GCP_REGISTRY=us-central1-docker.pkg.dev/clarateach/clarateach \
FC_SNAPSHOT_NAME=clara2-snapshot \
go run ./cmd/server/
```

### 4. Run the Frontend

```bash
cd frontend
npm install
npm run dev
```

Access at `http://localhost:5173`

### 5. Test Firecracker Flow via API

```bash
# Create workshop with Firecracker runtime
curl -X POST http://localhost:8080/api/workshops \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test Workshop",
    "seats": 2,
    "runtime_type": "firecracker"
  }'

# Check workshop status (wait for "running")
curl http://localhost:8080/api/workshops/{id}

# Get VM info
curl http://localhost:8080/api/workshops/{id}/vm
```

---

## Environment Variables Reference

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `GCP_PROJECT` | Yes | - | GCP project ID |
| `GCP_ZONE` | No | `us-central1-a` | GCE zone |
| `GCP_REGISTRY` | Yes* | - | Artifact Registry URL (for Docker runtime) |
| `FC_SNAPSHOT_NAME` | No | - | Snapshot name (enables Firecracker support) |
| `FC_MACHINE_TYPE` | No | `n2-standard-8` | Machine type for Firecracker VMs |
| `AGENT_TOKEN` | No | - | Token for agent authentication |
| `GCP_USE_SPOT` | No | `false` | Use spot/preemptible VMs |

*Required only for Docker runtime

---

## Request Flow Detail

### Workshop Creation (Firecracker)

```
POST /api/workshops { runtime_type: "firecracker", seats: 3 }
    │
    ├─► Store workshop in DB (status: "provisioning")
    ├─► Return 200 immediately
    │
    └─► [Async Goroutine]
        │
        ├─► GCPFirecrackerProvider.CreateVM()
        │   ├─► Create GCP VM from clara2-snapshot
        │   ├─► VM boots, agent starts via systemd
        │   ├─► Poll GET /health until ready (2 min timeout)
        │   ├─► POST /vms for each seat (creates MicroVMs)
        │   └─► Return VMInstance with external IP
        │
        ├─► Store VM info in DB
        └─► Update workshop status: "running"
```

### User Join

```
GET /api/session/{accessCode}
    │
    ├─► Lookup registration by code
    ├─► Get workshop (includes runtime_type)
    ├─► Get VM (includes external IP)
    ├─► Assign seat to user
    │
    └─► Return:
        {
          "status": "ready",
          "endpoint": "http://{vm_ip}:9090",  // port 9090 for Firecracker
          "seat": 1,
          "runtime_type": "firecracker"
        }
```

### Workspace Access

```
Frontend connects to:
  Terminal: ws://{endpoint}/proxy/{workshop_id}/{seat}/terminal
  Files:    http://{endpoint}/proxy/{workshop_id}/{seat}/files/
  Health:   http://{endpoint}/proxy/{workshop_id}/{seat}/health
```

---

## Files Reference

| File | Purpose |
|------|---------|
| `backend/internal/api/server.go` | API handlers, provisioner routing |
| `backend/internal/provisioner/gcp_firecracker.go` | GCP + Firecracker provisioner |
| `backend/internal/agentapi/server.go` | Agent API endpoints |
| `backend/internal/agentapi/proxy.go` | WebSocket + HTTP proxy |
| `backend/cmd/server/main.go` | Server initialization |
| `backend/cmd/agent/main.go` | Agent initialization |
| `frontend/src/pages/Dashboard.tsx` | Workshop creation UI |
| `frontend/src/pages/Workspace.tsx` | Workspace UI (terminal, files) |

---

## Previous Work (Completed)

### Agent & MicroVM Infrastructure ✅

- [x] Agent creates MicroVMs via Firecracker
- [x] Network bridge + TAP devices working
- [x] Workspace server runs in MicroVM (terminal :3001, files :3002)
- [x] Agent proxy routes traffic to MicroVMs
- [x] Agent systemd service auto-starts on boot
- [x] E2E test scripts pass (14/14 tests)

### GCP Integration ✅

- [x] `GCPFirecrackerProvider` creates VMs from snapshot
- [x] Waits for agent health
- [x] Creates MicroVMs via agent API
- [x] Passes agent token via GCP metadata
- [x] Error handling and rollback

### Documentation ✅

- [x] VM snapshot setup documented (FIRECRACKER_IMPLEMENTATION_PLAN.md)
- [x] Testing guide updated
- [x] Laptop development instructions (this document)

---

## Definition of Done (Next Phase) - COMPLETED

- [x] Frontend has runtime_type selector in workshop creation
- [x] Can create Firecracker workshop from portal UI
- [x] User can join and access workspace via portal
- [x] Terminal and file editor work correctly
- [x] Workshop deletion cleans up VM

**Completed**: 2026-01-16
