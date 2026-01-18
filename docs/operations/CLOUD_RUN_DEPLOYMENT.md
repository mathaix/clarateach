# ClaraTeach Cloud Run Deployment Guide

This guide documents the complete process for deploying ClaraTeach to Google Cloud Run.

## Prerequisites

### Required Tools

```bash
# Google Cloud CLI
gcloud --version

# Docker
docker --version

# Authenticate with GCP
gcloud auth login
gcloud auth configure-docker us-central1-docker.pkg.dev
```

### GCP Project Configuration

```bash
# Set your project
export GCP_PROJECT=clarateach
gcloud config set project $GCP_PROJECT

# Enable required APIs
gcloud services enable \
  run.googleapis.com \
  artifactregistry.googleapis.com \
  secretmanager.googleapis.com \
  compute.googleapis.com

# Create Artifact Registry repository (if not exists)
gcloud artifacts repositories create clarateach \
  --repository-format=docker \
  --location=us-central1 \
  --description="ClaraTeach container images"
```

---

## Backend Deployment

### 1. Backend Dockerfile

Location: `backend/Dockerfile`

```dockerfile
# ClaraTeach Backend - Multi-stage Dockerfile for Cloud Run
# Builds a minimal container for the Go API server

# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the server binary
# CGO_ENABLED=0 for static binary, GOOS=linux for Linux target
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server ./cmd/server/

# Runtime stage - minimal image
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for HTTPS calls (GCP APIs, Neon DB)
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user for security
RUN adduser -D -u 1000 appuser
USER appuser

# Copy binary from builder
COPY --from=builder /app/server /app/server

# Copy migrations (needed for DB setup)
COPY --from=builder /app/migrations /app/migrations

# Cloud Run uses PORT env var (default 8080)
ENV PORT=8080

# Expose the port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the server
CMD ["/app/server"]
```

### 2. Build Backend Docker Image

```bash
cd backend

# Build for linux/amd64 (required for Cloud Run)
docker build --platform linux/amd64 -t us-central1-docker.pkg.dev/clarateach/clarateach/backend:latest .

# Tag with version (optional but recommended)
docker tag us-central1-docker.pkg.dev/clarateach/clarateach/backend:latest \
  us-central1-docker.pkg.dev/clarateach/clarateach/backend:v1.0.0
```

### 3. Push Backend Image to Artifact Registry

```bash
docker push us-central1-docker.pkg.dev/clarateach/clarateach/backend:latest
```

### 4. Set Up Secrets in GCP Secret Manager

```bash
# Database URL (Neon PostgreSQL)
echo -n "postgresql://user:password@ep-xxx.us-east-2.aws.neon.tech/neondb?sslmode=require" | \
  gcloud secrets create DATABASE_URL --data-file=-

# Agent authentication token
echo -n "your-secure-agent-token" | \
  gcloud secrets create FC_AGENT_TOKEN --data-file=-

# JWT signing secret for workspace tokens
echo -n "your-jwt-signing-secret" | \
  gcloud secrets create WORKSPACE_TOKEN_SECRET --data-file=-

# JWT secret for user authentication
echo -n "your-jwt-secret" | \
  gcloud secrets create JWT_SECRET --data-file=-
```

### 5. Deploy Backend to Cloud Run

**CRITICAL Environment Variables:**

| Variable | Purpose | Example |
|----------|---------|---------|
| `GCP_PROJECT` | GCP project ID for VM provisioning | `clarateach` |
| `GCP_ZONE` | GCP zone for VMs | `us-central1-b` |
| `GCP_REGISTRY` | Artifact Registry URL for container images | `us-central1-docker.pkg.dev/clarateach/clarateach` |
| `FC_SNAPSHOT_NAME` | Snapshot name for Firecracker VMs | `clara2-snapshot-20260117-auth` |
| `CORS_ORIGINS` | Allowed CORS origins (frontend URL) | `https://clarateach-frontend-xxx.run.app` |
| `BACKEND_URL` | **Public URL of this backend service** | `https://clarateach-backend-xxx.run.app` |

**Why BACKEND_URL is critical:** When workshop VMs are created, `BACKEND_URL` is passed as GCP metadata. The VM's tunnel service uses this to auto-register its Cloudflare Quick Tunnel URL with the backend. Without this, VMs cannot report their tunnel URLs, and the frontend will fail to connect (Mixed Content errors when trying HTTP from HTTPS).

