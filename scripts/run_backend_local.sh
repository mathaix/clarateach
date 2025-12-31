#!/bin/bash
# Run the Backend locally (requires Go installed)

# Ensure we are in the backend directory
cd "$(dirname "$0")/../backend"

# Detect Docker Host
DOCKER_HOST_DETECTED=$(docker context inspect --format '{{.Endpoints.docker.Host}}' 2>/dev/null || echo "unix:///var/run/docker.sock")
export DOCKER_HOST="$DOCKER_HOST_DETECTED"

# Environment Variables
export DB_PATH="./clarateach.db"
export BASE_DOMAIN="localhost"
export PORT="8080"

echo ">>> Starting Backend Locally on Port $PORT..."
echo "    Database: $DB_PATH"
echo "    Docker:   $DOCKER_HOST"

# Run the server
go run cmd/server/main.go
