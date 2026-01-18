# ClaraTeach Operations Guide

This guide covers deploying, configuring, and operating ClaraTeach in development and production environments.

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Prerequisites](#prerequisites)
3. [Database Setup (PostgreSQL/Neon)](#database-setup)
4. [GCP Configuration](#gcp-configuration)
5. [Credentials Management](#credentials-management)
6. [Local Development](#local-development)
7. [Production Deployment](#production-deployment)
8. [Operations Tasks](#operations-tasks)
9. [Troubleshooting](#troubleshooting)

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    CONTROL PLANE                             │
│  ┌─────────────────┐    ┌──────────────────────────────┐    │
│  │  Frontend       │    │  Backend (Go)                 │    │
│  │  (React/Vite)   │───▶│  - API Server                │    │
│  │  Port 5173      │    │  - GCP Firecracker Provisioner│   │
│  └─────────────────┘    │  - PostgreSQL (Neon)          │    │
│                         │  Port 8080                    │    │
│                         └──────────────┬───────────────┘    │
└────────────────────────────────────────┼────────────────────┘
                                         │ Creates VM + calls Agent API
                                         ▼
┌─────────────────────────────────────────────────────────────┐
│              GCP VM (per workshop, from snapshot)            │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  Agent Server (port 9090)                            │    │
│  │  - Manages Firecracker MicroVMs                      │    │
│  │  - Proxies Terminal (WebSocket) and Files (HTTP)     │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                              │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐                      │
│  │ MicroVM │  │ MicroVM │  │ MicroVM │  ... (up to ~30)     │
│  │ Seat 1  │  │ Seat 2  │  │ Seat 3  │                      │
│  │ 2 vCPU  │  │ 2 vCPU  │  │ 2 vCPU  │                      │
│  │ 512MB   │  │ 512MB   │  │ 512MB   │                      │
│  └─────────┘  └─────────┘  └─────────┘                      │
│       │192.168.100.11  │.12       │.13                      │
└───────┴────────────────┴──────────┴─────────────────────────┘
```

## Prerequisites

### Required Tools

```bash
# Google Cloud CLI
brew install google-cloud-sdk  # macOS
# Or: https://cloud.google.com/sdk/docs/install

# Go 1.21+
brew install go

# Node.js 18+
brew install node

# PostgreSQL client (for migrations)
brew install postgresql
```

### GCP Project Setup

1. Enable required APIs:
```bash
gcloud services enable compute.googleapis.com
gcloud services enable secretmanager.googleapis.com
gcloud services enable artifactregistry.googleapis.com
```

2. Create Artifact Registry for Docker images:
```bash
gcloud artifacts repositories create clarateach \
  --repository-format=docker \
  --location=us-central1
```

---

## Database Setup

ClaraTeach uses **PostgreSQL** (recommended: [Neon](https://neon.tech) for serverless).

### Option A: Neon (Recommended for Production)

1. Create account at [neon.tech](https://neon.tech)
2. Create a new project (e.g., "clarateach")
3. Copy the connection string from the dashboard

```bash
# Format
DATABASE_URL=postgresql://user:password@ep-xxx.us-east-2.aws.neon.tech/neondb?sslmode=require
```

### Option B: Local PostgreSQL (Development)

```bash
# Start PostgreSQL
brew services start postgresql

# Create database
createdb clarateach

# Connection string
DATABASE_URL=postgresql://localhost/clarateach?sslmode=disable
```

### Running Migrations

```bash
cd backend

# Using psql directly
psql "$DATABASE_URL" -f migrations/001_initial_schema.up.sql

# Or use golang-migrate (if installed)
migrate -database "$DATABASE_URL" -path migrations up
```

### Database Schema

| Table | Purpose |
|-------|---------|
| `users` | Instructor accounts |
| `workshops` | Workshop configurations |
| `sessions` | Learner seats within workshops |
| `workshop_vms` | GCP VM tracking (IP, SSH keys, status) |
| `registrations` | Learner registration with access codes |

---

## GCP Configuration

### Required Permissions

The service account needs:
- `compute.instances.*` - Create/manage VMs
- `compute.disks.*` - Create disks from snapshots
- `compute.snapshots.useReadOnly` - Use snapshots
- `secretmanager.versions.access` - Read secrets

### Creating a Service Account

```bash
# Create service account
gcloud iam service-accounts create clarateach-backend \
  --display-name="ClaraTeach Backend"

# Grant permissions
gcloud projects add-iam-policy-binding clarateach \
  --member="serviceAccount:clarateach-backend@clarateach.iam.gserviceaccount.com" \
  --role="roles/compute.instanceAdmin.v1"

gcloud projects add-iam-policy-binding clarateach \
  --member="serviceAccount:clarateach-backend@clarateach.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"

# Create and download key (for local dev)
gcloud iam service-accounts keys create ~/clarateach-sa-key.json \
  --iam-account=clarateach-backend@clarateach.iam.gserviceaccount.com

export GOOGLE_APPLICATION_CREDENTIALS=~/clarateach-sa-key.json
```

---

## Credentials Management

### Overview of Secrets

| Secret | Purpose | Where Used |
|--------|---------|------------|
| `DATABASE_URL` | PostgreSQL connection | Backend server |
| `FC_AGENT_TOKEN` | Agent API authentication | Backend → Agent |
| `WORKSPACE_TOKEN_SECRET` | JWT signing for workspace auth | Backend + Agent |

### Option A: GCP Secret Manager (Production)

```bash
# Create secrets
echo -n "postgresql://user:pass@host/db" | \
  gcloud secrets create DATABASE_URL --data-file=-

echo -n "your-secure-agent-token" | \
  gcloud secrets create FC_AGENT_TOKEN --data-file=-

echo -n "your-jwt-signing-secret" | \
  gcloud secrets create WORKSPACE_TOKEN_SECRET --data-file=-

# Grant access to service account
for secret in DATABASE_URL FC_AGENT_TOKEN WORKSPACE_TOKEN_SECRET; do
  gcloud secrets add-iam-policy-binding $secret \
    --member="serviceAccount:clarateach-backend@clarateach.iam.gserviceaccount.com" \
    --role="roles/secretmanager.secretAccessor"
done
```

The backend automatically loads secrets from GCP Secret Manager when `GCP_PROJECT` is set.

### Option B: Environment Variables (Local Dev)

Create `.env` file in the project root:

```bash
# .env (DO NOT COMMIT)

# Database
DATABASE_URL=postgresql://user:password@host/database?sslmode=require

# GCP
GCP_PROJECT=clarateach
GCP_ZONE=us-central1-b
GCP_REGISTRY=us-central1-docker.pkg.dev/clarateach/clarateach

# Firecracker
FC_SNAPSHOT_NAME=clara2-snapshot-20260117-auth
FC_AGENT_TOKEN=  # Empty for dev (skips agent auth)
WORKSPACE_TOKEN_SECRET=clarateach-dev-secret-2024

# Optional
PORT=8080
CORS_ORIGINS=http://localhost:5173
```

### Agent Token Configuration

The agent token is passed to GCP VMs via instance metadata:

1. Backend stores token in config
2. When creating VM, token is set as metadata: `agent-token=<value>`
3. Agent reads token on startup from GCP metadata service
4. If token is empty, agent skips authentication (dev mode)

---

## Local Development

### Quick Start

```bash
# 1. Clone and setup
git clone <repo>
cd ClaraTeach

# 2. Create .env with DATABASE_URL (see above)
cp backend/.env.example .env
# Edit .env with your DATABASE_URL

# 3. Run migrations
cd backend
psql "$DATABASE_URL" -f migrations/001_initial_schema.up.sql

# 4. Start backend (Terminal 1)
./scripts/start-backend.sh

# 5. Start frontend (Terminal 2)
./scripts/start-frontend.sh

# 6. Open browser
open http://localhost:5173
```

### Environment Variables Reference

```bash
# Required
DATABASE_URL          # PostgreSQL connection string
GCP_PROJECT           # GCP project ID (e.g., "clarateach")

# Required for Firecracker workshops
FC_SNAPSHOT_NAME      # GCP snapshot name for agent VMs
GCP_ZONE              # GCP zone (e.g., "us-central1-b")
GCP_REGISTRY          # Docker registry path

# Optional
PORT                  # Backend port (default: 8080)
CORS_ORIGINS          # Allowed origins (default: "*")
FC_AGENT_TOKEN        # Agent auth token (empty = no auth)
WORKSPACE_TOKEN_SECRET # JWT secret (has default for dev)
GCP_USE_SPOT          # Use spot VMs (default: false)
```

### Building the Agent

The agent runs on GCP VMs and requires Linux:

```bash
cd backend

# Cross-compile from macOS
GOOS=linux GOARCH=amd64 go build -o agent-linux ./cmd/agent/

# Deploy to existing VM
gcloud compute scp agent-linux clarateach@<VM_NAME>:/tmp/
gcloud compute ssh <VM_NAME> --command="sudo mv /tmp/agent-linux /usr/local/bin/agent && sudo systemctl restart clarateach-agent"
```

---

## Production Deployment

> **Complete Guide:** See [Cloud Run Deployment Guide](./CLOUD_RUN_DEPLOYMENT.md) for detailed Docker build and deployment steps.

### Backend Deployment Options

#### Option 1: Cloud Run (Recommended)

```bash
# Build for linux/amd64 (required for Cloud Run)
cd backend
docker build --platform linux/amd64 -t us-central1-docker.pkg.dev/clarateach/clarateach/backend:latest .
docker push us-central1-docker.pkg.dev/clarateach/clarateach/backend:latest

# Deploy to Cloud Run
gcloud run deploy clarateach-backend \
  --image=us-central1-docker.pkg.dev/clarateach/clarateach/backend:latest \
  --region=us-central1 \
  --allow-unauthenticated \
  --port=8080 \
  --memory=512Mi \
  --set-env-vars="GCP_PROJECT=clarateach,GCP_ZONE=us-central1-b,FC_SNAPSHOT_NAME=clara2-snapshot-20260117-auth" \
  --set-secrets="DATABASE_URL=DATABASE_URL:latest,FC_AGENT_TOKEN=FC_AGENT_TOKEN:latest,WORKSPACE_TOKEN_SECRET=WORKSPACE_TOKEN_SECRET:latest,JWT_SECRET=JWT_SECRET:latest"
```

#### Option 2: GCE VM

```bash
# Create VM
gcloud compute instances create clarateach-backend \
  --zone=us-central1-b \
  --machine-type=e2-small \
  --image-family=ubuntu-2204-lts \
  --image-project=ubuntu-os-cloud \
  --service-account=clarateach-backend@clarateach.iam.gserviceaccount.com \
  --scopes=cloud-platform

# SSH and setup
gcloud compute ssh clarateach-backend --zone=us-central1-b

# Install Go, copy binary, setup systemd...
```

### Frontend Deployment

#### Option 1: Cloud Run with nginx (Recommended)

```bash
cd frontend

# Build Docker image with nginx
docker build --platform linux/amd64 -t us-central1-docker.pkg.dev/clarateach/clarateach/frontend:latest .
docker push us-central1-docker.pkg.dev/clarateach/clarateach/frontend:latest

# Deploy to Cloud Run
gcloud run deploy clarateach-frontend \
  --image=us-central1-docker.pkg.dev/clarateach/clarateach/frontend:latest \
  --region=us-central1 \
  --allow-unauthenticated \
  --port=8080 \
  --memory=256Mi
```

The nginx configuration proxies `/api/*` requests to the backend Cloud Run service.

#### Option 2: Cloud Storage + CDN

```bash
cd frontend

# Build static files
npm run build

# Deploy to Cloud Storage
gsutil -m cp -r dist/* gs://clarateach-frontend/
```

### Creating Agent Snapshot

See [update-gcp-snapshot.md](./update-gcp-snapshot.md) for detailed instructions.

Quick reference:
```bash
# 1. Build agent
GOOS=linux GOARCH=amd64 go build -o agent ./cmd/agent/

# 2. Deploy to base VM
gcloud compute scp agent clara2:/tmp/
gcloud compute ssh clara2 --command="sudo mv /tmp/agent /usr/local/bin/"

# 3. Stop VM and create snapshot
gcloud compute instances stop clara2 --zone=us-central1-b
gcloud compute snapshots create clara2-snapshot-$(date +%Y%m%d) \
  --source-disk=clara2 \
  --source-disk-zone=us-central1-b

# 4. Update FC_SNAPSHOT_NAME
export FC_SNAPSHOT_NAME=clara2-snapshot-YYYYMMDD
```

---

## Operations Tasks

### Monitoring Workshop VMs

```bash
# List all workshop VMs
gcloud compute instances list --filter="name~clarateach-fc-ws"

# Check VM quota
gcloud compute regions describe us-central1 \
  --format="get(quotas)" | tr ';' '\n' | grep CPU
```

### Checking Agent Health

```bash
# Direct health check
curl http://<VM_EXTERNAL_IP>:9090/health

# Check MicroVM count
curl http://<VM_EXTERNAL_IP>:9090/health | jq '.vm_count'
```

### Cleaning Up Resources

```bash
# Delete old workshop VMs
gcloud compute instances delete clarateach-fc-ws-<ID> --zone=us-central1-b

# Delete old snapshots (keep last 3)
gcloud compute snapshots list --filter="name~clara" \
  --sort-by=~creationTimestamp \
  --format="value(name)" | tail -n +4 | \
  xargs -I {} gcloud compute snapshots delete {} --quiet
```

### Database Maintenance

```bash
# Connect to Neon database
psql "$DATABASE_URL"

# Check workshop status
SELECT id, name, status, created_at FROM workshops ORDER BY created_at DESC LIMIT 10;

# Clean up deleted workshops
DELETE FROM sessions WHERE workshop_id IN (SELECT id FROM workshops WHERE status = 'deleted');
DELETE FROM workshops WHERE status = 'deleted' AND created_at < NOW() - INTERVAL '7 days';
```

---

## Troubleshooting

### Common Issues

#### "DATABASE_URL is required"

```bash
# Ensure .env exists and has DATABASE_URL
cat .env | grep DATABASE_URL

# Or export directly
export DATABASE_URL=postgresql://...
```

#### "CORS preflight failed" / 405 on OPTIONS

The agent needs explicit OPTIONS handlers for CORS. Ensure agent is updated:
```bash
# Check agent version
curl http://<VM_IP>:9090/health

# Update agent if needed
GOOS=linux GOARCH=amd64 go build -o agent-linux ./cmd/agent/
gcloud compute scp agent-linux <VM>:/tmp/
gcloud compute ssh <VM> --command="sudo mv /tmp/agent-linux /usr/local/bin/agent && sudo systemctl restart clarateach-agent"
```

#### "VM not found" after agent restart

Agent tracks MicroVMs in memory. After restart, MicroVMs must be re-provisioned:
```bash
# Check VM count
curl http://<VM_IP>:9090/health | jq '.vm_count'

# Manually provision MicroVM for seat
curl -X POST http://<VM_IP>:9090/vms/ \
  -H "Content-Type: application/json" \
  -d '{"workshop_id":"<WS_ID>","seat_id":1}'
```

#### GCP CPU Quota Exceeded

```bash
# Check usage
gcloud compute project-info describe --project=clarateach \
  --format="get(quotas)" | tr ';' '\n' | grep CPUS_ALL_REGIONS

# Delete unused VMs
gcloud compute instances list
gcloud compute instances delete <VM_NAME> --zone=us-central1-b
```

#### Terminal connects but Explorer shows "No files"

This is a CORS issue. Verify OPTIONS requests return 200:
```bash
curl -X OPTIONS http://<VM_IP>:9090/proxy/<WS_ID>/<SEAT>/files -v
# Should return HTTP 200 with Access-Control-* headers
```

### Logs

```bash
# Backend logs
./scripts/start-backend.sh 2>&1 | tee backend.log

# Agent logs on GCP VM
gcloud compute ssh <VM> --command="journalctl -u clarateach-agent -f"

# MicroVM logs
gcloud compute ssh <VM> --command="cat /var/log/firecracker-*.log"
```

---

## Quick Reference

### Start Development

```bash
# Terminal 1: Backend
./scripts/start-backend.sh

# Terminal 2: Frontend
./scripts/start-frontend.sh
```

### Deploy Agent Update

```bash
cd backend
GOOS=linux GOARCH=amd64 go build -o agent-linux ./cmd/agent/
gcloud compute scp agent-linux <VM>:/tmp/
gcloud compute ssh <VM> --command="sudo mv /tmp/agent-linux /usr/local/bin/agent && sudo systemctl restart clarateach-agent"
```

### Create New Snapshot

```bash
gcloud compute instances stop <VM> --zone=us-central1-b
gcloud compute snapshots create clarateach-snapshot-$(date +%Y%m%d-%H%M%S) \
  --source-disk=<VM> --source-disk-zone=us-central1-b
gcloud compute instances start <VM> --zone=us-central1-b
```

### URLs

| Environment | Frontend | Backend |
|-------------|----------|---------|
| Local Dev | http://localhost:5173 | http://localhost:8080 |
| Cloud Run | https://clarateach-frontend-864969804676.us-central1.run.app | https://clarateach-backend-864969804676.us-central1.run.app |
| Production | https://learn.claramap.com | https://api.claramap.com |

---

## See Also

- [Cloud Run Deployment Guide](./CLOUD_RUN_DEPLOYMENT.md) - Complete Docker build and Cloud Run deployment steps
