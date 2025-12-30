# ClaraTeach Local Development Networking Analysis

**Date:** 2025-12-30
**Issue:** Frontend cannot connect to workspace containers on macOS

---

## Executive Summary

The Go backend's debug proxy cannot reach Docker container IPs on macOS because Docker runs inside a Linux VM, and container networks are not routable from the macOS host. This document analyzes the problem and presents solution options with clear migration paths to production.

---

## 1. Production Architecture (Target State)

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                                    INTERNET                                          │
└─────────────────────────────────────────┬───────────────────────────────────────────┘
                                          │
                    ┌─────────────────────┴─────────────────────┐
                    │                                           │
                    ▼                                           ▼
┌───────────────────────────────────────┐    ┌─────────────────────────────────────────┐
│           ADMIN STACK                 │    │          WORKSPACE STACKS               │
│         (Cloud Run - Always On)       │    │      (GCE VMs - Per Workshop)           │
│                                       │    │                                         │
│  ┌─────────────────────────────────┐  │    │  ┌─────────────────────────────────┐    │
│  │         Portal API              │  │    │  │      WORKSHOP "ABC"             │    │
│  │                                 │  │    │  │      ws-abc.clarateach.io       │    │
│  │  • Workshop CRUD                │  │    │  │      (GCE VM: 34.56.78.90)      │    │
│  │  • Provisions GCE VMs           │  │    │  │                                 │    │
│  │  • Creates DNS records          │  │    │  │  ┌─────────────────────────┐    │    │
│  │  • Issues JWT tokens            │  │    │  │  │       Caddy             │    │    │
│  │  • Returns workspace endpoint   │  │    │  │  │   (Reverse Proxy + TLS) │    │    │
│  │                                 │  │    │  │  └───────────┬─────────────┘    │    │
│  └─────────────────────────────────┘  │    │  │              │                  │    │
│                                       │    │  │   ┌──────────┼──────────┐       │    │
│  ┌─────────────────────────────────┐  │    │  │   ▼          ▼          ▼       │    │
│  │      Frontend (React SPA)       │  │    │  │ ┌────┐    ┌────┐    ┌────┐     │    │
│  │      portal.clarateach.io       │  │    │  │ │C-1 │    │C-2 │    │C-10│     │    │
│  └─────────────────────────────────┘  │    │  │ └────┘    └────┘    └────┘     │    │
│                                       │    │  └─────────────────────────────────┘    │
│  ┌─────────────────────────────────┐  │    │                                         │
│  │      Secret Manager             │  │    │  ┌─────────────────────────────────┐    │
│  │  • JWT signing keys             │  │    │  │      WORKSHOP "DEF"             │    │
│  │  • API keys                     │  │    │  │      ws-def.clarateach.io       │    │
│  └─────────────────────────────────┘  │    │  │      (GCE VM: 34.56.78.91)      │    │
│                                       │    │  │           ...                   │    │
│  Cost: ~$6/month                      │    │  └─────────────────────────────────┘    │
└───────────────────────────────────────┘    │                                         │
                                             │  Cost: ~$0.27/hour per workshop         │
                                             └─────────────────────────────────────────┘

