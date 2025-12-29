# ClaraTeach - Implementation Plan

This document outlines the phased implementation plan for ClaraTeach MVP.

---

## Overview

The implementation is divided into 4 phases, with each phase building on the previous:

| Phase | Focus | Deliverable |
|-------|-------|-------------|
| 1 | Local Development | Working Docker container with terminal, editor, browser |
| 2 | Workspace Server | Container orchestration, WebSocket terminal, file API |
| 3 | Portal API | Workshop CRUD, session management, GCP integration |
| 4 | Frontend & Integration | React SPA, end-to-end flows |

---

## Phase 1: Local Development Environment

**Goal:** Create a single Docker container that runs locally with the full learner workspace experience.

### 1.1 Base Container Image

Create a Dockerfile for the learner workspace:

```
workspace/
├── Dockerfile
├── docker-compose.yml
├── scripts/
│   ├── entrypoint.sh
│   └── setup-claude.sh
└── config/
    └── tmux.conf
```

**Container includes:**
- Ubuntu 22.04 base
- Python 3.11 + pip
- Node.js 20 LTS + npm
- Claude CLI (installed via npm)
- tmux (session persistence)
- Basic dev tools (git, curl, jq)

**Acceptance Criteria:**
- [ ] Container builds successfully
- [ ] Claude CLI responds to `claude --version`
- [ ] tmux session persists across container restarts
- [ ] `/workspace` directory is writable

### 1.2 Workspace Server (Minimal)

Add a minimal workspace server to the container:

```
workspace/
├── server/
│   ├── package.json
│   ├── tsconfig.json
│   └── src/
│       ├── index.ts
│       ├── terminal.ts      # WebSocket PTY
│       └── files.ts         # REST file API
```

**Endpoints:**
- `WS /terminal` - WebSocket terminal (xterm.js compatible)
- `GET /files?path=` - List directory
- `GET /files/:path` - Read file
- `PUT /files/:path` - Write file
- `DELETE /files/:path` - Delete file

**Acceptance Criteria:**
- [ ] Terminal WebSocket connects and echoes input
- [ ] Can list files in `/workspace`
- [ ] Can create, read, update, delete files via API
- [ ] PTY resizing works

### 1.3 Browser Preview (neko)

Add neko container for browser streaming:

```yaml
# docker-compose.yml additions
services:
  neko:
    image: m1k1o/neko:firefox
    environment:
      NEKO_SCREEN: 1280x720@30
      NEKO_PASSWORD: neko
      NEKO_PASSWORD_ADMIN: admin
    ports:
      - "8080:8080"
      - "52000-52100:52000-52100/udp"
```

**Acceptance Criteria:**
- [ ] neko UI accessible at localhost:8080
- [ ] Browser stream visible via WebRTC
- [ ] Can navigate to URLs in the browser

### 1.4 Local Frontend Shell

Create a minimal frontend to test the workspace:

```
frontend/
├── package.json
├── vite.config.ts
├── src/
│   ├── App.tsx
│   ├── components/
│   │   ├── Terminal.tsx      # xterm.js wrapper
│   │   ├── Editor.tsx        # Monaco editor
│   │   └── Browser.tsx       # neko embed
│   └── hooks/
│       └── useWebSocket.ts
```

**Acceptance Criteria:**
- [ ] Three-panel layout renders
- [ ] Terminal connects to workspace server
- [ ] Editor loads and saves files
- [ ] Browser panel shows neko stream

---

## Phase 2: Workspace Server (Full)

**Goal:** Production-ready workspace server with authentication and multi-container support.

### 2.1 JWT Authentication

Add token validation to workspace server:

```typescript
// middleware/auth.ts
- Validate RS256 JWT
- Extract seat, workshopId, containerId
- Reject expired tokens
- (MVP) No revocation check
```

**Acceptance Criteria:**
- [ ] Requests without token return 401
- [ ] Invalid tokens return 401
- [ ] Valid tokens grant access
- [ ] Token claims available in request context

