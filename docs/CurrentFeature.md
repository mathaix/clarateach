# Current Feature: HTTPS/TLS Security with Quick Tunnels

> **Workflow**: See [CurrentFeatureWorkflow.md](CurrentFeatureWorkflow.md) for how this document is managed through the release cycle.

## Goal

Secure all ClaraTeach traffic with HTTPS using Cloudflare for the portal and Quick Tunnels for dynamic VM access.

**Portal Domain:** `learn.claramap.com`
**VM Tunnels:** `*.trycloudflare.com` (auto-generated, hidden from users)

## Current State Summary

| Component | Status | Notes |
|-----------|--------|-------|
| Cloudflare DNS (portal) | ⚠️ Partial | Tunnels created, need to connect cloudflared |
| Agent VM cloudflared | ✅ Complete | Installed in snapshot, managed by Go code |
| Backend tunnel registration | ✅ Complete | `POST /api/internal/workshops/{id}/tunnel` |
| Backend JWT generation | ✅ Complete | Token in session response |
| Backend VM metadata | ✅ Complete | Passes workshop-id, backend-url, workspace-token-secret |
| Agent JWT validation | ✅ Complete | Validates query param or Authorization header |
| Frontend token handling | ✅ Complete | Passes token to WebSocket and HTTP requests |
| Network bridge setup | ✅ Complete | Go code in `internal/network/` (no bash scripts) |
| Tunnel manager | ✅ Complete | Go code in `internal/tunnel/` (no bash scripts) |

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              USER BROWSER                                    │
│                                                                             │
│   User sees: https://learn.claramap.com (portal with embedded workspace)   │
│   User NEVER sees the trycloudflare.com URLs                               │
└───────────────┬─────────────────────────────────────┬───────────────────────┘
                │                                     │
                │ https://learn.claramap.com          │ wss://xxx.trycloudflare.com
                │ (portal + API)                      │ (hidden, embedded in UI)
                ▼                                     ▼
┌───────────────────────────────┐    ┌────────────────────────────────────────┐
│   CLOUDFLARE EDGE             │    │   CLOUDFLARE QUICK TUNNEL              │
│                               │    │                                        │
│   learn.claramap.com          │    │   *.trycloudflare.com                  │
│   (Proxied → Origin)          │    │   (Auto-generated per VM)              │
│   • TLS termination           │    │   • No DNS config needed               │
│   • DDoS protection           │    │   • Auto-cleanup when VM stops         │
└───────────────┬───────────────┘    └───────────────┬────────────────────────┘
                │                                     │
                ▼                                     ▼