```bash
# First, get the backend URL (after initial deployment) or use the known URL
export BACKEND_URL=https://clarateach-backend-864969804676.us-central1.run.app

gcloud run deploy clarateach-backend \
  --image=us-central1-docker.pkg.dev/clarateach/clarateach/backend:latest \
  --region=us-central1 \
  --platform=managed \
  --allow-unauthenticated \
  --port=8080 \
  --memory=512Mi \
  --cpu=1 \
  --min-instances=0 \
  --max-instances=10 \
  --set-env-vars="GCP_PROJECT=clarateach,GCP_ZONE=us-central1-b,FC_SNAPSHOT_NAME=clara2-snapshot-20260117-auth,CORS_ORIGINS=https://clarateach-frontend-864969804676.us-central1.run.app,BACKEND_URL=$BACKEND_URL" \
  --set-secrets="DATABASE_URL=DATABASE_URL:latest,FC_AGENT_TOKEN=FC_AGENT_TOKEN:latest,WORKSPACE_TOKEN_SECRET=WORKSPACE_TOKEN_SECRET:latest,JWT_SECRET=JWT_SECRET:latest"
```

**Note:** For initial deployment, you may need to deploy once without `BACKEND_URL`, then update with the correct URL after Cloud Run assigns it.

### 6. Verify Backend Deployment

```bash
# Get the backend URL
BACKEND_URL=$(gcloud run services describe clarateach-backend \
  --region=us-central1 \
  --format='value(status.url)')

echo "Backend URL: $BACKEND_URL"

# Test health endpoint
curl "$BACKEND_URL/health"
# Expected: {"status":"ok","fc_snapshot":"clara2-snapshot-20260117-auth"}

# Test API endpoint
curl "$BACKEND_URL/api/workshops"
# Expected: {"workshops":[]} or list of workshops
```

---

## Frontend Deployment

### 1. Frontend Dockerfile

Location: `frontend/Dockerfile`

```dockerfile
# ClaraTeach Frontend - Multi-stage Dockerfile for Cloud Run
# Builds React app and serves with nginx

# Build stage
FROM node:20-alpine AS builder

WORKDIR /app

# Copy package files
COPY package.json package-lock.json ./

# Install dependencies
RUN npm ci

# Copy source code
COPY . .

# Build the application
RUN npm run build

# Production stage - nginx
FROM nginx:alpine

# Remove default nginx config
RUN rm /etc/nginx/conf.d/default.conf

# Copy custom nginx config
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Copy built assets from builder
COPY --from=builder /app/dist /usr/share/nginx/html

# Cloud Run uses PORT env var (default 8080)
EXPOSE 8080

# Start nginx
CMD ["nginx", "-g", "daemon off;"]
```

### 2. Nginx Configuration

Location: `frontend/nginx.conf`

```nginx
server {
    listen 8080;
    server_name _;

    root /usr/share/nginx/html;
    index index.html;

    # Gzip compression
    gzip on;
    gzip_types text/plain text/css application/json application/javascript text/xml application/xml text/javascript;

    # API proxy to backend
    location /api/ {
        proxy_pass https://clarateach-backend-864969804676.us-central1.run.app/api/;
        proxy_http_version 1.1;
        proxy_set_header Host clarateach-backend-864969804676.us-central1.run.app;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket support
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        # Timeouts for long-running connections
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }

    # Health check endpoint (for Cloud Run)
    location /health {
        proxy_pass https://clarateach-backend-864969804676.us-central1.run.app/health;
        proxy_http_version 1.1;
        proxy_set_header Host clarateach-backend-864969804676.us-central1.run.app;
    }

    # SPA routing - serve index.html for all non-file routes
    location / {
        try_files $uri $uri/ /index.html;
    }

    # Cache static assets
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
}
```

### 3. Frontend .dockerignore

Location: `frontend/.dockerignore`

```
# Dependencies
node_modules/

# Build output (we rebuild in Docker)
dist/

# IDE and editor files
.idea/
.vscode/
*.swp
*.swo

# Test files
*.test.ts
*.test.tsx
__tests__/
coverage/

# Logs
*.log
.vite/

# Git
.git/
.gitignore

# Local env files
.env
.env.local
.env.*.local
```

### 4. Build Frontend Docker Image

```bash
cd frontend

# Build for linux/amd64 (required for Cloud Run)
docker build --platform linux/amd64 -t us-central1-docker.pkg.dev/clarateach/clarateach/frontend:latest .

# Tag with version (optional but recommended)
docker tag us-central1-docker.pkg.dev/clarateach/clarateach/frontend:latest \
  us-central1-docker.pkg.dev/clarateach/clarateach/frontend:v1.0.0
```