### 2.2 Container Orchestration

Scripts to manage multiple learner containers:

```
workspace/
├── orchestrator/
│   ├── create-container.sh
│   ├── destroy-container.sh
│   ├── list-containers.sh
│   └── container-template/
│       └── docker-compose.learner.yml
```

**Container naming:** `clarateach-{workshop-id}-{seat}`

**Acceptance Criteria:**
- [ ] Can create container for seat N
- [ ] Can destroy container by seat
- [ ] Can list all containers for a workshop
- [ ] Containers are isolated (no cross-container access)

### 2.3 Caddy Reverse Proxy

Configure Caddy for routing:

```
# Caddyfile
{workshop-id}.clarateach.io {
    # Terminal WebSocket
    handle /vm/{seat}/terminal {
        reverse_proxy clarateach-{workshop-id}-{seat}:3001
    }

    # File API
    handle /vm/{seat}/files/* {
        reverse_proxy clarateach-{workshop-id}-{seat}:3002
    }

    # Browser (neko)
    handle /vm/{seat}/browser/* {
        reverse_proxy clarateach-{workshop-id}-{seat}-neko:8080
    }
}
```

**Acceptance Criteria:**
- [ ] Routes resolve to correct container
- [ ] WebSocket upgrade works through proxy
- [ ] TLS terminates at Caddy (local: self-signed)

### 2.4 JWKS Endpoint

Workspace VMs need to validate tokens without calling Portal API:

```
# Cached JWKS on workspace VM
/etc/clarateach/jwks.json
```

**Bootstrap process:**
1. VM starts
2. Fetches JWKS from Portal API (or Secret Manager)
3. Caches locally
4. Refreshes every 6 hours

**Acceptance Criteria:**
- [ ] JWKS file exists on VM
- [ ] Workspace server validates tokens using cached JWKS
- [ ] Token validation works offline

---

## Phase 3: Portal API

**Goal:** Admin API for workshop management and GCP integration.

### 3.1 API Scaffold

```
portal/
├── package.json
├── tsconfig.json
├── src/
│   ├── index.ts
│   ├── routes/
│   │   ├── health.ts
│   │   ├── workshops.ts
│   │   └── join.ts
│   ├── services/
│   │   ├── gcp.ts           # GCE operations
│   │   ├── jwt.ts           # Token signing
│   │   └── workshop.ts      # Business logic
│   └── middleware/
│       └── auth.ts          # (MVP: no-op or simple password)
```

**Acceptance Criteria:**
- [ ] `GET /api/health` returns 200
- [ ] API runs in Docker container
- [ ] Environment variables configured

### 3.2 Workshop CRUD

Implement workshop endpoints:

| Endpoint | Action |
|----------|--------|
| `POST /api/workshops` | Create workshop (stores in memory/VM labels) |
| `GET /api/workshops` | List workshops |
| `GET /api/workshops/:id` | Get workshop details |
| `DELETE /api/workshops/:id` | Delete workshop |

**State storage (MVP):** In-memory Map (lost on restart)

**Acceptance Criteria:**
- [ ] Can create workshop with name, seats, API key
- [ ] Workshop code auto-generated
- [ ] Can list and retrieve workshops
- [ ] Can delete workshop

### 3.3 GCP Integration

Implement VM provisioning:

```typescript
// services/gcp.ts
class GCPService {
  async createVM(workshop: Workshop): Promise<string>
  async deleteVM(vmName: string): Promise<void>
  async getVM(vmName: string): Promise<VMInfo>
  async setMetadata(vmName: string, key: string, value: string): Promise<void>
}
```

**VM configuration:**
- Machine type: e2-standard-8
- Image: Custom image with Docker + containers
- Labels: type, code, seats, owner
- Metadata: workshop-name, seats-map, api-key-secret

**Acceptance Criteria:**
- [ ] `POST /workshops/:id/start` creates GCE VM
- [ ] `POST /workshops/:id/stop` deletes GCE VM
- [ ] VM labels and metadata set correctly
- [ ] VM IP returned when running

