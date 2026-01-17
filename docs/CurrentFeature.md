# Current Feature: HTTPS/TLS Security with Quick Tunnels

> **Workflow**: See [CurrentFeatureWorkflow.md](CurrentFeatureWorkflow.md) for how this document is managed through the release cycle.

## Goal

Secure all ClaraTeach traffic with HTTPS using Cloudflare for the portal and Quick Tunnels for dynamic VM access.

**Portal Domain:** `learn.claramap.com`
**VM Tunnels:** `*.trycloudflare.com` (auto-generated, hidden from users)

## Current State Summary

| Component | Status | Notes |
|-----------|--------|-------|
| Cloudflare DNS (portal) | ⬜ Not started | `learn.claramap.com` → origin |
| Agent VM cloudflared | ✅ Complete | Scripts ready, install in snapshot |
| Backend tunnel registration | ✅ Complete | `POST /api/internal/workshops/{id}/tunnel` |
| Backend JWT generation | ✅ Complete | Token in session response |
| Backend VM metadata | ✅ Complete | Passes workshop-id, backend-url, workspace-token-secret |
| Agent JWT validation | ✅ Complete | Validates query param or Authorization header |
| Frontend token handling | ✅ Complete | Passes token to WebSocket and HTTP requests |

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

**VM Boot Flow:**
```bash
# 1. Start quick tunnel
cloudflared tunnel --url http://localhost:9090
# Output: https://calm-river-1234.trycloudflare.com

# 2. Report URL to backend
curl -X POST "$BACKEND_URL/api/internal/workshops/$WORKSHOP_ID/tunnel" \
  -d '{"tunnel_url": "https://calm-river-1234.trycloudflare.com"}'
```

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
- [x] Create `clarateach-tunnel.service` systemd unit
- [x] Create `clarateach-tunnel.sh` startup script
- [x] Update `clarateach-agent.service` to fetch workspace-token-secret
- [x] Update `prepare-snapshot.sh` with cloudflared verification
- [x] Pass metadata via GCP: workshop-id, backend-url, workspace-token-secret
- [ ] **Deploy**: Install `cloudflared` on VM and create new snapshot

### Phase 2: Backend Changes
- [x] Add `POST /api/internal/workshops/{id}/tunnel` endpoint
- [x] Store `tunnel_url` in workshop record
- [x] Add `WORKSPACE_TOKEN_SECRET` env var
- [x] Generate JWT in `/api/session/{code}` response
- [x] Return tunnel URL + token in session response
- [x] Pass `BACKEND_URL` and `WORKSPACE_TOKEN_SECRET` to provisioner

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
| `backend/internal/api/server.go` | Tunnel registration, JWT generation |
| `backend/internal/provisioner/gcp_firecracker.go` | Pass metadata to VM |
| `backend/internal/agentapi/proxy.go` | JWT validation |
| `backend/internal/auth/auth.go` | Workspace token functions |
| `backend/internal/store/store.go` | TunnelURL field in WorkshopVM |
| `backend/cmd/server/main.go` | BACKEND_URL, WORKSPACE_TOKEN_SECRET config |
| `scripts/prepare-snapshot.sh` | Verify cloudflared installed |
| `scripts/clarateach-agent.service` | Agent service with token secret |
| `scripts/clarateach-tunnel.service` | Quick tunnel systemd service |
| `scripts/clarateach-tunnel.sh` | Quick tunnel startup + URL reporting |
| `frontend/src/pages/SessionWorkspace.tsx` | Token in WebSocket URL |
| `frontend/src/lib/api.ts` | Store token from session |

## Definition of Done

- [ ] All traffic over HTTPS
- [ ] VM reports tunnel URL to backend on boot
- [ ] WebSocket connections require valid JWT
- [ ] Invalid token → 401 rejection
- [ ] Direct IP access to agent blocked
