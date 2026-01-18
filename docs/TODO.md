# ClaraTeach TODO

## Critical Security Issues

### 1. Workspace Proxy Auth Disabled by Default (CRITICAL)

**Location**: `backend/internal/agentapi/proxy.go:33-110`, `server.go:65-106`

**Problem**: Token validation is skipped unless `WORKSPACE_TOKEN_SECRET` is set. WebSocket origins are fully permissive (`CheckOrigin` always returns true). Combined with public `/proxy` routes, anyone who can reach the agent can access learner terminals and files without authentication.

**Tasks**:
- [ ] Require `WORKSPACE_TOKEN_SECRET` - fail startup if not set in production
- [ ] Implement proper WebSocket origin checking
- [ ] Add authentication to all proxy routes by default

---

### 2. Hardcoded Fallback Secrets (CRITICAL)

**Location**: `backend/internal/auth/auth.go:41-57,59-89`

**Problem**: JWT and workspace tokens fall back to hardcoded secrets when env vars are missing. Attackers can mint valid user and workspace tokens. With auth bypassed, admin endpoints including SSH key download are exposed.

**Tasks**:
- [ ] Remove all hardcoded fallback secrets
- [ ] Fail startup if required secrets are not configured
- [ ] Add secret rotation mechanism

---

### 3. Token Scheme Mismatch (CRITICAL)

**Location**: `backend/internal/auth/auth.go:59-89`, `workspace/server/src/middleware/auth.ts:1-178`

**Problem**: Backend issues HS256 workspace tokens, but workspace server only accepts RS256 via JWKS. WebSocket/file auth fails unless `AUTH_DISABLED=true`, which disables auth entirely.

**Tasks**:
- [ ] Align token signing algorithms (both HS256 or both RS256)
- [ ] Remove `AUTH_DISABLED` option or restrict to dev mode only
- [ ] Add token validation tests across components

---

### 4. Predictable IDs and Access Codes (HIGH)

**Location**: `backend/internal/api/server.go:648-673` (also 204/223/510/740/1137)

**Problem**: Uses `math/rand` without seeding. Workshop IDs, access codes, and registrations are deterministic after each restart - easy to guess/collide.

**Tasks**:
- [ ] Replace `math/rand` with `crypto/rand`
- [ ] Add proper seeding if math/rand is needed for non-security purposes
- [ ] Audit all ID generation code paths

---

### 5. Broken Worker Capacity Checks (HIGH)

**Location**: `backend/internal/agentapi/server.go:169-178`, `backend/internal/orchestrator/firecracker.go:396-415`

**Problem**: `getVMCount` calls `List(nil, "")`, but Firecracker provider filters by `workshopID + "-"`, so empty workshopID returns zero VMs. Capacity enforcement never triggers, allowing unbounded VM creation.

**Tasks**:
- [ ] Fix `List()` to return all VMs when workshopID is empty
- [ ] Add integration tests for capacity enforcement
- [ ] Add monitoring/alerts for VM count

---

### 7. Public Tunnel Exposes Unauthenticated Proxies (CRITICAL)

**Location**: `backend/internal/agentapi/server.go:65-106`, `proxy.go:42-110`, `scripts/clarateach-tunnel.sh:53-93`

**Problem**: Agent quick tunnel publishes port 9090 to the internet. `/proxy/...` routes are intentionally unauthenticated, and workspace token validation is skipped unless `WORKSPACE_TOKEN_SECRET` is set. Default config gives anyone full terminal/files access via the tunnel.

**Tasks**:
- [ ] Require `WORKSPACE_TOKEN_SECRET` - fail startup if missing
- [ ] Add authentication to ALL proxy routes
- [ ] Consider cloudflared ingress rules to expose only specific paths

---

### 8. Unauthenticated Tunnel Registration (CRITICAL)

**Location**: `backend/internal/api/server.go:148-151,892-931`

**Problem**: `/api/internal/workshops/{id}/tunnel` is completely unauthenticated, assumes "internal" use. Anyone who can reach the backend can overwrite a workshop's tunnel URL to point users to an attacker-controlled endpoint. Quick tunnels are world-accessible, so "internal" assumption is invalid.

**Tasks**:
- [ ] Add authentication to tunnel registration endpoint (shared secret or agent token)
- [ ] Restrict to agent CIDRs or mTLS
- [ ] Validate tunnel URL format/domain

---

### 9. Tunnel Exposes Full Agent Control API (HIGH)

**Location**: `backend/cmd/agent/main.go:25-81`, `backend/internal/agentapi/server.go:83-97`

**Problem**: Quick tunnel points at `:9090` which serves BOTH `/proxy` (user access) AND agent lifecycle routes (`/vms`, `/info`). Auth relies on `AGENT_TOKEN`; if unset (allowed for "local dev"), internet users can create/destroy MicroVMs via the tunnel.