┌───────────────────────────────┐    ┌────────────────────────────────────────┐
│   CONTROL PLANE               │    │   AGENT VM (per workshop)              │
│                               │    │                                        │
│   Backend:  :8080             │    │   cloudflared quick tunnel             │
│   - /api/workshops            │    │      → reports URL to backend          │
│   - /api/session/{code}       │    │   Agent API (:9090)                    │
│       → returns tunnel URL    │    │      → validates JWT token             │
│       → returns JWT token     │    │   MicroVMs (192.168.100.x)             │
└───────────────────────────────┘    └────────────────────────────────────────┘
```

## Quick Tunnels Approach

**Why Quick Tunnels:**
- No Cloudflare API calls needed
- No DNS management
- Auto-cleanup when VM stops
- Users never see the tunnel URLs (embedded in portal UI)

**VM Boot Flow (all in Go):**
```
1. Agent starts (systemd clarateach-agent.service)
2. internal/network.SetupBridge() creates fcbr0 bridge
3. internal/tunnel.Manager starts cloudflared subprocess
4. Manager captures tunnel URL from cloudflared stderr
5. Manager POSTs tunnel URL to backend API
6. Agent waits for tunnel registration (2 min timeout)
7. Agent ready to receive VM creation requests
```

**Key:** The tunnel URL is registered DURING provisioning, so the backend must create the VM database record BEFORE calling `CreateVM()` to avoid the race condition.

## Token-Based WebSocket Auth

```
1. User calls GET /api/session/{accessCode}
2. Backend generates JWT: { workshop_id, seat, exp: now+2h }
3. Backend returns: { endpoint (tunnel URL), token, seat }
4. Frontend connects: wss://xxx.trycloudflare.com/proxy/ws-xxx/1/terminal?token=eyJ...
5. Agent validates: signature, expiry, workshop_id matches path
6. If invalid → 401 Unauthorized
```

## Implementation Tasks

### Phase 1: Agent VM Changes
- [x] ~~Create `clarateach-tunnel.service` systemd unit~~ (replaced by Go code)
- [x] ~~Create `clarateach-tunnel.sh` startup script~~ (replaced by Go code)
- [x] Move network setup to Go (`internal/network/bridge_linux.go`)
- [x] Move tunnel management to Go (`internal/tunnel/manager.go`)
- [x] Pass metadata via GCP: workshop-id, backend-url, workspace-token-secret
- [x] **Deploy**: Install `cloudflared` on VM and create new snapshot (`clarateach-agent-20260117-230800`)

### Phase 2: Backend Changes
- [x] Add `POST /api/internal/workshops/{id}/tunnel` endpoint
- [x] Store `tunnel_url` in workshop record
- [x] Add `WORKSPACE_TOKEN_SECRET` env var
- [x] Generate JWT in `/api/session/{code}` response
- [x] Return tunnel URL + token in session response
- [x] Pass `BACKEND_URL` and `WORKSPACE_TOKEN_SECRET` to provisioner
- [x] Fix tunnel URL race condition (create VM record before provisioning)
- [x] Keep VMs on failure for debugging (no auto-delete)

### Phase 3: Agent Auth Changes
- [x] Extract token from WebSocket URL query param or Authorization header
- [x] Validate JWT signature, expiry, claims
- [x] Reject connection if invalid

### Phase 4: Frontend Changes
- [x] Store token from session response
- [x] Append token to WebSocket URL

### Phase 5: Security Hardening
- [ ] **Deploy**: Remove GCP firewall rule for port 9090
- [x] Add `CORS_ORIGINS` env var (comma-separated list, default: `*`)
- [ ] **Deploy**: Set `CORS_ORIGINS=https://learn.claramap.com` in production

## Files Reference

| File | Purpose |
|------|---------|
| `backend/internal/api/server.go` | Tunnel registration, JWT generation, VM provisioning |
| `backend/internal/provisioner/gcp_firecracker.go` | Pass metadata to VM, keeps VM on failure |
| `backend/internal/agentapi/proxy.go` | JWT validation |
| `backend/internal/auth/auth.go` | Workspace token functions |
| `backend/internal/store/store.go` | TunnelURL field in WorkshopVM |
| `backend/internal/network/bridge_linux.go` | MicroVM bridge network setup (Go, replaces bash) |
| `backend/internal/tunnel/manager.go` | Cloudflare tunnel manager (Go, replaces bash) |
| `backend/cmd/server/main.go` | BACKEND_URL, WORKSPACE_TOKEN_SECRET config |
| `backend/cmd/agent/main.go` | Agent entry point with network + tunnel setup |
| `backend/scripts/create-agent-snapshot.sh` | Creates GCP snapshot with agent pre-installed |
| `frontend/src/pages/SessionWorkspace.tsx` | Token in WebSocket URL |
| `frontend/src/lib/api.ts` | Store token from session |

## Learnings & Gotchas

### 1. Tunnel URL Race Condition (Critical)
**Problem:** The tunnel URL registration happened during `prov.CreateVM()`, but the VM database record was only created AFTER `CreateVM()` returned. The `UPDATE workshop_vms SET tunnel_url = ...` affected 0 rows because the row didn't exist yet.

**Solution:** Create the VM record in the database BEFORE starting provisioning (with status "PROVISIONING"), then update it after provisioning completes. This ensures the tunnel URL can be registered during the provisioning process.

### 2. Bash Scripts Are Fragile
**Problem:** Systemd `ExecStartPre` scripts for network setup failed silently or were missing from snapshots, causing agent startup failures.

