#!/bin/bash
# Run the Backend using a temporary Go Docker container
# Useful if you don't have Go installed locally.

docker run --rm \
  --label clarateach.component=backend \
  -v "$(pwd)/backend:/app" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -w /app \
  -p 8080:8080 \
  -e DB_PATH=/app/clarateach.db \
  -e BASE_DOMAIN=localhost \
  golang:1.24-bullseye \
  bash -c "go mod tidy && go run cmd/server/main.go"
