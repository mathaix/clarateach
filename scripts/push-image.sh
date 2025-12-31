#!/bin/bash
# Push workspace image to Google Artifact Registry
#
# Prerequisites:
#   1. Install gcloud: brew install google-cloud-sdk
#   2. Authenticate: gcloud auth login
#   3. Create Artifact Registry:
#      gcloud artifacts repositories create clarateach \
#        --repository-format=docker \
#        --location=us-central1 \
#        --project=clarateach
#   4. Configure Docker auth:
#      gcloud auth configure-docker us-central1-docker.pkg.dev

set -e

PROJECT=${GCP_PROJECT:-clarateach}
REGION=${GCP_REGION:-us-central1}
REPO=clarateach
IMAGE_NAME=workspace
TAG=${1:-latest}

REGISTRY="$REGION-docker.pkg.dev/$PROJECT/$REPO"
FULL_IMAGE="$REGISTRY/$IMAGE_NAME:$TAG"

echo "=== ClaraTeach Image Push ==="
echo "Project: $PROJECT"
echo "Registry: $REGISTRY"
echo "Image: $FULL_IMAGE"
echo ""

# Check if gcloud is installed
if ! command -v gcloud &> /dev/null; then
    echo "Error: gcloud CLI not installed"
    echo "Install with: brew install google-cloud-sdk"
    exit 1
fi

# Check if authenticated
if ! gcloud auth print-identity-token &> /dev/null; then
    echo "Error: Not authenticated with gcloud"
    echo "Run: gcloud auth login"
    exit 1
fi

# Check if Artifact Registry repo exists
echo "Checking Artifact Registry..."
if ! gcloud artifacts repositories describe $REPO --location=$REGION --project=$PROJECT &> /dev/null; then
    echo "Creating Artifact Registry repository..."
    gcloud artifacts repositories create $REPO \
        --repository-format=docker \
        --location=$REGION \
        --project=$PROJECT \
        --description="ClaraTeach Docker images"
fi

# Configure Docker to use gcloud credentials
echo "Configuring Docker authentication..."
gcloud auth configure-docker $REGION-docker.pkg.dev --quiet

# Build the image for amd64 (GCP VMs run on amd64)
echo ""
echo "Building workspace image for linux/amd64..."
cd "$(dirname "$0")/../workspace"
docker build --platform linux/amd64 -t $IMAGE_NAME:$TAG .

# Tag for registry
echo ""
echo "Tagging image..."
docker tag $IMAGE_NAME:$TAG $FULL_IMAGE

# Push to registry
echo ""
echo "Pushing to Artifact Registry..."
docker push $FULL_IMAGE

echo ""
echo "=== Success! ==="
echo "Image pushed: $FULL_IMAGE"
echo ""
echo "Set this in your .env:"
echo "GCP_REGISTRY=$REGISTRY"
