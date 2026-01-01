# Current Feature: End-to-End Firecracker Flow Test

> **Workflow**: See [CurrentFeatureWorkflow.md](CurrentFeatureWorkflow.md) for how this document is managed through the release cycle.

## Goal

Validate the complete Firecracker workshop flow end-to-end:
1. Backend API receives workshop creation request
2. GCP spot VM is provisioned (using clara2 snapshot)
3. Agent starts and becomes healthy
4. MicroVMs are created for each seat
5. User can access workspace interfaces
6. Cleanup works correctly

## Success Criteria

- [ ] Workshop creation triggers VM provisioning
- [ ] VM created as spot instance with nested virtualization
- [ ] Agent health check passes within 2 minutes of boot
- [ ] MicroVMs created and pingable
- [ ] User can access terminal (WebSocket)
- [ ] User can access file API (HTTP)
- [ ] Workshop deletion cleans up VM and MicroVMs

## Architecture Under Test

```
┌──────────────────────────────────────────────────────────────────────────┐
│                         Test Script / Manual Test                         │
└────────────────────────────────┬─────────────────────────────────────────┘
                                 │
                                 ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                    POST /api/workshops                                    │
│                    { runtime: "firecracker", seats: 3 }                  │
└────────────────────────────────┬─────────────────────────────────────────┘
                                 │
                                 ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                         Control Plane (server:8080)                       │
│                                                                          │
│  GCPFirecrackerProvisioner:                                              │
│  1. Create GCP VM from clara2-snapshot (spot, nested virt)              │
│  2. Wait for external IP                                                 │
│  3. Poll agent health endpoint until ready                               │
│  4. Call POST /vms for each seat                                         │
│  5. Return workshop with endpoints                                       │
└────────────────────────────────┬─────────────────────────────────────────┘
                                 │
                                 ▼
┌──────────────────────────────────────────────────────────────────────────┐
│               GCP Spot VM (clara2-snapshot)                              │
│               n2-standard-8, nested virtualization                       │
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────────┐ │
│  │  Agent (:9090)                                                      │ │
│  │  - Starts via systemd on boot                                       │ │
│  │  - Creates MicroVMs via POST /vms                                   │ │
│  │  - Proxies user traffic (Phase 2)                                   │ │
│  └─────────────────────────────────────────────────────────────────────┘ │
│                                                                          │
│  ┌───────────────┐  ┌───────────────┐  ┌───────────────┐                │
│  │ MicroVM Seat1 │  │ MicroVM Seat2 │  │ MicroVM Seat3 │                │
│  │ 192.168.100.11│  │ 192.168.100.12│  │ 192.168.100.13│                │
│  └───────────────┘  └───────────────┘  └───────────────┘                │
│                                                                          │
│                    clarateach0 bridge (192.168.100.1/24)                │
└──────────────────────────────────────────────────────────────────────────┘
```

## Implementation Plan

### Phase 1: Create GCPFirecrackerProvisioner

**File**: `internal/provisioner/gcp_firecracker.go`

Combines GCP VM creation with agent-based MicroVM provisioning.

```go
type GCPFirecrackerProvisioner struct {
    computeClient *compute.InstancesClient
    project       string
    zone          string
    snapshotName  string  // "clara2-snapshot"
}

func (p *GCPFirecrackerProvisioner) CreateVM(ctx context.Context, cfg VMConfig) (*VMInstance, error) {
    // Step 1: Create GCP VM from snapshot
    // Step 2: Wait for external IP
    // Step 3: Wait for agent health (poll GET /health)
    // Step 4: Create MicroVMs (POST /vms for each seat)
    // Step 5: Return VMInstance
}
```

**Tasks**:
- [x] Create `gcp_firecracker.go` with provisioner struct
- [x] Implement `CreateVM` - create VM from snapshot
- [x] Implement `waitForExternalIP` - poll until IP assigned
- [x] Implement `waitForAgentHealth` - poll agent /health endpoint
- [x] Implement `createMicroVMs` - call agent API for each seat
- [x] Implement `DeleteVM` - destroy MicroVMs then delete GCP VM
- [ ] Add unit tests

### Phase 2: Agent Proxy for User Access

**File**: `internal/agentapi/proxy.go`

Add reverse proxy to route user traffic to MicroVMs.

| Endpoint | Proxies To | Protocol |
|----------|------------|----------|
| `/proxy/{workshopID}/{seatID}/terminal` | `192.168.100.{10+seatID}:3001` | WebSocket |
| `/proxy/{workshopID}/{seatID}/files/*` | `192.168.100.{10+seatID}:3002` | HTTP |

**Tasks**:
- [x] Create `proxy.go` with WebSocket proxy handler
- [x] Create HTTP reverse proxy handler
- [x] Register routes in `server.go`
- [ ] Test WebSocket connection through proxy
- [ ] Test file API through proxy

### Phase 3: Agent Systemd Service

Ensure agent starts automatically when VM boots.

**File**: `/etc/systemd/system/clarateach-agent.service` (on clara2)

```ini
[Unit]
Description=ClaraTeach Worker Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
Environment=IMAGES_DIR=/var/lib/clarateach/images
Environment=SOCKET_DIR=/tmp/clarateach
ExecStartPre=/bin/bash -c 'AGENT_TOKEN=$(curl -s -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/attributes/agent-token || echo "default-token"); echo "AGENT_TOKEN=$AGENT_TOKEN" > /run/clarateach-agent.env'
EnvironmentFile=/run/clarateach-agent.env
ExecStart=/usr/local/bin/agent
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

**Tasks**:
- [x] SSH into clara2 (we're already on clara2)
- [x] Copy agent binary to `/usr/local/bin/agent`
- [x] Create systemd service file
- [x] Enable and test service
- [ ] Update snapshot (requires stopping VM)

### Phase 4: End-to-End Test Script

**File**: `scripts/test-e2e-gcp-firecracker.sh`

```bash
#!/bin/bash
set -euo pipefail