### 3.4 Session Management (Join Flow)

Implement learner join:

```typescript
// POST /api/join
// Request: { code, name, odehash? }
// Response: { token, endpoint, odehash, seat }

1. Find workshop by code
2. If odehash provided:
   - Look up existing seat
   - Return same seat
3. Else:
   - Allocate new seat (CAS on metadata)
   - Generate odehash
4. Sign JWT with claims
5. Return response
```

**Acceptance Criteria:**
- [ ] Join with valid code returns token
- [ ] Join with invalid code returns 404
- [ ] Reconnect with odehash returns same seat
- [ ] Full workshop returns NO_SEATS error
- [ ] JWT contains correct claims

### 3.5 DNS Record Management

Automate DNS record creation:

```typescript
// services/dns.ts
class DNSService {
  async createRecord(workshopId: string, ip: string): Promise<void>
  async deleteRecord(workshopId: string): Promise<void>
}
```

**Acceptance Criteria:**
- [ ] Starting workshop creates DNS A record
- [ ] Stopping workshop deletes DNS record
- [ ] Records created in correct zone

---

## Phase 4: Frontend & Integration

**Goal:** Complete React SPA with all user flows.

### 4.1 Landing Page & Navigation

Implement from wireframes:

```
src/
├── pages/
│   ├── LandingPage.tsx
│   ├── TeacherDashboard.tsx
│   ├── TeacherClassView.tsx
│   ├── LearnerJoin.tsx
│   └── Workspace.tsx
├── components/
│   └── (from wireframes)
└── lib/
    ├── api.ts            # API client
    └── auth.ts           # Token storage
```

**Acceptance Criteria:**
- [ ] Landing page with role selection
- [ ] Navigation between pages
- [ ] Responsive layout

### 4.2 Teacher Dashboard

- Create workshop form
- List active workshops
- Workshop status polling

**Acceptance Criteria:**
- [ ] Can create new workshop
- [ ] Active workshops displayed with codes
- [ ] Status updates when VM provisioning completes

### 4.3 Teacher Class View

- Display join code
- List connected learners
- End class button

**Acceptance Criteria:**
- [ ] Join code prominently displayed
- [ ] Learner list updates in real-time (polling)
- [ ] End class stops workshop

### 4.4 Learner Join Flow

- Enter code + name
- Handle errors (invalid code, full workshop)
- Store odehash in localStorage

**Acceptance Criteria:**
- [ ] Join with valid code navigates to workspace
- [ ] Error messages for invalid/full
- [ ] Odehash stored for reconnection

### 4.5 Workspace Integration

Connect all workspace components:

- Terminal (xterm.js → WebSocket)
- Editor (Monaco → File API)
- Browser (neko embed)

**Acceptance Criteria:**
- [ ] Terminal connects and works
- [ ] File tree loads from API
- [ ] Editor saves to API
- [ ] Browser preview renders

### 4.6 Reconnection Flow

Handle disconnection gracefully:

- Detect WebSocket disconnect
- Show reconnection UI
- Auto-reconnect with odehash

**Acceptance Criteria:**
- [ ] Disconnect shows banner/modal
- [ ] Can reconnect using stored odehash
- [ ] Terminal history preserved (tmux)

---

## Phase 5: Production Readiness

**Goal:** Deploy to GCP and prepare for real workshops.

### 5.1 Infrastructure as Code

```
infra/
├── terraform/
│   ├── main.tf
│   ├── variables.tf
│   ├── outputs.tf
│   ├── modules/
│   │   ├── cloud-run/
│   │   ├── dns/
│   │   └── secrets/
│   └── environments/
│       ├── dev/
│       └── prod/
```

**Acceptance Criteria:**
- [ ] `terraform apply` creates all resources
- [ ] Cloud Run deploys Portal API
- [ ] Secrets stored in Secret Manager
- [ ] DNS zone configured

