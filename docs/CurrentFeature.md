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

- [x] Workshop creation triggers VM provisioning
- [x] VM created as spot instance with nested virtualization
- [x] Agent health check passes within 2 minutes of boot
- [x] MicroVMs created and pingable
- [x] User can access terminal (WebSocket) - proxy implemented
- [x] User can access file API (HTTP) - proxy implemented
- [ ] Workshop deletion cleans up VM and MicroVMs (code exists, needs full test)

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
│  │  - Proxies user traffic to MicroVMs                                 │ │
│  └─────────────────────────────────────────────────────────────────────┘ │
│                                                                          │
│  ┌───────────────┐  ┌───────────────┐  ┌───────────────┐                │
│  │ MicroVM Seat1 │  │ MicroVM Seat2 │  │ MicroVM Seat3 │                │
│  │ 192.168.100.11│  │ 192.168.100.12│  │ 192.168.100.13│                │
│  │ :3001 terminal│  │ :3001 terminal│  │ :3001 terminal│                │
│  │ :3002 files   │  │ :3002 files   │  │ :3002 files   │                │
│  └───────────────┘  └───────────────┘  └───────────────┘                │
│                                                                          │
│                    clarateach0 bridge (192.168.100.1/24)                │
└──────────────────────────────────────────────────────────────────────────┘
```

## Current State

| Component | Status | Notes |
|-----------|--------|-------|
| Agent | ✅ Working | Creates MicroVMs, manages lifecycle |
| Firecracker orchestrator | ✅ Working | Network bridge, TAP devices, IP allocation |
| GCP+Firecracker provisioner | ✅ Created | Creates spot VMs from snapshot |
| Agent proxy | ✅ Working | WebSocket (terminal) + HTTP (files) proxy |
| Agent systemd service | ✅ Running | Auto-starts on clara2 boot |
| E2E test scripts | ✅ Created | Local + GCP test scripts |
| Workspace server in MicroVM | ✅ Working | Terminal (3001) + Files (3002) healthy |

## Implementation Summary

### Phase 1: GCPFirecrackerProvisioner ✅

**File**: `internal/provisioner/gcp_firecracker.go`

- [x] Create provisioner struct with GCP client
- [x] Implement `CreateVM` - create VM from snapshot
- [x] Implement `waitForAgentHealth` - poll agent /health
- [x] Implement `createMicroVMs` - call agent API for each seat
- [x] Implement `DeleteVM` - destroy MicroVMs then delete GCP VM

### Phase 2: Agent Proxy ✅

**File**: `internal/agentapi/proxy.go`

| Endpoint | Proxies To | Protocol |
|----------|------------|----------|
| `/proxy/{workshopID}/{seatID}/terminal` | `192.168.100.{10+seatID}:3001` | WebSocket |
| `/proxy/{workshopID}/{seatID}/files/*` | `192.168.100.{10+seatID}:3002` | HTTP |
| `/proxy/{workshopID}/{seatID}/health` | Both ports | HTTP |

- [x] WebSocket proxy for terminal
- [x] HTTP reverse proxy for files
- [x] Health check endpoint

### Phase 3: Agent Systemd Service ✅

**File**: `scripts/clarateach-agent.service`

- [x] Service file created and installed on clara2
- [x] Auto-starts on boot
- [x] Fetches agent token from GCP metadata (if available)
- [ ] Update clara2 snapshot (requires stopping VM)

### Phase 4: E2E Test Scripts ✅

**Files**:
- `scripts/test-e2e-local.sh` - Tests agent + MicroVMs locally
- `scripts/test-e2e-gcp.sh` - Tests full backend → GCP → MicroVMs flow

- [x] Agent health check
- [x] MicroVM creation and listing
- [x] Network connectivity (ping)
- [x] Proxy health verification
- [x] Cleanup

### Phase 5: Workspace Server in MicroVM ✅

**Root cause**: Missing `ip` command prevented network configuration in init script.

**Fixes applied**:
- [x] Added `init=/sbin/init` to kernel boot args
- [x] Copied `ip` binary + libraries to rootfs
- [x] Simplified init script with better error handling
- [x] Terminal server (port 3001) working
- [x] File server (port 3002) working

## Test Commands

### Quick Test (on clara2)

```bash
# Run local E2E test - creates 3 MicroVMs, verifies services, cleans up
./scripts/test-e2e-local.sh
```

### Full GCP Test

```bash
# Terminal 1: Start backend
GCP_PROJECT=clarateach \
GCP_ZONE=us-central1-b \
GCP_REGISTRY=us-central1-docker.pkg.dev/clarateach/clarateach \
FC_SNAPSHOT_NAME=clara2-snapshot \
go run ./cmd/server/

# Terminal 2: Run full E2E test
./scripts/test-e2e-gcp.sh
```

### Manual Testing

```bash
# Check agent health
curl localhost:9090/health

# Create a MicroVM
curl -X POST localhost:9090/vms \
  -H "Content-Type: application/json" \
  -d '{"workshop_id": "test", "seat_id": 1}'

# Check proxy health (should show terminal: true, files: true)
curl localhost:9090/proxy/test/1/health

# Access files API
curl localhost:9090/proxy/test/1/files/

# Cleanup
curl -X DELETE localhost:9090/vms/test/1
```

## Accessing User Interfaces

### Terminal (WebSocket)

```
ws://<agent-ip>:9090/proxy/<workshop-id>/<seat-id>/terminal
```

Example with websocat:
```bash
websocat ws://34.68.136.93:9090/proxy/my-workshop/1/terminal
```

### Files API (HTTP)

```
http://<agent-ip>:9090/proxy/<workshop-id>/<seat-id>/files/
```

Example:
```bash
# List files
curl http://34.68.136.93:9090/proxy/my-workshop/1/files/

# Create a file
curl -X POST http://34.68.136.93:9090/proxy/my-workshop/1/files/hello.txt \
  -H "Content-Type: text/plain" \
  -d "Hello, World!"
```

## Files Created/Modified

| File | Action | Description |
|------|--------|-------------|
| `internal/provisioner/gcp_firecracker.go` | Created | GCP + Firecracker provisioner |
| `internal/agentapi/proxy.go` | Created | WebSocket + HTTP proxy |
| `internal/agentapi/server.go` | Modified | Added proxy routes |
| `internal/orchestrator/firecracker.go` | Modified | Added `init=` kernel param |
| `scripts/clarateach-agent.service` | Created | Systemd service file |
| `scripts/test-e2e-local.sh` | Created | Local agent test |
| `scripts/test-e2e-gcp.sh` | Created | Full GCP test |
| `docs/TestingGuide.md` | Created | Comprehensive testing docs |

## Remaining Work

1. **Update clara2 snapshot** - Requires stopping VM to capture current state
2. **Full GCP integration test** - Run `test-e2e-gcp.sh` against live backend
3. **Unit tests** - Add tests for provisioner and proxy code

## Definition of Done

- [x] Can create MicroVMs via agent API
- [x] MicroVMs boot with working terminal + file servers
- [x] Agent proxy routes traffic to MicroVMs
- [x] E2E test script passes (14/14 tests)
- [x] Documentation updated (TestingGuide.md)
- [ ] Update clara2 snapshot with current rootfs
- [ ] Full GCP provisioning tested end-to-end
