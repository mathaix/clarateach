#!/bin/bash
set -e

# ClaraTeach Workspace Entrypoint
# Starts the workspace server (terminal and file API)

echo "Starting ClaraTeach Workspace Server..."
echo "TERMINAL_PORT: ${TERMINAL_PORT:-3001}"
echo "FILES_PORT: ${FILES_PORT:-3002}"
echo "AUTH_DISABLED: ${AUTH_DISABLED:-false}"
echo "WORKSPACE_DIR: ${WORKSPACE_DIR:-/workspace}"

# Change to server directory and start
cd /home/learner/server
exec node dist/index.js