### 5.2 VM Image (Packer)

Build custom GCE image:

```
packer/
├── workspace.pkr.hcl
├── scripts/
│   ├── install-docker.sh
│   ├── install-caddy.sh
│   └── install-workspace.sh
└── files/
    ├── Caddyfile
    └── docker-compose.base.yml
```

**Acceptance Criteria:**
- [ ] Image builds successfully
- [ ] Image boots and runs containers
- [ ] Caddy serves HTTPS
- [ ] Containers start automatically

### 5.3 Deployment Pipeline

```yaml
# .github/workflows/deploy.yml
- Build Portal API container
- Push to Artifact Registry
- Deploy to Cloud Run
- (Optional) Build workspace image
```

**Acceptance Criteria:**
- [ ] Push to main triggers deployment
- [ ] Portal API updates without downtime
- [ ] Rollback possible

### 5.4 Monitoring & Alerting

- Cloud Logging structured logs
- Cloud Monitoring dashboards
- Alerts for VM failures, high CPU

**Acceptance Criteria:**
- [ ] Logs visible in Cloud Console
- [ ] Dashboard shows key metrics
- [ ] Alerts fire on threshold breach

---

## Open Issues to Resolve

Before implementation begins, resolve these from `ISSUES.md`:

| Issue | Decision Needed |
|-------|-----------------|
| #1 Instructor Auth | Open endpoints for MVP or simple password? |
| #2 TURN Server | Skip for MVP or include coturn? |
| #3 Playwright/Neko | Defer Playwright for MVP? |
| #5 Network Isolation | Shared network for MVP? |
| #6 DNS-01 Config | Domain and zone details? |

---

## Implementation Order

```
Week 1-2: Phase 1 (Local Dev)
├── Day 1-2: Base container
├── Day 3-4: Workspace server
├── Day 5-6: neko integration
└── Day 7-8: Local frontend

Week 3-4: Phase 2 (Workspace Server)
├── Day 1-2: JWT auth
├── Day 3-4: Container orchestration
├── Day 5-6: Caddy proxy
└── Day 7-8: Testing & fixes

Week 5-6: Phase 3 (Portal API)
├── Day 1-2: API scaffold
├── Day 3-4: Workshop CRUD
├── Day 5-6: GCP integration
└── Day 7-8: Session management

Week 7-8: Phase 4 (Frontend)
├── Day 1-2: Pages & navigation
├── Day 3-4: Teacher flows
├── Day 5-6: Learner flows
└── Day 7-8: Workspace integration

Week 9-10: Phase 5 (Production)
├── Day 1-3: Terraform
├── Day 4-5: Packer image
├── Day 6-7: CI/CD
└── Day 8-10: Testing & launch
```

---

## Success Criteria (MVP)

The MVP is complete when:

1. **Instructor can:**
   - Create a workshop with name and seat count
   - See generated workshop code
   - Start workshop (provisions VM)
   - View connected learners
   - Stop workshop (destroys VM)

2. **Learner can:**
   - Join with workshop code + name
   - Access terminal with Claude CLI
   - Edit files in workspace
   - View browser preview
   - Reconnect after disconnect

3. **Infrastructure:**
   - Portal API runs on Cloud Run
   - Workspace VMs provision on demand
   - TLS works for all connections
   - Cost < $2 per 4-hour workshop

---

## Tech Stack Summary

| Layer | Technology |
|-------|------------|
| Frontend | React 18, Vite, TypeScript, Tailwind |
| Terminal | xterm.js, node-pty |
| Editor | Monaco Editor |
| Browser | neko (Firefox) |
| Workspace Server | Node.js (Fastify), TypeScript |
| Portal API | Node.js (Fastify), TypeScript |
| Proxy | Caddy |
| Container | Docker, Docker Compose |
| Cloud | GCP (Cloud Run, GCE, DNS, Secrets) |
| IaC | Terraform, Packer |
| CI/CD | GitHub Actions |