**Solution:** Move all runtime logic into Go code:
- `internal/network/bridge_linux.go` - Bridge network setup (with `bridge_stub.go` for non-Linux)
- `internal/tunnel/manager.go` - Cloudflare tunnel spawning and URL registration

Benefits:
- Single binary deployment
- Better error handling and logging
- Build-time type checking
- Easier debugging

### 3. Keep VMs on Failure for Debugging
**Problem:** Auto-deleting VMs on provisioning failure made debugging impossible.

**Solution:** On failure, log the VM name and keep it running. Include instructions for accessing serial console:
```
gcloud compute instances get-serial-port-output VM_NAME --zone=ZONE
```

### 4. Mixed Content Blocking
**Problem:** HTTPS frontend trying to connect to HTTP VM IP causes browser to block the request.

**Solution:** Always use the tunnel URL (HTTPS) instead of direct IP. The tunnel URL is registered via the internal API and stored in the database.

## Deployment: Portal VM Setup

**Target:** Single small VM with Cloudflare Tunnel (no exposed ports)

```
learn.claramap.com → Cloudflare Tunnel → e2-small VM
                                              ├── nginx (frontend :80)
                                              ├── backend (:8080)
                                              └── SQLite DB
```

### 1. Create VM
```bash
gcloud compute instances create clarateach-portal \
  --project=clarateach \
  --zone=us-central1-a \
  --machine-type=e2-small \
  --image-family=ubuntu-2204-lts \
  --image-project=ubuntu-os-cloud \
  --boot-disk-size=20GB
```

### 2. Install Dependencies
```bash
# On the VM
sudo apt update && sudo apt install -y nginx

# Install cloudflared
curl -L https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 \
  -o /usr/local/bin/cloudflared
chmod +x /usr/local/bin/cloudflared
```

### 3. Create Cloudflare Tunnel
```bash
# Login to Cloudflare
cloudflared tunnel login

# Create tunnel
cloudflared tunnel create clarateach-portal

# Route DNS
cloudflared tunnel route dns clarateach-portal learn.claramap.com
```

### 4. Configure Tunnel (`/etc/cloudflared/config.yml`)
```yaml
tunnel: <TUNNEL_ID>
credentials-file: /root/.cloudflared/<TUNNEL_ID>.json

ingress:
  - hostname: learn.claramap.com
    path: /api/*
    service: http://localhost:8080
  - hostname: learn.claramap.com
    service: http://localhost:80
  - service: http_status:404
```

### 5. Systemd Service (`/etc/systemd/system/cloudflared.service`)
```ini
[Unit]
Description=Cloudflare Tunnel
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/cloudflared tunnel run
Restart=always

[Install]
WantedBy=multi-user.target
```

### 6. Deploy Backend
```bash
# Copy binary and create service
scp backend/server clarateach-portal:/usr/local/bin/clarateach-backend

# Environment file: /etc/clarateach/backend.env
GCP_PROJECT=clarateach
GCP_ZONE=us-central1-a
GCP_REGISTRY=us-central1-docker.pkg.dev/clarateach/clarateach
FC_SNAPSHOT_NAME=clara2-snapshot
FC_AGENT_TOKEN=<token>
BACKEND_URL=https://learn.claramap.com
WORKSPACE_TOKEN_SECRET=<secret>
CORS_ORIGINS=https://learn.claramap.com
DB_PATH=/var/lib/clarateach/clarateach.db
```

### 7. Deploy Frontend
```bash
cd frontend && npm run build
scp -r dist/* clarateach-portal:/var/www/html/
```

### 8. Start Services
```bash
sudo systemctl enable --now cloudflared
sudo systemctl enable --now clarateach-backend
sudo systemctl restart nginx
```

## Definition of Done

- [ ] All traffic over HTTPS via Cloudflare Tunnel
- [ ] VM reports tunnel URL to backend on boot
- [ ] WebSocket connections require valid JWT
- [ ] Invalid token → 401 rejection
- [ ] Direct IP access to agent blocked
- [ ] Portal accessible at https://learn.claramap.com
