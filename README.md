# ClaraTeach

A platform for running interactive coding workshops with isolated learner workspaces powered by Firecracker MicroVMs.

## Project Structure

```
clarateach/
├── frontend/          # React web application
├── backend/           # Go API server and agent
├── workspace/         # MicroVM workspace server (Node.js)
├── scripts/           # Build, setup, and test scripts
├── docs/              # Architecture and planning documentation
└── tests/             # Integration tests
```

## Components

| Component | Technology | Description |
|-----------|------------|-------------|
| **Frontend** | React + Vite + TypeScript | Web UI for teachers and learners. Includes terminal (xterm.js), code editor (Monaco), and file browser. |
| **Backend Server** | Go (Chi router) | Control plane API. Manages workshops, provisions GCP VMs, orchestrates MicroVMs. Runs on port 8080. |
| **Backend Agent** | Go | Runs on worker VMs. Manages Firecracker MicroVMs, proxies terminal/file requests. Runs on port 9090. |
| **Workspace Server** | Node.js | Runs inside each MicroVM. Provides terminal (port 3001) and file API (port 3002) for learners. |
| **Rootfs Builder** | Go | Builds the ext4 root filesystem image used by MicroVMs. |

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│  Frontend (React)                                                │
│  - Teacher dashboard                                            │
│  - Learner workspace (terminal + editor + files)                │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│  Backend Server (port 8080)                                      │
│  - Workshop CRUD API                                            │
│  - GCP VM provisioning                                          │
│  - Agent coordination                                           │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│  GCP Worker VM (clara2)                                          │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  Agent (port 9090)                                         │  │
│  │  - MicroVM lifecycle (create/destroy)                      │  │
│  │  - Proxy: terminal WebSocket, file API                     │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │
│  │  MicroVM 1  │  │  MicroVM 2  │  │  MicroVM 3  │   ...        │
│  │  (seat 1)   │  │  (seat 2)   │  │  (seat 3)   │              │
│  │  .100.11    │  │  .100.12    │  │  .100.13    │              │
│  └─────────────┘  └─────────────┘  └─────────────┘              │
└─────────────────────────────────────────────────────────────────┘
```

## Developer Workflow

### Prerequisites

- Go 1.24+
- Node.js 20+
- Docker (for rootfs building)
- GCP access (for full E2E testing)

### 1. Frontend Development

```bash
cd frontend
npm install
npm run dev          # Start dev server (http://localhost:5173)
npm run build        # Production build
npm run test         # Run tests
npm run typecheck    # Type checking
```

### 2. Backend Development

```bash
cd backend

# Run the control plane server
go run ./cmd/server/

# Run the agent (on a worker VM)
go run ./cmd/agent/

# Build the rootfs builder
go build -o rootfs-builder ./cmd/rootfs-builder/

# Run tests
go test ./...

# Build all binaries
go build ./cmd/server/
go build ./cmd/agent/
```

### 3. Build Rootfs Image

```bash
# Build the MicroVM root filesystem (requires Docker)
cd scripts
sudo ./build-rootfs.sh
```

### 4. Testing

```bash
# Local E2E test (on worker VM with agent running)
cd ~/clarateach
./scripts/test-e2e-local.sh

# Full GCP E2E test
./scripts/test-e2e-gcp.sh

# Backend unit tests
cd backend && go test ./...

# Frontend tests
cd frontend && npm test
```

### 5. Deployment

```bash
# Setup a new Linux VM
./scripts/setup_linux_vm.sh

# Install Firecracker
./scripts/setup-firecracker.sh

# Prepare VM snapshot for fast provisioning
sudo ./scripts/prepare-snapshot.sh
```

## Scripts Reference

| Script | Purpose |
|--------|---------|
| `build-rootfs.sh` | Build the ext4 rootfs image for MicroVMs using Docker |
| `setup-firecracker.sh` | Download and install Firecracker binary and kernel |
| `setup_linux_vm.sh` | Initial setup for a new GCP worker VM |
| `prepare-snapshot.sh` | Prepare a GCP VM snapshot with agent pre-installed |
| `test-e2e-local.sh` | Run E2E tests locally on a worker VM |
| `test-e2e-gcp.sh` | Run full E2E tests through the backend API |
| `test-firecracker.sh` | Test Firecracker MicroVM creation directly |
| `test_backend_flow.sh` | Test backend API flow |
| `run_backend_local.sh` | Run backend server locally |
| `run_backend_docker.sh` | Run backend server in Docker |
| `stack.sh` | Docker Compose stack management |
| `push-image.sh` | Push Docker images to GCP Artifact Registry |
| `clarateach-agent.service` | Systemd service file for the agent |

## Environment Variables

### Backend Server

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server port | `8080` |
| `GCP_PROJECT` | GCP project ID | - |
| `GCP_ZONE` | GCP zone | `us-central1-b` |
| `GCP_REGISTRY` | Artifact Registry URL | - |
| `FC_SNAPSHOT_NAME` | Snapshot name for worker VMs | `clara2-snapshot` |
| `JWT_SECRET` | Secret for JWT signing | - |

### Agent

| Variable | Description | Default |
|----------|-------------|---------|
| `AGENT_PORT` | Agent port | `9090` |
| `WORKER_ID` | Worker identifier | hostname |
| `IMAGES_PATH` | Path to kernel and rootfs | `/var/lib/clarateach/images` |

## Documentation

- [TestingGuide.md](docs/TestingGuide.md) - How to test MicroVM functionality
- [CurrentFeature.md](docs/CurrentFeature.md) - Current development focus
- [ARCHITECTURE.md](docs/ARCHITECTURE.md) - System architecture
- [API_SPEC.md](docs/API_SPEC.md) - API specification

## Quick Commands

```bash
# Check agent health
curl localhost:9090/health

# List MicroVMs
curl localhost:9090/vms

# Create a MicroVM
curl -X POST localhost:9090/vms -H "Content-Type: application/json" \
  -d '{"workshop_id": "test", "seat_id": 1}'

# Delete a MicroVM
curl -X DELETE localhost:9090/vms/test/1
```
