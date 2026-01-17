# Firecracker MicroVM Architecture

This document describes the architecture of ClaraTeach's Firecracker-based MicroVM system, which provides isolated workspace environments for learners.

## Overview

Each learner gets an isolated MicroVM running inside a Firecracker virtual machine. The system uses a layered architecture:

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Frontend (React)                            │
│                      localhost:5173 / 5174                          │
└─────────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      Backend API (Go)                               │
│                        localhost:8080                               │
│  • Workshop management                                              │
│  • Seat allocation                                                  │
│  • Proxies requests to Agent VMs                                    │
└─────────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    Agent VM (GCP Instance)                          │
│                    External IP: varies                              │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                  Agent Server (:9090)                        │   │
│  │  • Manages Firecracker processes                             │   │
│  │  • Proxies to MicroVMs                                       │   │
│  │  • Routes: /proxy/{workshopID}/{seatID}/*                    │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                              │                                      │
│           ┌──────────────────┼──────────────────┐                  │
│           ▼                  ▼                  ▼                   │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐            │
│  │  MicroVM 1  │    │  MicroVM 2  │    │  MicroVM N  │            │
│  │ 192.168.100 │    │ 192.168.100 │    │ 192.168.100 │            │
│  │    .11      │    │    .12      │    │   .10+N     │            │
│  │  :3001 term │    │  :3001 term │    │  :3001 term │            │
│  │  :3002 files│    │  :3002 files│    │  :3002 files│            │
│  └─────────────┘    └─────────────┘    └─────────────┘            │
└─────────────────────────────────────────────────────────────────────┘
```

## Component Details

### Frontend

- **Technology**: React + Vite + TypeScript
- **Ports**: 5173 (dev), 5174 (portal)
- **Key Components**:
  - `WorkspacePage.tsx` - Main workspace UI
  - `FileExplorer.tsx` - File browser panel
  - `MonacoEditor` - Code editor
  - `XTerm.js` - Terminal emulator

### Backend API

- **Technology**: Go
- **Port**: 8080
- **Key Responsibilities**:
  - Workshop CRUD operations
  - Seat registration and management
  - Proxying WebSocket/HTTP to Agent VMs
  - Database management (SQLite)

**Key Routes**:
```
POST /api/workshops              - Create workshop
GET  /api/workshops              - List workshops
POST /api/register               - Register for seat
GET  /api/workshops/:id/proxy/*  - Proxy to agent
```

### Agent Server

- **Technology**: Go
- **Port**: 9090
- **Location**: Runs on GCP VM provisioned from snapshot
- **Key Responsibilities**:
  - Starting/stopping Firecracker MicroVMs
  - Managing tap network interfaces
  - Proxying requests to correct MicroVM

**Proxy Routes**:
```
/proxy/{workshopID}/{seatID}/terminal  → MicroVM:3001 (WebSocket)
/proxy/{workshopID}/{seatID}/files     → MicroVM:3002 (HTTP)
/proxy/{workshopID}/{seatID}/files/*   → MicroVM:3002 (HTTP)
```

### MicroVM (Firecracker)

- **Technology**: Firecracker + Custom rootfs
- **Ports**: 3001 (terminal), 3002 (files)
- **IP Scheme**: `192.168.100.{10 + seatID}`
- **Components**:
  - Linux kernel (vmlinux)
  - Root filesystem (rootfs.ext4)
  - Workspace server (Node.js)

## Network Architecture

### MicroVM Networking

Each MicroVM gets a unique IP on the `192.168.100.0/24` subnet:

```
┌─────────────────────────────────────────────────┐
│                  Agent VM                        │
│                                                  │
│  eth0 (external) ◄─── GCP assigns public IP     │
│       │                                          │
│       │  NAT/Masquerade                         │
│       ▼                                          │
│  ┌─────────────────────────────────────────┐    │
│  │         192.168.100.1 (host)            │    │
│  │              tap interfaces              │    │
│  │  tap0 ─── 192.168.100.11 (MicroVM 1)   │    │
│  │  tap1 ─── 192.168.100.12 (MicroVM 2)   │    │
│  │  tapN ─── 192.168.100.{10+N}           │    │
│  └─────────────────────────────────────────┘    │
└─────────────────────────────────────────────────┘
```

### Port Assignments

| Service | Port | Protocol | Description |
|---------|------|----------|-------------|
| Frontend | 5173/5174 | HTTP | Development servers |
| Backend API | 8080 | HTTP | Main API |
| Agent Server | 9090 | HTTP/WS | Agent management |
| Terminal | 3001 | WebSocket | PTY terminal |
| Files API | 3002 | HTTP | File operations |

## Critical Environment Variables

### MICROVM_MODE

**This is the most critical variable.** It changes how the workspace server registers routes.

| Value | Route Registration | Used By |
|-------|-------------------|---------|
| `false` or unset | `/vm/:seat/files`, `/vm/:seat/terminal` | Docker/container mode |
| `true` | `/files`, `/terminal` | Firecracker MicroVM mode |

**Why it matters**: The agent proxy expects routes at `/files` and `/terminal`. Without `MICROVM_MODE=true`, the workspace server registers routes with `/vm/:seat/` prefix, causing 404 errors.

### Other Required Variables

```bash
AUTH_DISABLED=true       # Disable auth in MicroVM (auth handled at backend)
HOME=/home/learner       # User home directory
WORKSPACE_DIR=/workspace # Working directory for learner
NODE_ENV=production      # Node.js environment
```

## Request Flow

### Terminal Connection

```
1. Frontend opens WebSocket to:
   /api/workshops/{workshopID}/proxy/{seatID}/terminal

2. Backend proxies to Agent:
   http://{agentIP}:9090/proxy/{workshopID}/{seatID}/terminal

3. Agent proxies to MicroVM:
   http://192.168.100.{10+seatID}:3001/terminal

4. Workspace server upgrades to WebSocket, attaches PTY
```

### File Operations

```
1. Frontend requests:
   GET /api/workshops/{workshopID}/proxy/{seatID}/files

2. Backend proxies to Agent:
   GET http://{agentIP}:9090/proxy/{workshopID}/{seatID}/files

3. Agent proxies to MicroVM:
   GET http://192.168.100.{10+seatID}:3002/files

4. Workspace server returns file listing
```

## Deployment

### GCP Snapshot-Based Provisioning

Agent VMs are provisioned from GCP snapshots containing:
- Pre-installed Firecracker binary
- Pre-built rootfs.ext4 with workspace server
- vmlinux kernel
- Agent server binary

**Snapshot naming**: `clarateach-agent-YYYYMMDD-HHMMSS`

### Updating the System

1. Modify code (backend, workspace server, or init script)
2. Rebuild rootfs if workspace server changed
3. Create new GCP snapshot
4. Update `FC_SNAPSHOT_NAME` in configuration
5. New workshops will use updated snapshot

See [Update GCP Snapshot Procedure](../operations/update-gcp-snapshot.md) for details.

## Troubleshooting

Common issues and their causes:

| Symptom | Likely Cause | Solution |
|---------|--------------|----------|
| "Connection closed" in terminal | MICROVM_MODE not set | Check init script sets `MICROVM_MODE=true` |
| 404 on /files endpoint | Wrong route registration | Verify MICROVM_MODE and agent proxy config |
| MicroVM exits immediately | Init script error | Check for `set -e` issues, use robust init |
| No network in MicroVM | tap interface not created | Check agent tap setup and IP forwarding |

See [Troubleshooting Guide](../troubleshooting/workspace-issues.md) for more details.
