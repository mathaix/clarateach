# Current Feature: GCP + Firecracker End-to-End Integration

## Overview

Enable the full production flow where the backend orchestrates everything:
1. User creates a workshop via API
2. Backend provisions a GCP spot VM (with nested virtualization)
3. Pre-baked agent starts automatically on the VM
4. Agent spins up Firecracker MicroVMs for each seat
5. Users access their workspace through the agent proxy

## Architecture

```
┌──────────────────────────────────────────────────────────────────────────┐
│                              User Request                                 │
│                    POST /api/workshops (runtime=firecracker)             │
└────────────────────────────────┬─────────────────────────────────────────┘
                                 │
                                 ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                         Control Plane (server:8080)                       │
│                                                                          │
│  1. Create workshop record (status: provisioning)                        │
│  2. Call GCPFirecrackerProvisioner.CreateVM()                           │
│     - Create GCP spot VM from clara2 snapshot                           │
│     - Wait for agent health check                                        │
│  3. Call agent API: POST /vms for each seat                             │
│  4. Update workshop status to "running"                                  │
│  5. Return endpoints to user                                             │
└────────────────────────────────┬─────────────────────────────────────────┘
                                 │
                                 ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                     GCP Spot VM (from clara2 snapshot)                   │
│                                                                          │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │                      Agent (port 9090)                              │ │
│  │                                                                      │ │
│  │  - Pre-installed on snapshot                                        │ │
│  │  - Starts on boot via systemd                                       │ │
│  │  - Manages Firecracker MicroVMs                                     │ │
│  │  - Proxies user traffic to MicroVMs                                 │ │
│  └──────────────────────────┬─────────────────────────────────────────┘ │
│                              │                                           │
│         ┌────────────────────┼────────────────────┐                     │
│         ▼                    ▼                    ▼                     │
│  ┌────────────┐       ┌────────────┐       ┌────────────┐              │
│  │  MicroVM   │       │  MicroVM   │       │  MicroVM   │              │
│  │  Seat 1    │       │  Seat 2    │       │  Seat N    │              │
│  │            │       │            │       │            │              │
│  │ 192.168.   │       │ 192.168.   │       │ 192.168.   │              │
│  │ 100.11     │       │ 100.12     │       │ 100.{10+N} │              │
│  │            │       │            │       │            │              │
│  │ :3001 term │       │ :3001 term │       │ :3001 term │              │
│  │ :3002 files│       │ :3002 files│       │ :3002 files│              │
│  └────────────┘       └────────────┘       └────────────┘              │
│                                                                          │
│                    Bridge: clarateach0 (192.168.100.1/24)               │
└──────────────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                            User Access                                    │
│                                                                          │
│  Terminal:  wss://{worker-ip}:9090/proxy/{workshopID}/{seatID}/terminal │
│  Files:     https://{worker-ip}:9090/proxy/{workshopID}/{seatID}/files  │
└──────────────────────────────────────────────────────────────────────────┘
```

## Current State

### What Exists

| Component | File | Status |
|-----------|------|--------|
| GCP Provisioner | `internal/provisioner/gcp.go` | Creates VMs with Docker containers |
| Firecracker Provisioner | `internal/provisioner/firecracker.go` | Creates local MicroVMs via agent |
| Agent | `cmd/agent/` | Manages MicroVM lifecycle |
| Agent API | `internal/agentapi/` | CRUD for VMs |
| Orchestrator | `internal/orchestrator/firecracker.go` | Low-level Firecracker control |
| API Server | `internal/api/server.go` | Calls provisioner based on runtime_type |

### Gap Analysis

| Missing Piece | Description |
|---------------|-------------|
| GCP+Firecracker Provisioner | Combines GCP VM creation with agent-based MicroVM provisioning |
| Agent Proxy | Routes user traffic from agent to MicroVMs |
| Snapshot-based VM Creation | Create VMs from clara2 snapshot instead of startup script |
| Agent Auto-start | Systemd service to start agent on boot |
| End-to-End Test | Script to validate the full flow |

## Implementation Plan

### Phase 1: GCP Firecracker Provisioner

**File**: `internal/provisioner/gcp_firecracker.go`

```go
type GCPFirecrackerProvisioner struct {
    project     string
    zone        string
    snapshot    string  // clara2-snapshot
    agentToken  string
}

func (p *GCPFirecrackerProvisioner) CreateVM(ctx context.Context, cfg VMConfig) (*VMInstance, error) {
    // 1. Create GCP VM from snapshot
    //    - Spot instance
    //    - Nested virtualization enabled
    //    - n2-standard-8 machine type

    // 2. Wait for VM to get external IP

    // 3. Wait for agent health check (GET /health)
    //    - Retry with backoff until agent responds

    // 4. Create MicroVMs via agent API
    //    - POST /vms for each seat

    // 5. Return VMInstance with agent endpoint
}
```

**Key Configuration**:
- Snapshot: `clara2-snapshot`
- Machine type: `n2-standard-8`
- Spot: `true`
- Termination action: `STOP`
- Nested virtualization: `enabled`