### 5. Push Frontend Image to Artifact Registry

```bash
docker push us-central1-docker.pkg.dev/clarateach/clarateach/frontend:latest
```

### 6. Deploy Frontend to Cloud Run

```bash
gcloud run deploy clarateach-frontend \
  --image=us-central1-docker.pkg.dev/clarateach/clarateach/frontend:latest \
  --region=us-central1 \
  --platform=managed \
  --allow-unauthenticated \
  --port=8080 \
  --memory=256Mi \
  --cpu=1 \
  --min-instances=0 \
  --max-instances=10
```

### 7. Verify Frontend Deployment

```bash
# Get the frontend URL
FRONTEND_URL=$(gcloud run services describe clarateach-frontend \
  --region=us-central1 \
  --format='value(status.url)')

echo "Frontend URL: $FRONTEND_URL"

# Test static files
curl -s -o /dev/null -w "%{http_code}" "$FRONTEND_URL"
# Expected: 200

# Test health proxy
curl "$FRONTEND_URL/health"
# Expected: {"status":"ok","fc_snapshot":"..."}

# Test API proxy
curl "$FRONTEND_URL/api/workshops"
# Expected: {"workshops":[...]}
```

---

## Complete Deployment Script

Here's a complete script to deploy both services:

```bash
#!/bin/bash
set -e

# Configuration
export GCP_PROJECT=clarateach
export GCP_REGION=us-central1
export REGISTRY=us-central1-docker.pkg.dev/$GCP_PROJECT/clarateach
export FC_SNAPSHOT_NAME=clara2-snapshot-20260117-auth

echo "=== ClaraTeach Cloud Run Deployment ==="

# Authenticate Docker with GCP
gcloud auth configure-docker us-central1-docker.pkg.dev --quiet

# Deploy Backend (initial deployment to get URL)
echo ""
echo "=== Building Backend ==="
cd backend
docker build --platform linux/amd64 -t $REGISTRY/backend:latest .

echo "=== Pushing Backend ==="
docker push $REGISTRY/backend:latest

echo "=== Deploying Backend to Cloud Run (initial) ==="
gcloud run deploy clarateach-backend \
  --image=$REGISTRY/backend:latest \
  --region=$GCP_REGION \
  --platform=managed \
  --allow-unauthenticated \
  --port=8080 \
  --memory=512Mi \
  --set-env-vars="GCP_PROJECT=$GCP_PROJECT,GCP_ZONE=us-central1-b,FC_SNAPSHOT_NAME=$FC_SNAPSHOT_NAME" \
  --set-secrets="DATABASE_URL=DATABASE_URL:latest,FC_AGENT_TOKEN=FC_AGENT_TOKEN:latest,WORKSPACE_TOKEN_SECRET=WORKSPACE_TOKEN_SECRET:latest,JWT_SECRET=JWT_SECRET:latest" \
  --quiet

BACKEND_URL=$(gcloud run services describe clarateach-backend --region=$GCP_REGION --format='value(status.url)')
echo "Backend deployed: $BACKEND_URL"

# Deploy Frontend
echo ""
echo "=== Building Frontend ==="
cd ../frontend
docker build --platform linux/amd64 -t $REGISTRY/frontend:latest .

echo "=== Pushing Frontend ==="
docker push $REGISTRY/frontend:latest

echo "=== Deploying Frontend to Cloud Run ==="
gcloud run deploy clarateach-frontend \
  --image=$REGISTRY/frontend:latest \
  --region=$GCP_REGION \
  --platform=managed \
  --allow-unauthenticated \
  --port=8080 \
  --memory=256Mi \
  --quiet

FRONTEND_URL=$(gcloud run services describe clarateach-frontend --region=$GCP_REGION --format='value(status.url)')
echo "Frontend deployed: $FRONTEND_URL"

# CRITICAL: Update backend with BACKEND_URL and CORS_ORIGINS
# This is required for VM tunnel auto-registration to work
echo ""
echo "=== Updating Backend with BACKEND_URL and CORS_ORIGINS ==="
gcloud run services update clarateach-backend \
  --region=$GCP_REGION \
  --update-env-vars="BACKEND_URL=$BACKEND_URL,CORS_ORIGINS=$FRONTEND_URL" \
  --quiet
echo "Backend updated with BACKEND_URL=$BACKEND_URL"

# Verification
echo ""
echo "=== Verification ==="
echo "Backend health:"
curl -s "$BACKEND_URL/health" | head -c 100
echo ""
echo "Frontend status:"
curl -s -o /dev/null -w "HTTP %{http_code}" "$FRONTEND_URL"
echo ""

echo ""
echo "=== Deployment Complete ==="
echo "Frontend: $FRONTEND_URL"
echo "Backend:  $BACKEND_URL"
echo ""
echo "IMPORTANT: Any existing workshop VMs will need to be recreated"
echo "for the tunnel auto-registration to work with the new BACKEND_URL."
```

