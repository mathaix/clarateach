#!/bin/bash
# ClaraTeach GCP One-Time Setup Script
# Run this ONCE before your first deployment
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

echo "=========================================="
echo "ClaraTeach GCP One-Time Setup"
echo "=========================================="
echo "Project: $GCP_PROJECT"
echo "Region:  $GCP_REGION"
echo "=========================================="
echo ""

# Check if gcloud is authenticated
log "Checking gcloud authentication..."
gcloud auth print-identity-token &>/dev/null || error "Not authenticated. Run: gcloud auth login"

# Set project
log "Setting GCP project to $GCP_PROJECT..."
gcloud config set project $GCP_PROJECT

# Enable required APIs
log "Enabling required GCP APIs..."
gcloud services enable \
    run.googleapis.com \
    artifactregistry.googleapis.com \
    secretmanager.googleapis.com \
    compute.googleapis.com \
    --quiet

# Create Artifact Registry repository
log "Creating Artifact Registry repository..."
if gcloud artifacts repositories describe clarateach --location=$GCP_REGION &>/dev/null; then
    warn "Artifact Registry repository 'clarateach' already exists"
else
    gcloud artifacts repositories create clarateach \
        --repository-format=docker \
        --location=$GCP_REGION \
        --description="ClaraTeach container images"
    log "Created Artifact Registry repository"
fi

# Configure Docker authentication
log "Configuring Docker authentication..."
gcloud auth configure-docker ${GCP_REGION}-docker.pkg.dev --quiet

# Get service account for Cloud Run
SERVICE_ACCOUNT="864969804676-compute@developer.gserviceaccount.com"

# Create secrets
echo ""
log "Setting up secrets..."

create_secret() {
    local name=$1
    local description=$2

    if gcloud secrets describe $name --project=$GCP_PROJECT &>/dev/null; then
        warn "Secret '$name' already exists"
    else
        # Generate secure random value
        local value=$(openssl rand -base64 32)
        echo -n "$value" | gcloud secrets create $name \
            --data-file=- \
            --project=$GCP_PROJECT
        log "Created secret: $name"

        # Grant access to Cloud Run service account
        gcloud secrets add-iam-policy-binding $name \
            --member="serviceAccount:$SERVICE_ACCOUNT" \
            --role="roles/secretmanager.secretAccessor" \
            --project=$GCP_PROJECT \
            --quiet
        log "Granted access to $name for Cloud Run"
    fi
}

# DATABASE_URL needs to be set manually with actual Neon connection string
if gcloud secrets describe DATABASE_URL --project=$GCP_PROJECT &>/dev/null; then
    warn "Secret 'DATABASE_URL' already exists"
else
    echo ""
    echo -e "${YELLOW}DATABASE_URL secret does not exist.${NC}"
    echo "You need to create it manually with your Neon PostgreSQL connection string:"
    echo ""
    echo "  echo -n 'postgresql://user:pass@host/db?sslmode=require' | \\"
    echo "    gcloud secrets create DATABASE_URL --data-file=- --project=$GCP_PROJECT"
    echo ""
    read -p "Do you want to enter the DATABASE_URL now? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        read -p "Enter DATABASE_URL: " DATABASE_URL_VALUE
        echo -n "$DATABASE_URL_VALUE" | gcloud secrets create DATABASE_URL \
            --data-file=- \
            --project=$GCP_PROJECT
        gcloud secrets add-iam-policy-binding DATABASE_URL \
            --member="serviceAccount:$SERVICE_ACCOUNT" \
            --role="roles/secretmanager.secretAccessor" \
            --project=$GCP_PROJECT \
            --quiet
        log "Created and configured DATABASE_URL secret"
    fi
fi

# Ensure DATABASE_URL has proper IAM binding
if gcloud secrets describe DATABASE_URL --project=$GCP_PROJECT &>/dev/null; then
    gcloud secrets add-iam-policy-binding DATABASE_URL \
        --member="serviceAccount:$SERVICE_ACCOUNT" \
        --role="roles/secretmanager.secretAccessor" \
        --project=$GCP_PROJECT \
        --quiet 2>/dev/null || true
fi

# Create auto-generated secrets
create_secret "FC_AGENT_TOKEN" "Agent authentication token"
create_secret "WORKSPACE_TOKEN_SECRET" "Secret for signing workspace tokens"
create_secret "JWT_SECRET" "Secret for JWT authentication"

# List all secrets
echo ""
log "Current secrets:"
gcloud secrets list --project=$GCP_PROJECT

echo ""
echo "=========================================="
echo -e "${GREEN}Setup Complete${NC}"
echo "=========================================="
echo ""
echo "Next steps:"
echo "  1. Verify DATABASE_URL is set correctly"
echo "  2. Run the deployment script:"
echo "     ./scripts/deploy-cloud-run.sh"
echo ""
echo "=========================================="