**Tasks**:
- [ ] Require `AGENT_TOKEN` in production - fail startup if missing
- [ ] Separate proxy routes from control routes (different ports or paths)
- [ ] Use cloudflared ingress rules to only expose `/proxy/*` and `/health`
- [ ] Or add local reverse-proxy to filter exposed endpoints

---

### 10. Incomplete Firewall Hardening (MEDIUM)

**Location**: `docs/CurrentFeature.md:92-95`

**Problem**: Doc lists "remove GCP firewall rule for port 9090" as not done. Public IP + tunnel are both open unless manually locked down.

**Tasks**:
- [ ] Remove GCP firewall rule for port 9090 (traffic should only go through tunnel)
- [ ] Document required firewall rules for production
- [ ] Add terraform/scripts to enforce firewall state

---

## Architecture Decisions Needed

### Decision 1: Tunnel Registration Authentication
**Question**: Should tunnel registration be authenticated (shared secret or mTLS) and restricted to agent CIDRs?

**Options**:
- A) Shared secret in header (simple, requires secret management)
- B) mTLS (most secure, more complex setup)
- C) Restrict to VPC internal IPs only (relies on network isolation)

**Recommendation**: Option A (shared secret) as minimum, with Option C as defense-in-depth.

---

### Decision 2: Mandatory Token Validation
**Question**: Should workspace token validation be mandatory (fail startup if `WORKSPACE_TOKEN_SECRET` missing)?

**Options**:
- A) Yes, always required
- B) Required unless explicit `DEV_MODE=true`
- C) Keep current optional behavior

**Recommendation**: Option B - require in production, allow bypass only with explicit dev flag.

---

### Decision 3: Single Token Signing Scheme
**Question**: Should we use HS256 or RS256 for workspace tokens?

**Options**:
- A) HS256 everywhere (simpler, shared secret)
- B) RS256 everywhere (asymmetric, JWKS support)
- C) Keep mismatch and fix workspace server

**Recommendation**: Option A (HS256) for simplicity - workspace server should accept HS256 tokens signed with shared secret.

---

### Decision 4: Tunnel Path Exposure
**Question**: Should the tunnel expose only locked-down paths instead of the whole agent port?

**Options**:
- A) Cloudflared ingress rules (filter at tunnel level)
- B) Local reverse-proxy on agent (nginx/caddy filtering)
- C) Separate ports: 9090 for control (firewalled), 9091 for proxy (tunneled)

**Recommendation**: Option C - cleanest separation of concerns.

---

### 6. SSH Keys in Plaintext + CORS/CSRF Issues (MEDIUM)

**Location**: `backend/internal/store/sqlite.go:200-206,306-313`, `backend/internal/api/server.go:135-146,81-89`, `backend/internal/agentapi/server.go:65-78`

**Problem**:
- SSH private keys stored in plaintext and returned via admin API
- CORS set to `*` with credentials enabled on both APIs
- Service is CSRFable when tokens are in localStorage

**Tasks**:
- [ ] Encrypt SSH keys at rest
- [ ] Restrict CORS origins in production (not `*`)
- [ ] Remove `AllowCredentials: true` when using `*` origin (already fixed in agent)
- [ ] Consider using httpOnly cookies instead of localStorage for tokens
- [ ] Add CSRF protection

---

## High Priority

### MicroVM Persistence (Risk: MEDIUM-HIGH)

**Problem**: When the agent restarts, all MicroVM state is lost because the VM registry is in-memory only.

**Impact**:
- Users lose their work (unsaved terminal state)
- Orphaned Firecracker processes consume resources
- All active sessions break and can't reconnect
- Manual intervention required to clean up

**When it happens**:
- Agent code deployments
- Agent crashes
- VM reboots/maintenance
- systemd restarts

**Tasks**:
- [ ] Design persistence strategy (file vs socket discovery)
- [ ] Implement VM state file persistence (`/var/lib/clarateach/vms.json`)
- [ ] Add VM discovery on agent startup (scan for running Firecracker processes)
- [ ] Handle orphaned VM cleanup
- [ ] Add graceful agent shutdown with user notification

**Recommended approach**:
1. Quick fix: Persist VM state to JSON file, reload on startup
2. Better: Use Firecracker socket files as source of truth, re-attach on startup
3. Best: Graceful shutdown + VM snapshots + centralized registry

---

## Medium Priority

### Tunnel URL Auto-Registration

**Problem**: Quick tunnel URL capture script doesn't reliably extract the URL.

**Tasks**:
- [ ] Fix startup script URL parsing
- [ ] Add retry logic for URL registration
- [ ] Consider using named Cloudflare tunnels for stable URLs

---

## Low Priority

### Dashboard Create Workshop Button

**Problem**: Browser automation couldn't click the Create Workshop button reliably.

**Tasks**:
- [ ] Investigate form submission issues
- [ ] Add better error handling for form failures

---

## Completed

- [x] CORS fix for file browser (duplicate headers from MicroVM)
- [x] JWT token authentication for workspace access
- [x] Cloudflare Quick Tunnel integration
