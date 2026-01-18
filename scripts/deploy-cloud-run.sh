#!/bin/bash
# ClaraTeach Cloud Run Deployment Script
# Deploys both frontend and backend to Google Cloud Run
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() { echo -e "${GREEN}[INFO]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# Configuration
export GCP_PROJECT=${GCP_PROJECT:-clarateach}
export GCP_REGION=${GCP_REGION:-us-central1}
export GCP_ZONE=${GCP_ZONE:-us-central1-b}
export REGISTRY=us-central1-docker.pkg.dev/$GCP_PROJECT/clarateach
export FC_SNAPSHOT_NAME=${FC_SNAPSHOT_NAME:-clarateach-agent-20260117-230800}

# Determine what to deploy
DEPLOY_BACKEND=false
DEPLOY_FRONTEND=false

usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --backend     Deploy backend only"
    echo "  --frontend    Deploy frontend only"
    echo "  --all         Deploy both (default if no option specified)"
    echo "  --help        Show this help message"
    echo ""
    echo "Environment variables:"
    echo "  GCP_PROJECT       GCP project ID (default: clarateach)"
    echo "  GCP_REGION        GCP region (default: us-central1)"
    echo "  FC_SNAPSHOT_NAME  Firecracker snapshot name"
    exit 0
}

# Parse arguments
if [[ $# -eq 0 ]]; then
    DEPLOY_BACKEND=true
    DEPLOY_FRONTEND=true
fi

while [[ $# -gt 0 ]]; do
    case $1 in
        --backend)
            DEPLOY_BACKEND=true
            shift
            ;;
        --frontend)
            DEPLOY_FRONTEND=true
            shift
            ;;
        --all)
            DEPLOY_BACKEND=true
            DEPLOY_FRONTEND=true
            shift
            ;;
        --help)
            usage
            ;;
        *)
            error "Unknown option: $1"
            ;;
    esac
done

# Get script directory and project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "=========================================="
echo "ClaraTeach Cloud Run Deployment"
echo "=========================================="
echo "Project:  $GCP_PROJECT"
echo "Region:   $GCP_REGION"
echo "Snapshot: $FC_SNAPSHOT_NAME"
echo "Backend:  $DEPLOY_BACKEND"
echo "Frontend: $DEPLOY_FRONTEND"
echo "=========================================="
echo ""

# Authenticate Docker with GCP
log "Authenticating Docker with GCP Artifact Registry..."
gcloud auth configure-docker us-central1-docker.pkg.dev --quiet

# Deploy Backend
if [[ "$DEPLOY_BACKEND" == "true" ]]; then
    log "Building backend..."
    cd "$PROJECT_ROOT/backend"
    docker build --platform linux/amd64 -t $REGISTRY/backend:latest .

    log "Pushing backend image..."
    docker push $REGISTRY/backend:latest

    log "Deploying backend to Cloud Run..."

    # Deploy backend first (BACKEND_URL will be updated after we get the actual URL)
    gcloud run deploy clarateach-backend \
        --image=$REGISTRY/backend:latest \
        --region=$GCP_REGION \
        --platform=managed \
        --allow-unauthenticated \
        --port=8080 \
        --memory=512Mi \
        --cpu=1 \
        --min-instances=0 \
        --max-instances=10 \
        --set-env-vars="GCP_PROJECT=$GCP_PROJECT,GCP_ZONE=$GCP_ZONE,GCP_REGISTRY=$REGISTRY,FC_SNAPSHOT_NAME=$FC_SNAPSHOT_NAME,CORS_ORIGINS=https://clarateach-frontend-864969804676.us-central1.run.app" \
        --set-secrets="DATABASE_URL=DATABASE_URL:latest,FC_AGENT_TOKEN=FC_AGENT_TOKEN:latest,WORKSPACE_TOKEN_SECRET=WORKSPACE_TOKEN_SECRET:latest,JWT_SECRET=JWT_SECRET:latest"

    # Get project number for the correct URL format
    PROJECT_NUMBER=$(gcloud projects describe $GCP_PROJECT --format='value(projectNumber)')
    BACKEND_URL="https://clarateach-backend-${PROJECT_NUMBER}.${GCP_REGION}.run.app"

    log "Updating BACKEND_URL to: $BACKEND_URL"
    gcloud run services update clarateach-backend \
        --region=$GCP_REGION \
        --update-env-vars="BACKEND_URL=$BACKEND_URL" \
        --quiet

    log "Backend deployed: $BACKEND_URL"
fi

# Deploy Frontend
if [[ "$DEPLOY_FRONTEND" == "true" ]]; then
    log "Building frontend..."
    cd "$PROJECT_ROOT/frontend"
    docker build --platform linux/amd64 -t $REGISTRY/frontend:latest .

    log "Pushing frontend image..."
    docker push $REGISTRY/frontend:latest

    log "Deploying frontend to Cloud Run..."
    gcloud run deploy clarateach-frontend \
        --image=$REGISTRY/frontend:latest \
        --region=$GCP_REGION \
        --platform=managed \
        --allow-unauthenticated \
        --port=8080 \
        --memory=256Mi \
        --cpu=1 \
        --min-instances=0 \
        --max-instances=10

    FRONTEND_URL=$(gcloud run services describe clarateach-frontend \
        --region=$GCP_REGION \
        --format='value(status.url)')

    log "Frontend deployed: $FRONTEND_URL"
fi

# Verification
echo ""
echo "=========================================="
log "Verifying deployment..."
echo "=========================================="

if [[ "$DEPLOY_BACKEND" == "true" ]]; then
    BACKEND_URL=$(gcloud run services describe clarateach-backend \
        --region=$GCP_REGION \
        --format='value(status.url)')
    echo -n "Backend health: "
    curl -s "$BACKEND_URL/health" || echo "FAILED"
    echo ""
fi

if [[ "$DEPLOY_FRONTEND" == "true" ]]; then
    FRONTEND_URL=$(gcloud run services describe clarateach-frontend \
        --region=$GCP_REGION \
        --format='value(status.url)')
    echo -n "Frontend status: "
    curl -s -o /dev/null -w "HTTP %{http_code}" "$FRONTEND_URL"
    echo ""
fi

echo ""
echo "=========================================="
echo -e "${GREEN}Deployment Complete${NC}"
echo "=========================================="
[[ "$DEPLOY_FRONTEND" == "true" ]] && echo "Frontend: $FRONTEND_URL"
[[ "$DEPLOY_BACKEND" == "true" ]] && echo "Backend:  $BACKEND_URL"
echo ""
echo "Note: Existing workshop VMs may need to be recreated"
echo "for tunnel auto-registration to work with BACKEND_URL."
echo "=========================================="
