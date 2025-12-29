# ClaraTeach - Architecture Overview

## System Architecture

ClaraTeach follows a two-stack architecture separating the persistent Admin Stack from the transient Workspace Stack.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              INTERNET                                       │
└─────────────────────────────────┬───────────────────────────────────────────┘
                                  │
                ┌─────────────────┴─────────────────┐
                │                                   │
                ▼                                   ▼
┌───────────────────────────────────┐ ┌───────────────────────────────────────┐
│         ADMIN STACK               │ │           WORKSPACE STACK             │
│     (Always Running)              │ │         (Per Workshop)                │
│                                   │ │                                       │
│  ┌─────────────────────────────┐  │ │  ┌─────────────────────────────────┐  │
│  │       Cloud Run             │  │ │  │     Compute Engine VM           │  │
│  │                             │  │ │  │                                 │  │
│  │  ┌───────────────────────┐  │  │ │  │  ┌─────────────────────────┐    │  │
│  │  │     Portal API        │  │  │ │  │  │   Caddy (Proxy + SSL)   │    │  │
│  │  │                       │  │  │ │  │  └───────────┬─────────────┘    │  │
│  │  │  - Workshop CRUD      │  │  │ │  │              │                  │  │
│  │  │  - Session mgmt       │  │  │ │  │  ┌───────────┴─────────────┐    │  │
│  │  │  - VM provisioning    │  │  │ │  │  │                         │    │  │
│  │  └───────────────────────┘  │  │ │  │  ▼                         ▼    │  │
│  │                             │  │ │  │  ┌─────┐ ┌─────┐     ┌─────┐    │  │
│  │  ┌───────────────────────┐  │  │ │  │  │ C-1 │ │ C-2 │ ... │C-10 │    │  │
│  │  │     Static Assets     │  │  │ │  │  │     │ │     │     │     │    │  │
│  │  │     (React SPA)       │  │  │ │  │  └─────┘ └─────┘     └─────┘    │  │
│  │  └───────────────────────┘  │  │ │  │  Learner Containers             │  │
│  └─────────────────────────────┘  │ │  │                                 │  │
│                                   │ │  │  ┌─────────────────────────┐    │  │
│  ┌─────────────────────────────┐  │ │  │  │   neko (Browser)        │    │  │
│  │     Secret Manager         │  │ │  │  │   Per container         │    │  │
│  │     - API Keys             │  │ │  │  └─────────────────────────┘    │  │
│  │     - JWT Signing Key      │  │ │  └─────────────────────────────────┘  │
│  └─────────────────────────────┘  │ │                                       │
│                                   │ │  Lifecycle: Created → Running → Destroyed
│  Cost: ~$10/month                 │ │  Cost: ~$0.50/hour while running      │
└───────────────────────────────────┘ └───────────────────────────────────────┘
```

---

## Component Details

### Admin Stack

The Admin Stack runs continuously and handles all control plane operations.

#### Portal API (Cloud Run)

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/workshops` | GET | List instructor's workshops |
| `/api/workshops` | POST | Create new workshop |
| `/api/workshops/:id` | GET | Get workshop details |
| `/api/workshops/:id/start` | POST | Provision workspace VM |
| `/api/workshops/:id/stop` | POST | Destroy workspace VM |
| `/api/join` | POST | Learner joins workshop |
| `/api/health` | GET | Health check |

#### Static Assets

The React SPA is served from Cloud Run or a CDN:
- Join page (`/join`)
- Workspace page (`/workspace`)
- Admin dashboard (`/admin`)

#### Secret Manager

Stores sensitive data:
- Instructor API keys (encrypted)
- JWT signing private key
- GCP service account credentials

---

### Workspace Stack

The Workspace Stack is ephemeral—created when a workshop starts and destroyed when it ends.

#### Compute Engine VM

- **Machine Type**: `e2-standard-8` (8 vCPU, 32GB RAM) for 10 learners
- **Disk**: 100GB SSD
- **OS**: Ubuntu 22.04 with Docker
- **Pricing**: Spot/preemptible for cost savings

#### Caddy (Reverse Proxy)

Handles routing and SSL:

- Terminates TLS with per-workshop certificates (DNS-01)
- Proxies HTTP/WebSocket signaling; WebRTC media flows directly

```
/vm/01/terminal  →  container-01:3001
/vm/01/files     →  container-01:3002
/vm/01/browser   →  container-01:3003
/vm/02/terminal  →  container-02:3001
...
```

#### Learner Containers

Each learner gets an isolated Docker container:

