# ClaraTeach - Open Issues & Clarifications

This document tracks open questions and inconsistencies identified during documentation review.

---

## Issue #1: Instructor Authentication

**Status:** Open
**Priority:** High (MVP blocker)
**Affected Files:** `ARCHITECTURE.md`, `API_SPEC.md`

### Description

Conflicting documentation on instructor authentication for MVP:

- **ARCHITECTURE.md** states: "Future: Google OAuth/SSO, MVP: Simple password or magic link"
- **API_SPEC.md** states: "MVP: No authentication required for instructor endpoints"

### Question

For MVP, should instructor endpoints be:
1. Completely open (no auth) - simpler but insecure
2. Protected by shared password - simple auth
3. Protected by magic link - passwordless but requires email

### Impact

Determines security posture of admin API and implementation complexity.

---

## Issue #2: TURN Server Missing from Infrastructure

**Status:** Open
**Priority:** Medium
**Affected Files:** `ARCHITECTURE.md`, `INFRASTRUCTURE.md`

### Description

ARCHITECTURE.md added TURN ports to firewall rules:
- 3478/udp (STUN/TURN)
- 5349/tcp (TURN over TLS)
- 443/udp (TURN over DTLS)

However:
- No TURN server (coturn) defined in Terraform
- No TURN configuration in container setup
- neko requires TURN for WebRTC through restrictive NAT

### Question

Should we:
1. Include coturn in the workspace VM (adds complexity)
2. Use a managed TURN service (Twilio, Xirsys - adds cost)
3. Rely on STUN only for MVP (limits connectivity for some learners)

### Impact

Learners behind symmetric NAT won't be able to use browser preview without TURN.

---

## Issue #3: Playwright and Neko Browser Interaction

**Status:** Open
**Priority:** Medium
**Affected Files:** `ARCHITECTURE.md`, `IMPLEMENTATION_*.md`

### Description

The architecture shows:
- Playwright MCP server in learner container for browser automation
- neko container providing browser preview via WebRTC

The interaction between these is unclear:
- Does Playwright control neko's browser via Chrome DevTools Protocol (CDP)?
- Or are there two separate browsers (Playwright headless + neko visible)?

### Question

How should Playwright integration work:
1. **Shared browser:** Playwright connects to neko's Chromium via CDP (port 9222)
2. **Separate browsers:** Playwright runs headless, screen is mirrored/streamed
3. **Deferred:** Skip Playwright integration for MVP

### Impact

Affects container networking, neko configuration, and demo capabilities.

---

## Issue #4: Token Revocation Mechanism

**Status:** Open
**Priority:** Low
**Affected Files:** `ARCHITECTURE.md`, `API_SPEC.md`, `IMPLEMENTATION_*.md`

### Description

ARCHITECTURE.md mentions checking a revocation list for JWT validation, but:
- No revoke endpoint in API_SPEC.md
- No revocation storage defined
- No implementation details in either TS or Python docs

### Question

For MVP, is token revocation needed? If so:
1. Where should the revocation list be stored?
   - VM metadata (persistent, shared)
   - In-memory on workspace VM (fast, lost on restart)
   - Redis/file on workspace VM
2. When should tokens be revoked?
   - Admin kicks learner
   - Workshop stops
   - Manual intervention

### Impact

Without revocation, a stolen token remains valid until expiry (1-2 hours per ARCHITECTURE.md).

---

## Issue #5: Per-Learner Docker Network Isolation

**Status:** Open
**Priority:** Medium
**Affected Files:** `ARCHITECTURE.md`, `IMPLEMENTATION_*.md`

### Description

ARCHITECTURE.md specifies:
> "One Docker network per learner container"

But implementation docs show containers on a shared bridge network with Caddy routing.

### Question

Should each learner's containers (workspace + neko) be:
1. **Isolated networks:** Separate Docker network per learner (stronger isolation)
2. **Shared network:** All containers on one bridge, rely on container firewall rules
3. **Hybrid:** Shared network for MVP, isolated networks for production

### Impact

Affects:
- Container-to-container communication
- Security posture
- Caddy/proxy configuration complexity

---

## Issue #6: DNS-01 Challenge Configuration

**Status:** Open
**Priority:** High (TLS required for WebRTC)
**Affected Files:** `ARCHITECTURE.md`, `INFRASTRUCTURE.md`

### Description

ARCHITECTURE.md specifies DNS-01 for TLS certificates, which requires:
- DNS provider API access for Caddy
- Service account with `dns.admin` role
- Zone already configured in Cloud DNS

Current gaps:
- Service account permissions not in Terraform
- Caddy DNS plugin configuration not documented
- Domain/zone setup not specified

### Questions

1. Is the domain zone already in Cloud DNS?
2. What is the domain pattern? (e.g., `{workshop-id}.clarateach.io`)
3. Should Terraform create the service account with `dns.admin` role?
4. Which Caddy DNS plugin? (`caddy-dns/cloudflare` or `caddy-dns/gcp`?)

### Impact

Without proper DNS-01 setup, TLS certificates cannot be provisioned, breaking WebRTC.

---

---

## Issue #7: Learner Name Collection

**Status:** Open
**Priority:** Medium
**Affected Files:** `API_SPEC.md`, `IMPLEMENTATION_*.md`

### Description