# Configuration
PROJECT=${GCP_PROJECT:-clarateach}
ZONE=${GCP_ZONE:-us-central1-b}
SEATS=3

# 1. Start control plane server (background)
# 2. Create workshop via API
# 3. Wait for status=running
# 4. Verify VM exists in GCP
# 5. Verify agent is healthy
# 6. Verify MicroVMs are running
# 7. Test terminal WebSocket proxy
# 8. Test file API proxy
# 9. Delete workshop
# 10. Verify VM is deleted
# 11. Report results
```

**Tasks**:
- [ ] Create test script skeleton
- [ ] Implement workshop creation test
- [ ] Implement VM verification
- [ ] Implement agent health check
- [ ] Implement MicroVM verification
- [ ] Implement proxy tests (terminal + files)
- [ ] Implement cleanup verification
- [ ] Add timeout handling
- [ ] Add colored output for pass/fail

### Phase 5: Fix Workspace Server in MicroVM

The current init script runs `sleep infinity`. Need to restore the Node.js workspace server.

**Tasks**:
- [ ] Debug why Node.js server fails to start
- [ ] Fix init script in rootfs
- [ ] Rebuild rootfs
- [ ] Update clara2 snapshot with new rootfs
- [ ] Verify terminal and file API work inside MicroVM

## Current State

| Component | Status | Notes |
|-----------|--------|-------|
| Agent | ✅ Working | Creates MicroVMs, fixed context issue |
| Firecracker orchestrator | ✅ Working | Network bridge, TAP devices working |
| GCP provisioner (Docker) | ✅ Exists | Creates VMs with Docker containers |
| GCP+Firecracker provisioner | ✅ Created | Phase 1 complete - creates spot VMs from snapshot |
| Agent proxy | ✅ Created | Phase 2 complete - WebSocket + HTTP proxy |
| Agent systemd service | ✅ Running | Phase 3 complete - service enabled on clara2 |
| E2E test script | ❌ Missing | Need to create |
| Workspace server in MicroVM | ⚠️ Broken | Exits immediately, using sleep infinity |

## Prerequisites

Before testing:

1. **clara2 snapshot exists** with:
   - Firecracker binary installed
   - Agent binary at `/usr/local/bin/agent`
   - Kernel at `/var/lib/clarateach/images/vmlinux`
   - Rootfs at `/var/lib/clarateach/images/rootfs.ext4`
   - Systemd service configured

2. **GCP firewall rule** allows port 9090:
   ```bash
   gcloud compute firewall-rules create clarateach-agent \
     --project=clarateach \
     --allow=tcp:9090 \
     --target-tags=clarateach \
     --source-ranges=0.0.0.0/0
   ```

3. **Control plane** has `GCP_PROJECT` and credentials configured

## Test Commands

### Manual Testing

```bash
# Start server with GCP+Firecracker provisioner
GCP_PROJECT=clarateach \
GCP_ZONE=us-central1-b \
  go run ./cmd/server/

# Create workshop
curl -X POST http://localhost:8080/api/workshops \
  -H "Content-Type: application/json" \
  -d '{"name": "Test", "seats": 3, "runtime": "firecracker"}'

# Check status
curl http://localhost:8080/api/workshops/{id}

# Join as user
curl -X POST http://localhost:8080/api/join \
  -H "Content-Type: application/json" \
  -d '{"code": "ABC123"}'

# Access terminal (via proxy)
websocat wss://{worker-ip}:9090/proxy/{workshopID}/1/terminal

# Access files (via proxy)
curl https://{worker-ip}:9090/proxy/{workshopID}/1/files/

# Delete workshop
curl -X DELETE http://localhost:8080/api/workshops/{id}
```

### Automated Testing

```bash
# Run full E2E test
./scripts/test-e2e-gcp-firecracker.sh

# With custom settings
GCP_PROJECT=clarateach SEATS=5 ./scripts/test-e2e-gcp-firecracker.sh
```

## Open Questions

1. **Agent token**: Pass via GCP instance metadata or hardcode for now?
   - Recommendation: Instance metadata for security

2. **Timeout values**:
   - VM creation: 3 minutes
   - Agent health: 2 minutes
   - MicroVM creation: 30 seconds each

3. **Error handling**: What happens if agent never becomes healthy?
   - Delete the VM and fail the workshop creation

## Files to Create/Modify

| Action | File | Priority |
|--------|------|----------|
| Create | `internal/provisioner/gcp_firecracker.go` | P0 |
| Create | `internal/agentapi/proxy.go` | P1 |
| Modify | `internal/agentapi/server.go` | P1 |
| Modify | `internal/api/server.go` | P0 |
| Create | `scripts/test-e2e-gcp-firecracker.sh` | P0 |
| Modify | clara2 VM (systemd, agent binary) | P0 |

## Definition of Done

- [ ] Can create workshop with `runtime: firecracker`
- [ ] GCP VM created from snapshot as spot instance
- [ ] Agent starts automatically and passes health check
- [ ] MicroVMs created for each seat
- [ ] Terminal WebSocket works through proxy
- [ ] File API works through proxy
- [ ] Workshop deletion cleans up everything
- [ ] E2E test script passes
- [ ] Documentation updated