---

## Updating Deployments

### Update Backend Only

```bash
cd backend
docker build --platform linux/amd64 -t us-central1-docker.pkg.dev/clarateach/clarateach/backend:latest .
docker push us-central1-docker.pkg.dev/clarateach/clarateach/backend:latest
gcloud run deploy clarateach-backend \
  --image=us-central1-docker.pkg.dev/clarateach/clarateach/backend:latest \
  --region=us-central1
```

### Update Frontend Only

```bash
cd frontend
docker build --platform linux/amd64 -t us-central1-docker.pkg.dev/clarateach/clarateach/frontend:latest .
docker push us-central1-docker.pkg.dev/clarateach/clarateach/frontend:latest
gcloud run deploy clarateach-frontend \
  --image=us-central1-docker.pkg.dev/clarateach/clarateach/frontend:latest \
  --region=us-central1
```

### Update Environment Variables

```bash
# Update backend env vars
gcloud run services update clarateach-backend \
  --region=us-central1 \
  --set-env-vars="NEW_VAR=value"

# Update secrets
gcloud run services update clarateach-backend \
  --region=us-central1 \
  --set-secrets="NEW_SECRET=secret-name:latest"
```

---

## Troubleshooting

### View Logs

```bash
# Backend logs
gcloud run services logs read clarateach-backend --region=us-central1 --limit=50

# Frontend logs
gcloud run services logs read clarateach-frontend --region=us-central1 --limit=50

# Stream logs
gcloud run services logs tail clarateach-backend --region=us-central1
```

### Check Service Status

```bash
# List all services
gcloud run services list --region=us-central1

# Describe specific service
gcloud run services describe clarateach-backend --region=us-central1
```

### Common Issues

#### "Permission denied" pushing to Artifact Registry

```bash
# Re-authenticate Docker
gcloud auth configure-docker us-central1-docker.pkg.dev
```

#### "exec format error" on Cloud Run

Ensure you're building for the correct platform:
```bash
docker build --platform linux/amd64 ...
```

#### CORS Errors

Update the backend's `CORS_ORIGINS` environment variable:
```bash
gcloud run services update clarateach-backend \
  --region=us-central1 \
  --set-env-vars="CORS_ORIGINS=https://your-frontend-url.run.app"
```

#### Secret Not Found

```bash
# List secrets
gcloud secrets list

# Create missing secret
echo -n "value" | gcloud secrets create SECRET_NAME --data-file=-

# Grant Cloud Run access
gcloud secrets add-iam-policy-binding SECRET_NAME \
  --member="serviceAccount:864969804676-compute@developer.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"
```

---

## Current Deployment URLs

| Service | URL |
|---------|-----|
| Frontend | https://clarateach-frontend-864969804676.us-central1.run.app |
| Backend | https://clarateach-backend-864969804676.us-central1.run.app |

---

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Google Cloud Run                             │
│                                                                      │
│  ┌────────────────────────┐      ┌────────────────────────────┐     │
│  │   clarateach-frontend  │      │   clarateach-backend       │     │
│  │   (nginx + React)      │─────▶│   (Go API Server)          │     │
│  │                        │ /api │                            │     │
│  │   Port 8080            │      │   Port 8080                │     │
│  └────────────────────────┘      └─────────────┬──────────────┘     │
│                                                │                     │
└────────────────────────────────────────────────┼─────────────────────┘
                                                 │
                    ┌────────────────────────────┼────────────────────┐
                    │                            │                     │
                    ▼                            ▼                     ▼
           ┌───────────────┐          ┌─────────────────┐    ┌────────────────┐
           │  Neon DB      │          │  GCP Compute    │    │  Secret Manager│
           │  (PostgreSQL) │          │  (Workshop VMs) │    │  (Credentials) │
           └───────────────┘          └─────────────────┘    └────────────────┘
```