Wireframes show collecting learner name on join, but current API spec doesn't include it:

- `POST /api/join` only accepts `code` and `odehash`
- No `name` field in request or JWT payload
- TeacherClassView shows learner names in the list

### Recommendation

Add `name` field to:
1. `POST /api/join` request body (optional, default to "Learner N")
2. JWT payload for display purposes
3. Session/seat metadata for instructor visibility

---

## Issue #8: Reconnect Code (Odehash) UI

**Status:** Open
**Priority:** Medium
**Affected Files:** Wireframes (`Workspace.tsx`, `LearnerJoin.tsx`)

### Description

Documentation specifies learners should save their odehash (5-char reconnect code) to rejoin if disconnected, but:

- Wireframes don't display the odehash to learners after joining
- No UI for entering odehash on reconnect (only class code field)
- LearnerJoin info box says "You can rejoin if disconnected" but doesn't explain how

### Recommendation

Add to Workspace header or modal:
1. Display odehash prominently after successful join
2. "Save this code to rejoin" messaging
3. Add odehash field to LearnerJoin (or auto-detect from localStorage)

---

## Issue #9: Teacher Class View Terminology

**Status:** Open
**Priority:** Low
**Affected Files:** Wireframes (`TeacherClassView.tsx`)

### Description

TeacherClassView shows status indicators with unclear terminology:

- "Workshop Machine" - should be "Workspace VM"
- "Front Door" - unclear, possibly Caddy/proxy?
- "Secure Vault" - unclear, possibly API key storage?

### Recommendation

Align terminology with architecture docs:
- "Workspace VM" - GCE instance status
- "Proxy" - Caddy reverse proxy status
- "API Key" - Claude API key configured

---

## Issue #10: Workspace Auth Disabled by Default

**Status:** Open
**Priority:** High
**Affected Files:** `workspace/docker-compose.yml`

### Description

`AUTH_DISABLED` defaults to `true` when not set, which disables workspace authentication in the main compose setup:

- `AUTH_DISABLED=${AUTH_DISABLED:-true}`
- Caddy exposes the workspace on `${CADDY_PORT:-8000}`

If this compose file is used outside local development, the workspace is accessible without auth.

### Recommendation

Default `AUTH_DISABLED` to `false` in non-local compose files and require explicit opt-out for local dev (or bind Caddy to `127.0.0.1`).

---

## Issue #11: Neko Exposed with Static Credentials

**Status:** Open
**Priority:** High
**Affected Files:** `workspace/docker-compose.yml`, `workspace/docker-compose.local.yml`

### Description

Neko exposes port 8080 with static passwords and control protections disabled:

- `NEKO_PASSWORD=neko`, `NEKO_PASSWORD_ADMIN=admin`
- `NEKO_CONTROL_PROTECTION=false`, `NEKO_IMPLICIT_CONTROL=true`

Anyone who can reach port 8080 can control the shared browser.

### Recommendation

Require passwords via env vars, enable control protection, and consider binding 8080 to localhost in local-only setups.

---

## Issue #12: Neko Requires SYS_ADMIN Capability

**Status:** Open
**Priority:** Medium
**Affected Files:** `workspace/docker-compose.yml`, `workspace/docker-compose.local.yml`

### Description

The Neko container is granted `SYS_ADMIN`, which broadens container privileges.

### Question

Is `SYS_ADMIN` required for Neko's Firefox image? If not, drop the capability or replace with narrower permissions.

---

## Issue #13: Workspace Image Builds Are Not Pinned

**Status:** Open
**Priority:** Medium
**Affected Files:** `workspace/Dockerfile`

### Description

The workspace image installs Node.js via a remote setup script and installs global npm packages without pinning:

- `curl -fsSL https://deb.nodesource.com/setup_20.x | bash -`
- `npm install -g @anthropic-ai/claude-code`
- `npm install -g pino-pretty`

This can lead to non-reproducible builds and unexpected breakages.

### Recommendation

Pin versions and/or use a base image that already provides Node 20, then install global packages with fixed versions.

---

## Issue #14: Healthchecks Missing in Main/Local Compose

**Status:** Open
**Priority:** Low
**Affected Files:** `workspace/docker-compose.yml`, `workspace/docker-compose.local.yml`

### Description

The orchestrator template defines a workspace healthcheck, but the main and local compose files do not, which can delay detection of unhealthy workspaces.

### Recommendation

Add the same healthcheck used in the learner template or document why it's intentionally omitted.

---

## Resolution Tracking

| Issue | Decision | Resolved In | Date |
|-------|----------|-------------|------|
| #1 Instructor Auth | | | |
| #2 TURN Server | | | |
| #3 Playwright/Neko | | | |
| #4 Token Revocation | | | |
| #5 Network Isolation | | | |
| #6 DNS-01 Config | | | |
| #7 Learner Name | | | |
| #8 Odehash UI | | | |
| #9 Terminology | | | |
| #10 Workspace Auth Default | | | |
| #11 Neko Credentials | | | |
| #12 Neko SYS_ADMIN | | | |
| #13 Workspace Image Pinning | | | |
| #14 Healthchecks | | | |

---

## Notes

- Issues should be resolved before implementation begins
- Update the Resolution Tracking table as decisions are made
- Reference issue numbers in commits that address them