PRODUCTION DATA FLOW:
━━━━━━━━━━━━━━━━━━━━━
1. Instructor creates workshop via Portal API
2. Portal provisions GCE VM, creates DNS record (ws-abc.clarateach.io → VM IP)
3. Portal returns endpoint to frontend: "https://ws-abc.clarateach.io"
4. Learner joins, gets JWT token + endpoint
5. Frontend connects DIRECTLY to workspace VM (not through Portal)
6. Caddy on VM handles TLS, routes to containers
```

**Key Production Characteristics:**
- Portal API **never proxies** workspace traffic
- Each workshop is a **completely independent VM**
- Frontend connects **directly** to workshop VMs
- Caddy runs **inside each VM**, routes to local containers
- Containers communicate via **Docker bridge network** (172.x.x.x)

---

## 2. The Local Development Problem

### Current Broken Setup

```
┌──────────────────────────────────────────────────────────────────────────────────────┐
│                              macOS HOST                                               │
│                                                                                      │
│  ┌─────────────────────┐                                                             │
│  │  Frontend (Vite)    │                                                             │
│  │  localhost:5173     │                                                             │
│  └──────────┬──────────┘                                                             │
│             │                                                                        │
│             │  /api/* proxied by Vite                                                │
│             ▼                                                                        │
│  ┌─────────────────────┐         ┌─────────────────────────────────────────────────┐ │
│  │  Go Backend         │         │  Docker Desktop (Linux VM hidden from macOS)    │ │
│  │  localhost:8080     │         │                                                 │ │
│  │                     │         │  ┌───────────────────────────────────────────┐  │ │
│  │  /api/join returns: │         │  │  Docker Network: 172.19.0.0/16            │  │ │
│  │  endpoint =         │         │  │  (NOT ROUTABLE FROM macOS!)               │  │ │
│  │  localhost:8080/    │         │  │                                           │  │ │
│  │  debug/proxy/ws-xxx │         │  │  ┌──────────────┐  ┌──────────────┐       │  │ │
│  │                     │    ❌    │  │  │  Container   │  │  Container   │       │  │ │
│  │  /debug/proxy/* ────┼────X────┼──┼─▶│  172.19.0.2  │  │  172.19.0.3  │       │  │ │
│  │  tries to reach     │         │  │  │  :3001/:3002 │  │  :3001/:3002 │       │  │ │
│  │  172.19.0.2:3001    │         │  │  └──────────────┘  └──────────────┘       │  │ │
│  │                     │         │  │                                           │  │ │
│  └─────────────────────┘         │  └───────────────────────────────────────────┘  │ │
│                                  └─────────────────────────────────────────────────┘ │
│                                                                                      │
│  ❌ FAILURE: Go backend on macOS cannot route to Docker's 172.x.x.x network         │
└──────────────────────────────────────────────────────────────────────────────────────┘
```

### Root Cause

| Platform | Docker Implementation | Container IPs Routable from Host? |
|----------|----------------------|----------------------------------|
| **Linux** | Native containers | ✅ Yes |
| **macOS** | Containers in Linux VM | ❌ No |
| **Windows** | Containers in WSL2/Hyper-V | ❌ No |

---

## 3. Solution Options

### Option A: Run Go Backend Inside Docker

**Concept:** Put the Go backend in the same Docker network as workspace containers.

```
┌──────────────────────────────────────────────────────────────────────────────────────┐
│                              macOS HOST                                               │
│                                                                                      │
│  ┌─────────────────────┐                                                             │
│  │  Frontend (Vite)    │                                                             │
│  │  localhost:5173     │                                                             │
│  └──────────┬──────────┘                                                             │
│             │                                                                        │
│             │  Connects to localhost:8080                                            │
│             ▼                                                                        │
│  ┌──────────────────────────────────────────────────────────────────────────────────┐│
│  │                     Docker Desktop (Linux VM)                                    ││
│  │                                                                                  ││
│  │  ┌────────────────────────────────────────────────────────────────────────────┐  ││
│  │  │                    Docker Network: clarateach-dev                          │  ││
│  │  │                                                                            │  ││
│  │  │  ┌─────────────────────┐                                                   │  ││
│  │  │  │  Go Backend         │                                                   │  ││
│  │  │  │  172.19.0.10:8080   │◀─── Port 8080 exposed to host                     │  ││
│  │  │  │                     │                                                   │  ││
│  │  │  │  /debug/proxy/* ────┼──────┐                                            │  ││
│  │  │  │                     │      │  ✅ CAN reach container IPs!               │  ││
│  │  │  └─────────────────────┘      │                                            │  ││
│  │  │                               ▼                                            │  ││
│  │  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                      │  ││
│  │  │  │  Container   │  │  Container   │  │  Neko        │                      │  ││
│  │  │  │  172.19.0.2  │  │  172.19.0.3  │  │  172.19.0.4  │                      │  ││
│  │  │  │  :3001/:3002 │  │  :3001/:3002 │  │  :3003       │                      │  ││
│  │  │  └──────────────┘  └──────────────┘  └──────────────┘                      │  ││
│  │  │                                                                            │  ││
│  │  └────────────────────────────────────────────────────────────────────────────┘  ││
│  └──────────────────────────────────────────────────────────────────────────────────┘│
└──────────────────────────────────────────────────────────────────────────────────────┘
```

**Implementation:**

```yaml
# docker-compose.dev.yml
services:
  backend:
    build: ./backend
    ports:
      - "8080:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      - BASE_DOMAIN=localhost
      - DB_PATH=/data/clarateach.db
    networks:
      - clarateach-dev

networks:
  clarateach-dev:
    driver: bridge
```

```go
// backend/internal/orchestrator/docker.go
// Modify Create() to attach new containers to "clarateach-dev" network
```

| Pros | Cons |
|------|------|
| ✅ No code changes to proxy logic | ❌ Need Docker socket mounting |
| ✅ Same networking as production | ❌ Rebuild container on code changes |
| ✅ Works on any platform | ❌ Docker-in-Docker complexity |
| ✅ Tests full flow including dynamic provisioning | ❌ Slightly slower dev iteration |

**Path to Production:**
```
LOCAL DEV                              PRODUCTION
───────────────────────────────────────────────────────────────
Backend in Docker container      →     Backend on Cloud Run
Containers on shared network     →     Containers on VM-local network
/debug/proxy/* route             →     Direct connection to VM
localhost:8080                   →     api.clarateach.io
```
Migration: Remove `/debug/proxy` route, deploy to Cloud Run. **No proxy code changes needed.**

---

### Option B: Port Mapping (Expose Container Ports to Host)

**Concept:** Map container ports to host ports, use `localhost:PORT` instead of container IPs.

```
┌──────────────────────────────────────────────────────────────────────────────────────┐
│                              macOS HOST                                               │
│                                                                                      │
│  ┌─────────────────────┐     ┌─────────────────────┐                                 │
│  │  Frontend (Vite)    │     │  Go Backend         │                                 │
│  │  localhost:5173     │────▶│  localhost:8080     │                                 │
│  └─────────────────────┘     │                     │                                 │
│                              │  /debug/proxy/*     │                                 │
│                              │  routes to:         │                                 │
│                              │  localhost:13011 ───┼──┐                              │
│                              │  localhost:13012 ───┼──┼──┐                           │
│                              │  localhost:13021 ───┼──┼──┼──┐                        │
│                              └─────────────────────┘  │  │  │                        │
│                                                       │  │  │                        │
│  ┌────────────────────────────────────────────────────┼──┼──┼───────────────────────┐│
│  │              Docker Desktop (Linux VM)             │  │  │                       ││
│  │                                                    │  │  │                       ││
│  │  ┌─────────────────────┐  ┌─────────────────────┐  │  │  │                       ││
│  │  │  Container 1        │  │  Container 2        │  │  │  │                       ││
│  │  │  :3001 ─────────────┼──┼──────────────────────  │  │  │ Port mappings:        ││
│  │  │  :3002 ─────────────┼──┼─────────────────────────  │  │ 13011 → C1:3001       ││
│  │  └─────────────────────┘  │  :3001 ───────────────────┼──┘ 13012 → C1:3002       ││
│  │                           │  :3002 ────────────────────    13021 → C2:3001       ││
│  │                           └─────────────────────┘          13022 → C2:3002       ││
│  │                                                                                  ││
│  └──────────────────────────────────────────────────────────────────────────────────┘│
└──────────────────────────────────────────────────────────────────────────────────────┘
```

**Implementation:**

```go
// backend/internal/orchestrator/docker.go

func (d *DockerProvider) Create(ctx context.Context, cfg InstanceConfig) (*Instance, error) {
    // Calculate unique host ports for this seat
    basePort := 13000 + (cfg.SeatID * 10)
    terminalHostPort := basePort + 1
    filesHostPort := basePort + 2
    browserHostPort := basePort + 3

    hostConfig := &container.HostConfig{
        PortBindings: nat.PortMap{
            "3001/tcp": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", terminalHostPort)}},
            "3002/tcp": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", filesHostPort)}},
        },
        // ... rest
    }

    return &Instance{
        ID:               resp.ID,
        IP:               ip,
        HostTerminalPort: terminalHostPort,
        HostFilesPort:    filesHostPort,
        HostBrowserPort:  browserHostPort,
    }, nil
}
```

```go
// backend/internal/proxy/proxy.go

func (p *DynamicProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // ... get workshopID, seatID, targetPath

    instance, err := p.orchestrator.GetInstance(ctx, workshopID, seatID)

    var targetStr string
    if instance.HostTerminalPort > 0 {
        // Local dev: use host port mapping
        if strings.HasPrefix(targetPath, "/terminal") {
            targetStr = fmt.Sprintf("http://localhost:%d", instance.HostTerminalPort)
        } else if strings.HasPrefix(targetPath, "/files") {
            targetStr = fmt.Sprintf("http://localhost:%d", instance.HostFilesPort)
        }
    } else {
        // Production: use container IP directly
        targetStr = fmt.Sprintf("http://%s:%d", instance.IP, port)
    }

    // ... proxy request
}
```

| Pros | Cons |
|------|------|
| ✅ No extra VMs or containers | ❌ Port allocation complexity |
| ✅ Backend runs natively (fast iteration) | ❌ Different code path than production |
| ✅ Works on any platform | ❌ Limited by available ports |
| | ❌ Must track port mappings |

**Path to Production:**
```
LOCAL DEV                              PRODUCTION
───────────────────────────────────────────────────────────────
Backend on macOS host            →     Backend on Cloud Run
Port mappings (localhost:13011)  →     Container IPs (172.x.x.x)
/debug/proxy/* with ports        →     Direct VM connection
Conditional port logic           →     Standard IP routing
```
Migration: The `HostTerminalPort > 0` check handles both cases. Production instances return `HostTerminalPort = 0`, falling back to IP routing.

---

### Option C: Local Linux VM (Production-like Environment)

**Concept:** Run everything inside a Linux VM where Docker networking works natively.

```
┌──────────────────────────────────────────────────────────────────────────────────────┐
│                              macOS HOST                                               │
│                                                                                      │
│  ┌─────────────────────┐                                                             │
│  │  Browser            │                                                             │
│  │  localhost:5173 ────┼────────────────────────────┐                                │
│  │  localhost:8080 ────┼───────────────────────┐    │                                │
│  └─────────────────────┘                       │    │ (Port forwarding)              │
│                                                │    │                                │
│  ┌─────────────────────────────────────────────┼────┼───────────────────────────────┐│
│  │              Linux VM (OrbStack / Lima)     │    │                               ││
│  │                                             │    │                               ││
│  │  ┌─────────────────────┐  ┌─────────────────┼────┼─────────────────────────────┐ ││
│  │  │  Frontend (Vite)    │  │  Go Backend     │    │                             │ ││
│  │  │  0.0.0.0:5173 ◀─────┼──┼─────────────────┼────┘                             │ ││
│  │  └─────────────────────┘  │  0.0.0.0:8080 ◀─┘                                  │ ││
│  │                           │                                                    │ ││
│  │                           │  /debug/proxy/* ───┐  ✅ Native Linux Docker!      │ ││
│  │                           └────────────────────┼───────────────────────────────┘ ││
│  │                                                │                                 ││
│  │  ┌─────────────────────────────────────────────┼───────────────────────────────┐ ││
│  │  │              Docker (Native on Linux)       │                               │ ││
│  │  │                                             ▼                               │ ││
│  │  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                       │ ││
│  │  │  │  Container   │  │  Container   │  │  Neko        │                       │ ││
│  │  │  │  172.19.0.2  │  │  172.19.0.3  │  │  172.19.0.4  │                       │ ││
│  │  │  │  :3001/:3002 │  │  :3001/:3002 │  │  :3003       │                       │ ││
│  │  │  └──────────────┘  └──────────────┘  └──────────────┘                       │ ││
│  │  │                                                                             │ ││
│  │  │  ✅ Backend CAN reach 172.x.x.x directly!                                   │ ││
│  │  └─────────────────────────────────────────────────────────────────────────────┘ ││
│  └──────────────────────────────────────────────────────────────────────────────────┘│
└──────────────────────────────────────────────────────────────────────────────────────┘
```

**Implementation Options:**

| Tool | Setup Command | Notes |
|------|--------------|-------|
| **OrbStack** | `brew install orbstack` | Best UX, fast, $8/mo |
| **Lima** | `brew install lima && limactl start` | Free, good Docker support |
| **Multipass** | `brew install multipass && multipass launch` | Ubuntu official, free |
| **Colima** | `brew install colima && colima start` | Free, Lima-based |

**Setup with Lima:**

```bash
# Install
brew install lima docker

# Create VM with Docker
limactl start --name=clarateach template://docker

# Shell into VM
limactl shell clarateach

# Clone and run
git clone <repo> ~/clarateach
cd ~/clarateach
./scripts/stack.sh start

# From macOS, access via forwarded ports
# or configure lima.yaml for port forwarding
```

| Pros | Cons |
|------|------|
| ✅ **Matches production exactly** | ❌ Extra resource usage (RAM/CPU) |
| ✅ No code changes at all | ❌ Initial setup time |
| ✅ Tests real Linux behavior | ❌ Need to sync code or mount volumes |
| ✅ Same networking as GCE VM | ❌ Slightly more complex workflow |

**Path to Production:**
```
LOCAL DEV                              PRODUCTION
───────────────────────────────────────────────────────────────
Linux VM on macOS                →     GCE VM on GCP
Docker native networking         →     Docker native networking
Backend reaches container IPs    →     Backend reaches container IPs
/debug/proxy/* route             →     Direct VM connection (Caddy)
```
Migration: **Identical code.** Just deploy to GCP instead of local VM. The only difference is the debug proxy route (used for local) vs direct Caddy connection (production).

---

### Option D: Use Existing docker-compose + Caddy Setup

**Concept:** Use the pre-built `docker-compose.local.yml` which already works.

```
┌──────────────────────────────────────────────────────────────────────────────────────┐
│                              macOS HOST                                               │
│                                                                                      │
│  ┌─────────────────────┐                                                             │
│  │  Frontend (Vite)    │                                                             │
│  │  localhost:5173     │                                                             │
│  └──────────┬──────────┘                                                             │
│             │                                                                        │
│             │  /api/* → localhost:8080 (backend)                                     │
│             │  Session endpoint → localhost:8000 (Caddy)                             │
│             │                                                                        │
│  ┌──────────┼───────────────────────────────────────────────────────────────────────┐│
│  │          │           Docker Desktop                                              ││
│  │          │                                                                       ││
│  │          │    ┌─────────────────────────────────────────────────────────────┐    ││
│  │          │    │              Docker Network: clarateach-local               │    ││
│  │          │    │                                                             │    ││
│  │          │    │  ┌───────────────────┐                                      │    ││
│  │          └────┼─▶│  Caddy            │◀─── localhost:8000 exposed           │    ││
│  │               │  │  (Reverse Proxy)  │                                      │    ││
│  │               │  └─────────┬─────────┘                                      │    ││
│  │               │            │                                                │    ││
│  │               │   ┌────────┼────────┬────────┐                              │    ││
│  │               │   ▼        ▼        ▼        ▼                              │    ││
│  │               │ ┌────┐  ┌────┐  ┌────┐  ┌──────┐                            │    ││
│  │               │ │ws-1│  │ws-2│  │ws-3│  │ neko │                            │    ││
│  │               │ └────┘  └────┘  └────┘  └──────┘                            │    ││
│  │               │                                                             │    ││
│  │               └─────────────────────────────────────────────────────────────┘    ││
│  │                                                                                  ││
│  │  ✅ Caddy handles routing inside Docker network                                  ││
│  │  ✅ Frontend connects to localhost:8000 (exposed port)                           ││
│  └──────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                      │
│  ┌─────────────────────┐                                                             │
│  │  Go Backend         │◀─── Runs on macOS host OR skip entirely for UI dev         │
│  │  localhost:8080     │                                                             │
│  │  (Optional for      │                                                             │
│  │   workspace testing)│                                                             │
│  └─────────────────────┘                                                             │
└──────────────────────────────────────────────────────────────────────────────────────┘
```

**Implementation:**

```bash
# Start everything
./scripts/stack.sh start

# This starts:
# - workspace-1, workspace-2, workspace-3 (pre-configured)
# - Caddy on localhost:8000
# - neko browser

# Frontend just needs to know the endpoint
# Modify Join page or set in localStorage:
localStorage.setItem('clarateach_session', JSON.stringify({
  endpoint: 'http://localhost:8000',
  seat: 1,
  // ...
}));
```

**Or modify backend to return Caddy endpoint:**

```go
// backend/internal/api/server.go
func (s *Server) joinWorkshop(w http.ResponseWriter, r *http.Request) {
    // ...

    var endpoint string
    if s.baseDomain == "localhost" {
        // Use Caddy directly, bypass debug proxy
        endpoint = os.Getenv("WORKSPACE_ENDPOINT_OVERRIDE")
        if endpoint == "" {
            endpoint = "http://localhost:8000"
        }
    } else {
        endpoint = fmt.Sprintf("https://%s.%s", workshop.ID, s.baseDomain)
    }

    // ...
}
```

| Pros | Cons |
|------|------|
| ✅ **Already working** | ❌ Doesn't test dynamic provisioning |
| ✅ No code changes needed | ❌ Fixed number of seats (3) |
| ✅ Simplest option | ❌ Skip Go orchestrator entirely |
| ✅ Good for frontend development | ❌ Different code path than production |

**Path to Production:**
```
LOCAL DEV                              PRODUCTION
───────────────────────────────────────────────────────────────
docker-compose pre-built         →     Go orchestrator provisions
Caddy on localhost:8000          →     Caddy on VM (ws-xxx.clarateach.io)
Fixed 3 seats                    →     Dynamic N seats
Endpoint hardcoded               →     Endpoint from Portal API
```
Migration: Need to ensure Go orchestrator works (test separately). Frontend code is the same.

---

## 4. Comparison Matrix

| Criterion | A: Backend in Docker | B: Port Mapping | C: Linux VM | D: Use Caddy |
|-----------|---------------------|-----------------|-------------|--------------|
| **Production Fidelity** | High | Medium | **Highest** | Low |
| **Tests Dynamic Provisioning** | ✅ Yes | ✅ Yes | ✅ Yes | ❌ No |
| **Code Changes Required** | Low | Medium | **None** | Low |
| **Setup Complexity** | Medium | Medium | Medium | **Low** |
| **Dev Iteration Speed** | Medium | **Fast** | Medium | **Fast** |
| **Cross-Platform** | ✅ Yes | ✅ Yes | macOS only | ✅ Yes |
| **Resource Usage** | Low | **Low** | Medium | **Low** |
| **Matches Prod Networking** | ✅ Yes | ❌ No | ✅ Yes | Partial |

---

## 5. Recommended Strategy

### Use Different Approaches for Different Purposes

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                           DEVELOPMENT WORKFLOW                                       │
│                                                                                     │
│  ┌────────────────────────────────────────────────────────────────────────────────┐ │
│  │  FRONTEND DEVELOPMENT                                                          │ │
│  │  ════════════════════                                                          │ │
│  │  Use: Option D (docker-compose + Caddy)                                        │ │
│  │                                                                                │ │
│  │  • Fast iteration on React components                                          │ │
│  │  • Terminal, Editor, Browser UI work                                           │ │
│  │  • No need to run Go backend                                                   │ │
│  │                                                                                │ │
│  │  Command: ./scripts/stack.sh start                                             │ │
│  └────────────────────────────────────────────────────────────────────────────────┘ │
│                                                                                     │
│  ┌────────────────────────────────────────────────────────────────────────────────┐ │
│  │  BACKEND DEVELOPMENT                                                           │ │
│  │  ═══════════════════                                                           │ │
│  │  Use: Option A (Backend in Docker) or Option B (Port Mapping)                  │ │
│  │                                                                                │ │
│  │  • Test workshop creation flow                                                 │ │
│  │  • Test dynamic container provisioning                                         │ │
│  │  • Test join/session management                                                │ │
│  │                                                                                │ │
│  │  Option A if: You want same networking as prod                                 │ │
│  │  Option B if: You want faster iteration (no container rebuild)                 │ │
│  └────────────────────────────────────────────────────────────────────────────────┘ │
│                                                                                     │
│  ┌────────────────────────────────────────────────────────────────────────────────┐ │
│  │  INTEGRATION TESTING / PRE-PRODUCTION                                          │ │
│  │  ════════════════════════════════════                                          │ │
│  │  Use: Option C (Linux VM)                                                      │ │
│  │                                                                                │ │
│  │  • Full production-like environment                                            │ │
│  │  • Test everything together                                                    │ │
│  │  • Catch Linux-specific issues                                                 │ │
│  │                                                                                │ │
│  │  Command: limactl start && limactl shell clarateach                            │ │
│  └────────────────────────────────────────────────────────────────────────────────┘ │
│                                                                                     │
│  ┌────────────────────────────────────────────────────────────────────────────────┐ │
│  │  CI/CD                                                                         │ │
│  │  ═════                                                                         │ │
│  │  Use: Option A (Backend in Docker)                                             │ │
│  │                                                                                │ │
│  │  • Runs on any CI platform (GitHub Actions, Cloud Build)                       │ │
│  │  • Reproducible environment                                                    │ │
│  │  • Test full flow in containers                                                │ │
│  └────────────────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 6. Implementation Priority

### Immediate (Unblock Development)

**Implement Option D** - Takes 5 minutes:

```bash
# Already works, just use it:
./scripts/stack.sh start

# Set session manually in browser console for testing:
localStorage.setItem('clarateach_session', JSON.stringify({
  endpoint: 'http://localhost:8000',
  seat: 1,
  token: '',
  odehash: 'test',
  code: 'TEST',
  name: 'Developer'
}));

# Navigate to /workspace
```

### Short-term (Backend Development)

**Implement Option B (Port Mapping)** - Half day of work:

1. Update `Instance` struct with host port fields (already done)
2. Modify `docker.go` to add port bindings when creating containers
3. Modify `proxy.go` to use host ports when available
4. Test end-to-end flow

### Long-term (Production-like Testing)

**Set up Option C (Linux VM)** - 1-2 hours:

1. Install Lima or OrbStack
2. Configure port forwarding
3. Document setup steps for team
4. Use for integration testing before deploys

---

## 7. Files to Modify

| File | Option A | Option B | Option C | Option D |
|------|----------|----------|----------|----------|
| `backend/internal/orchestrator/provider.go` | - | ✏️ Add port fields | - | - |
| `backend/internal/orchestrator/docker.go` | ✏️ Network config | ✏️ Port bindings | - | - |
| `backend/internal/proxy/proxy.go` | - | ✏️ Use host ports | - | - |
| `backend/internal/api/server.go` | - | - | - | ✏️ Endpoint override |
| `docker-compose.dev.yml` | ✏️ Create new | - | - | - |
| `scripts/stack.sh` | ✏️ Add backend service | - | - | - |

---

## 8. Conclusion

The networking issue is a fundamental platform difference. There's no single "best" solution—the right choice depends on what you're developing:

- **Frontend work** → Option D (Caddy setup)
- **Backend work** → Option A or B
- **Full integration** → Option C (Linux VM)
- **CI/CD** → Option A (Backend in Docker)

All options eventually converge to the same production architecture where the Portal API provisions VMs and returns direct endpoints to learners.
