#!/bin/bash
# clarateach-tunnel.sh - Start Cloudflare Quick Tunnel and report URL to backend
set -euo pipefail

METADATA_URL="http://metadata.google.internal/computeMetadata/v1/instance/attributes"
METADATA_HEADER="Metadata-Flavor: Google"

log() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"; }

# Fetch metadata from GCP
fetch_metadata() {
    local key=$1
    curl -s -H "$METADATA_HEADER" "$METADATA_URL/$key" 2>/dev/null || echo ""
}

# Wait for metadata service
log "Waiting for metadata service..."
for i in {1..30}; do
    if curl -s -H "$METADATA_HEADER" "$METADATA_URL/" &>/dev/null; then
        break
    fi
    sleep 1
done

# Get required metadata
WORKSHOP_ID=$(fetch_metadata "workshop-id")
BACKEND_URL=$(fetch_metadata "backend-url")
WORKSPACE_TOKEN_SECRET=$(fetch_metadata "workspace-token-secret")

if [[ -z "$WORKSHOP_ID" ]]; then
    log "ERROR: workshop-id metadata not found"
    exit 1
fi

if [[ -z "$BACKEND_URL" ]]; then
    log "ERROR: backend-url metadata not found"
    exit 1
fi

# Export for agent if needed
if [[ -n "$WORKSPACE_TOKEN_SECRET" ]]; then
    echo "WORKSPACE_TOKEN_SECRET=$WORKSPACE_TOKEN_SECRET" >> /run/clarateach-agent.env
fi

log "Workshop ID: $WORKSHOP_ID"
log "Backend URL: $BACKEND_URL"

# Function to report tunnel URL to backend
report_tunnel_url() {
    local tunnel_url=$1
    log "Reporting tunnel URL to backend: $tunnel_url"

    for i in {1..5}; do
        RESPONSE=$(curl -s -w "%{http_code}" -o /tmp/tunnel-response.txt \
            -X POST "$BACKEND_URL/api/internal/workshops/$WORKSHOP_ID/tunnel" \
            -H "Content-Type: application/json" \
            -d "{\"tunnel_url\": \"$tunnel_url\"}" 2>/dev/null)

        if [[ "$RESPONSE" == "200" ]]; then
            log "Successfully registered tunnel URL"
            return 0
        fi

        log "Failed to register tunnel (attempt $i): HTTP $RESPONSE"
        sleep 2
    done

    log "ERROR: Failed to register tunnel URL after 5 attempts"
    return 1
}

# Start cloudflared and capture URL
log "Starting Cloudflare Quick Tunnel..."

# Create a named pipe for capturing output
PIPE=/tmp/cloudflared-pipe
rm -f $PIPE
mkfifo $PIPE

# Start cloudflared in background, redirecting stderr to pipe
cloudflared tunnel --url http://localhost:9090 2>&1 | tee $PIPE &
CLOUDFLARED_PID=$!

# Read from pipe and look for the tunnel URL
URL_FOUND=false
while read -r line; do
    echo "$line"  # Echo to stdout for logging

    if [[ "$line" == *"trycloudflare.com"* ]] && [[ "$URL_FOUND" == "false" ]]; then
        # Extract URL from the line
        TUNNEL_URL=$(echo "$line" | grep -oE 'https://[^ ]+trycloudflare\.com' | head -1)

        if [[ -n "$TUNNEL_URL" ]]; then
            log "Captured tunnel URL: $TUNNEL_URL"
            URL_FOUND=true

            # Report to backend (in background so we don't block)
            report_tunnel_url "$TUNNEL_URL" &
        fi
    fi
done < $PIPE &
READER_PID=$!

# Wait for cloudflared (this keeps the service running)
wait $CLOUDFLARED_PID
