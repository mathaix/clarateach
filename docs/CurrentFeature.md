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
| Agent VM cloudflared | ⬜ Not started | Install in snapshot, quick tunnel startup |
| Backend tunnel registration | ✅ Complete | `POST /api/internal/workshops/{id}/tunnel` |
| Backend JWT generation | ✅ Complete | Token in session response |
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
- [ ] Install `cloudflared` in VM snapshot
- [ ] Pass metadata: workshop-id, backend-url, workspace-token-secret
- [ ] Startup script runs quick tunnel, captures URL, reports to backend

### Phase 2: Backend Changes
- [x] Add `POST /api/internal/workshops/{id}/tunnel` endpoint
- [x] Store `tunnel_url` in workshop record
- [x] Add `WORKSPACE_TOKEN_SECRET` env var
- [x] Generate JWT in `/api/session/{code}` response
- [x] Return tunnel URL + token in session response

### Phase 3: Agent Auth Changes
- [x] Extract token from WebSocket URL query param or Authorization header
- [x] Validate JWT signature, expiry, claims
- [x] Reject connection if invalid

### Phase 4: Frontend Changes
- [x] Store token from session response
- [x] Append token to WebSocket URL

### Phase 5: Security Hardening
- [ ] Remove firewall rule for port 9090
- [ ] Restrict CORS to `learn.claramap.com`

## Files Reference

| File | Purpose |
|------|---------|
| `backend/internal/api/server.go` | Tunnel registration, JWT generation |
| `backend/internal/provisioner/gcp_firecracker.go` | Pass metadata to VM |
| `backend/internal/agentapi/proxy.go` | JWT validation |
| `scripts/prepare-snapshot.sh` | Install cloudflared |
| `scripts/clarateach-agent.service` | Quick tunnel startup |
| `frontend/src/pages/Workspace.tsx` | Token in WebSocket URL |
| `frontend/src/lib/api.ts` | Store token from session |

## Definition of Done

- [ ] All traffic over HTTPS
- [ ] VM reports tunnel URL to backend on boot
- [ ] WebSocket connections require valid JWT
- [ ] Invalid token → 401 rejection
- [ ] Direct IP access to agent blocked