### Phase 2: Agent Proxy

**File**: `internal/agentapi/proxy.go`

Add proxy endpoints to the agent:

| Endpoint | Target | Description |
|----------|--------|-------------|
| `GET /proxy/{workshopID}/{seatID}/terminal` | `ws://192.168.100.{10+seatID}:3001` | WebSocket terminal |
| `ANY /proxy/{workshopID}/{seatID}/files/*` | `http://192.168.100.{10+seatID}:3002/*` | File API |

```go
func (s *Server) handleTerminalProxy(w http.ResponseWriter, r *http.Request) {
    workshopID := chi.URLParam(r, "workshopID")
    seatID := chi.URLParam(r, "seatID")

    // Look up VM IP
    vmIP := fmt.Sprintf("192.168.100.%d", 10 + seatID)
    target := fmt.Sprintf("ws://%s:3001", vmIP)

    // Upgrade and proxy WebSocket
    proxy.ServeWS(w, r, target)
}
```

### Phase 3: Agent Auto-start on Boot

**File**: `/etc/systemd/system/clarateach-agent.service` (on clara2 snapshot)

```ini
[Unit]
Description=ClaraTeach Worker Agent
After=network.target

[Service]
Type=simple
User=root
Environment=AGENT_TOKEN=<from-metadata>
Environment=IMAGES_DIR=/var/lib/clarateach/images
Environment=SOCKET_DIR=/tmp/clarateach
ExecStart=/usr/local/bin/agent
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

The agent token should be read from GCP instance metadata at startup.

### Phase 4: End-to-End Test Script

**File**: `scripts/test-e2e-firecracker.sh`

```bash
#!/bin/bash
# End-to-end test for GCP + Firecracker flow

# 1. Start the control plane server
# 2. Call POST /api/workshops with runtime=firecracker
# 3. Wait for status=running
# 4. Get workshop details (VM IP, seat endpoints)
# 5. Test terminal WebSocket connection
# 6. Test file API
# 7. Delete workshop
# 8. Verify VM is deleted
```

## VM Lifecycle

### Create Workshop

```
POST /api/workshops
{
  "name": "Python Workshop",
  "seats": 5,
  "runtime": "firecracker"
}
```

Backend flow:
1. Insert workshop (status: `provisioning`)
2. Create GCP VM from snapshot (async)
3. Wait for agent health
4. Create 5 MicroVMs via agent
5. Update status to `running`
6. Return workshop with endpoints

### User Joins

```
POST /api/join
{
  "code": "ABC123"
}
```

Response:
```json
{
  "token": "jwt...",
  "seat": 3,
  "endpoints": {
    "terminal": "wss://34.123.45.67:9090/proxy/ws-abc/3/terminal",
    "files": "https://34.123.45.67:9090/proxy/ws-abc/3/files"
  }
}
```

### Delete Workshop

```
DELETE /api/workshops/{id}
```

Backend flow:
1. Update status to `deleting`
2. Call agent: DELETE /vms for each seat
3. Delete GCP VM
4. Update status to `deleted`

## Open Questions

1. **Agent Token Management**: How to securely pass agent token to VMs?
   - Option A: GCP instance metadata
   - Option B: Generate per-workshop token, pass via metadata
   - Option C: Mutual TLS

2. **Firewall Rules**: Need to allow port 9090 from control plane to worker VMs
   - Create `clarateach-agent` firewall rule

3. **Health Check Timeout**: How long to wait for agent to be ready?
   - Recommendation: 2 minutes with exponential backoff

4. **MicroVM Networking**: Current init script uses `sleep infinity`. Need to:
   - Restore workspace server in init script
   - Debug why Node.js was failing to start

## Testing Checklist

- [ ] GCP VM creates from snapshot successfully
- [ ] VM gets external IP
- [ ] Agent starts automatically on boot
- [ ] Agent health check responds
- [ ] MicroVMs created via agent API
- [ ] MicroVMs are pingable from agent
- [ ] Terminal WebSocket proxy works
- [ ] File API proxy works
- [ ] Workshop deletion cleans up everything
- [ ] Spot preemption handled gracefully

## Files to Create/Modify

| Action | File |
|--------|------|
| Create | `internal/provisioner/gcp_firecracker.go` |
| Create | `internal/agentapi/proxy.go` |
| Modify | `internal/agentapi/server.go` (add proxy routes) |
| Modify | `internal/api/server.go` (use new provisioner) |
| Create | `scripts/test-e2e-firecracker.sh` |
| Create | `scripts/setup-clara2-snapshot.sh` (document snapshot setup) |

## Dependencies

- GCP Compute API (`cloud.google.com/go/compute`)
- WebSocket library for proxy (`github.com/gorilla/websocket`)
- HTTP reverse proxy (`net/http/httputil`)

## Timeline

This document captures the plan. Implementation order:
1. Phase 1: GCP Firecracker Provisioner
2. Phase 2: Agent Proxy
3. Phase 3: Agent Auto-start (update snapshot)
4. Phase 4: End-to-End Test