```
┌─────────────────────────────────────────────────────────────────┐
│                    Learner Container                            │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  Workspace Server (Node.js/Python)                      │    │
│  │                                                         │    │
│  │  :3001 - Terminal WebSocket (PTY)                       │    │
│  │  :3002 - File Server (REST API)                         │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  tmux session                                           │    │
│  │  └── bash                                               │    │
│  │      └── claude (CLI)                                   │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  /workspace (learner files)                             │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                 │
│  Environment:                                                   │
│  - CLAUDE_API_KEY=sk-ant-xxx                                   │
│  - WORKSPACE_DIR=/workspace                                    │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

#### neko (Browser Streaming)

Each container optionally has a paired neko instance for browser preview:

```
┌─────────────────────────────────────────────────────────────────┐
│                    neko Container                               │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  Firefox/Chromium                                       │    │
│  │  - Controlled by Playwright in learner container        │    │
│  │  - Viewport streamed via WebRTC                         │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                 │
│  :3003 - WebRTC signaling + streaming                          │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

Notes:
- WebRTC media can require TURN for restrictive networks
- Signaling can be proxied; media typically uses UDP directly

---

## Data Flow

### State Management (No Database)

ClaraTeach MVP operates without a traditional database by leveraging:

1. **GCP Resources as State**
   - Workshop metadata stored in VM labels/metadata
   - Workshop status derived from VM state (RUNNING/TERMINATED)

2. **JWT Tokens as Session State**
   - Self-contained tokens include all session info
   - No server-side session storage needed

3. **VM Metadata for Seat Assignments**
   - JSON blob stored in VM metadata
   - Maps odehash → seat number

4. **Instructor Ownership**
   - Instructor id stored as a VM label
   - Admin list queries filter by label to avoid cross-tenant leakage

```
┌─────────────────────────────────────────────────────────────────┐
│                    State Storage                                │
│                                                                 │
│  GCP Compute Engine VM                                          │
│  ├── name: clarateach-ws-abc                                   │
│  ├── labels:                                                    │
│  │   ├── type: clarateach-workshop                             │
│  │   ├── owner: inst-123                                       │
│  │   ├── code: claude-2024                                     │
│  │   └── seats: 10                                              │
│  ├── metadata:                                                  │
│  │   ├── workshop-name: "Claude CLI Basics"                    │
│  │   ├── api-key-secret: projects/xxx/secrets/yyy              │
│  │   └── seats-map: {"x7k2m": 1, "b3n9p": 2}                  │
│  └── status: RUNNING                                            │
│                                                                 │
│  JWT Token (in learner's browser)                               │
│  {                                                              │
│    "workshopId": "ws-abc",                                      │
│    "containerId": "c-01",                                       │
│    "seat": 1,                                                   │
│    "odehash": "x7k2m",                                          │
│    "vmIp": "34.56.78.90",                                       │
│    "exp": 1704067200                                            │
│  }                                                              │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Seat Allocation and Concurrency (Recommended)

- Join requests are handled by the Portal API only.
- Portal API updates `seats-map` using the GCE metadata fingerprint (CAS). On conflict, re-read and retry.
- If contention or auditability becomes a problem, move seat state to a small store (e.g., Firestore/Redis).
- MVP decision: no per-person seat limit beyond `odehash` idempotency. Preventing multi-seat joins is deferred to post-MVP (e.g., single-use join tickets or identity-based limits).

---

## Network Architecture

### DNS Configuration

```
┌─────────────────────────────────────────────────────────────────┐
│                    Cloud DNS                                    │
│                                                                 │
│  STATIC RECORDS (always exist):                                │
│  ───────────────────────────────                                │
│  portal.clarateach.io    A    → Cloud Run IP                   │
│  api.clarateach.io       A    → Cloud Run IP                   │
│                                                                 │
│  DYNAMIC RECORDS (per workshop):                               │
│  ─────────────────────────────────                              │
│  ws-abc.clarateach.io    A    → 34.56.78.90 (VM IP)            │
│  ws-def.clarateach.io    A    → 34.56.78.91 (VM IP)            │
│                                                                 │
│  CERT ISSUANCE (DNS-01):                                       │
│  ─────────────────────                                         │
│  _acme-challenge.ws-abc  TXT  → token                          │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### TLS Strategy (Recommended)

- Use per-workshop certificates (`ws-abc.clarateach.io`) via ACME DNS-01.
- Caddy requests certs at boot using a restricted DNS service account.
- Avoid distributing a wildcard private key to workspace VMs.

### Firewall Rules

```
┌─────────────────────────────────────────────────────────────────┐
│                    VPC Firewall                                 │
│                                                                 │
│  INGRESS (to Workspace VM):                                    │
│  ─────────────────────────────                                  │
│  - 80/tcp   (HTTP → redirects to HTTPS)                        │
│  - 443/tcp  (HTTPS)                                            │
│  - 22/tcp   (SSH, for debugging only)                          │
│  - 3478/udp (TURN, if using WebRTC)                            │
│  - 5349/tcp (TURN over TLS, if using WebRTC)                   │
│  - 443/udp  (WebRTC media, if required)                        │
│                                                                 │
│  EGRESS (from Workspace VM):                                   │
│  ────────────────────────────                                   │
│  - All allowed (for Claude API, npm, pip)                      │
│                                                                 │
│  CONTAINER ISOLATION:                                          │
│  ─────────────────────                                          │
│  - Containers cannot communicate with each other               │
│  - Docker network per learner container                        │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## Security Model

### Authentication Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                    Authentication                               │
│                                                                 │
│  1. INSTRUCTOR AUTH                                            │
│     - Future: Google OAuth / SSO                               │
│     - MVP: Simple password or magic link                       │
│                                                                 │
│  2. LEARNER AUTH                                               │
│     - Workshop code grants access                              │
│     - No individual learner accounts                           │
│     - JWT token for session                                    │
│                                                                 │
│  3. TOKEN VALIDATION                                           │
│     Admin Stack:                                                │
│     - Signs tokens with RS256 private key                      │
│                                                                 │
│     Workspace Stack:                                            │
│     - Validates with cached JWKS (no per-request network call) │
│     - JWKS refreshed periodically                               │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Token Lifecycle and Key Rotation

- Keep JWT TTL short (e.g., 1-2 hours) and reissue on reconnect.
- Use a JWKS with `kid` for key rotation.
- Maintain a small revocation list for removed learners (in Portal API or metadata).

### Container Security

```yaml
# Docker security configuration
security_opt:
  - no-new-privileges:true
cap_drop:
  - ALL
cap_add:
  - CHOWN
  - SETUID
  - SETGID
read_only: true
tmpfs:
  - /tmp:size=100M
  - /run:size=10M
volumes:
  - workspace-data:/workspace  # Only writable mount
```

Network isolation:
- One Docker network per learner container
- Default deny inter-container traffic; allow only Caddy → container

---

## Scaling Considerations

### Single VM Limits

| Learners | Machine Type | RAM | CPU | Est. Cost/hr |
|----------|--------------|-----|-----|--------------|
| 10 | e2-standard-8 | 32GB | 8 | $0.27 |
| 15 | e2-standard-16 | 64GB | 16 | $0.54 |
| 20 | e2-highmem-16 | 128GB | 16 | $0.74 |

### Multi-VM (Future)

For workshops > 20 learners, provision multiple VMs:

```
Workshop: 50 learners
├── VM-1: Learners 1-15
├── VM-2: Learners 16-30
├── VM-3: Learners 31-45
└── VM-4: Learners 46-50

Session manager routes to correct VM based on seat assignment.
```

---

## Disaster Recovery

### VM Preemption (Spot Instances)

If using spot instances, VMs can be preempted:

1. GCP gives 30-second warning
2. Workspace server notifies connected learners
3. State is lost (containers destroyed)
4. Instructor must re-provision

**Mitigation**: Use standard (non-spot) instances for critical workshops.

### Reconnection After Network Issues

1. Learner loses connection
2. Container keeps running
3. tmux session persists
4. Learner rejoins with odehash
5. Reconnects to same container + tmux session
6. All history preserved

---

## Monitoring & Logging

### Cloud Logging

All components log to Cloud Logging:

```
clarateach-portal     → Portal API logs
clarateach-ws-abc     → Workspace VM logs
```

### Key Metrics

| Metric | Source | Alert Threshold |
|--------|--------|-----------------|
| VM CPU usage | Cloud Monitoring | > 80% |
| Container count | Docker API | != expected seats |
| WebSocket connections | Portal API | > max_seats |
| API latency | Cloud Run | > 1000ms |

---

## Cost Breakdown

### Always-On (Admin Stack)

| Service | Spec | Monthly Cost |
|---------|------|--------------|
| Cloud Run | 1 vCPU, 512MB, min 0 instances | ~$5 |
| Secret Manager | 3 secrets | ~$0.50 |
| Cloud DNS | 1 zone | ~$0.50 |
| Cloud Logging | 1GB | Free |
| **Total** | | **~$6/month** |

### Per Workshop (Workspace Stack)

| Service | Spec | Hourly Cost |
|---------|------|-------------|
| Compute Engine (spot) | e2-standard-8 | ~$0.10 |
| Compute Engine (standard) | e2-standard-8 | ~$0.27 |
| Egress | ~5GB per workshop | ~$0.50 total |

**4-hour workshop with 10 learners**: ~$1-2

---

## Technology Choices

| Component | Technology | Rationale |
|-----------|------------|-----------|
| Admin API | Node.js (Fastify) or Python (FastAPI) | Fast, good WebSocket support |
| Frontend | React + Vite + TypeScript | Modern, fast dev experience |
| Terminal | xterm.js + node-pty | Industry standard |
| Editor | Monaco Editor | VS Code's editor, feature-rich |
| Browser Streaming | neko | Purpose-built, WebRTC-based |
| Reverse Proxy | Caddy | Auto-HTTPS, simple config |
| IaC | Terraform | Industry standard, GCP support |
| Image Building | Packer or Dockerfile | Reproducible images |
| Container Runtime | Docker | Ubiquitous, well-documented |
