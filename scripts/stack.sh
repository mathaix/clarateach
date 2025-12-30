#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PORTAL_DIR="$ROOT_DIR/portal"
FRONTEND_DIR="$PORTAL_DIR/frontend"
WORKSPACE_DIR="$ROOT_DIR/workspace"

# Load configuration from .env file and export variables
ENV_FILE="$ROOT_DIR/.env"
if [ -f "$ENV_FILE" ]; then
  # Export all variables from .env
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
fi

# Configuration with defaults
PORTAL_PORT="${PORTAL_PORT:-3000}"
PORTAL_HOST="${PORTAL_HOST:-0.0.0.0}"
FRONTEND_PORT="${FRONTEND_PORT:-5173}"
CADDY_PORT="${CADDY_PORT:-8000}"
WORKSPACE_ENDPOINT_OVERRIDE="${WORKSPACE_ENDPOINT_OVERRIDE:-http://localhost:$CADDY_PORT}"

PORTAL_PID_FILE="$PORTAL_DIR/.portal.pid"
PORTAL_LOG_FILE="$PORTAL_DIR/.portal.log"
VITE_PID_FILE="$FRONTEND_DIR/.vite.pid"
VITE_LOG_FILE="$FRONTEND_DIR/.vite.log"

wait_for_url() {
  local url="$1"
  local retries="${2:-40}"
  local delay="${3:-0.25}"

  if ! command -v curl >/dev/null 2>&1; then
    return 0
  fi

  for _ in $(seq 1 "$retries"); do
    if curl -sf "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep "$delay"
  done
  return 1
}

get_lsof_bin() {
  if command -v lsof >/dev/null 2>&1; then
    command -v lsof
    return 0
  fi
  if [ -x /usr/sbin/lsof ]; then
    echo /usr/sbin/lsof
    return 0
  fi
  if [ -x /usr/bin/lsof ]; then
    echo /usr/bin/lsof
    return 0
  fi
  return 1
}

check_port_in_use() {
  local port="$1"
  local lsof_bin
  if ! lsof_bin=$(get_lsof_bin); then
    return 1
  fi
  if "$lsof_bin" -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    return 0
  fi
  return 1
}

realpath_portable() {
  local target="$1"
  if [ -d "$target" ]; then
    (cd "$target" && pwd -P)
    return 0
  fi
  echo "$target"
  return 0
}

kill_port_owner() {
  local port="$1"
  local expected_dir="$2"
  local expected_real
  expected_real=$(realpath_portable "$expected_dir")

  local lsof_bin
  if ! lsof_bin=$(get_lsof_bin); then
    return 0
  fi

  local pids
  pids=$("$lsof_bin" -nP -iTCP:"$port" -sTCP:LISTEN -t 2>/dev/null || true)
  if [ -z "$pids" ]; then
    return 0
  fi

  local matched=false
  for pid in $pids; do
    local cwd
    cwd=$("$lsof_bin" -p "$pid" -a -d cwd -F n 2>/dev/null | sed -n 's/^n//p' | head -n 1)
    local cwd_real
    cwd_real=$(realpath_portable "$cwd")
    local cwd_norm
    local expected_norm
    cwd_norm=$(echo "$cwd_real" | tr '[:upper:]' '[:lower:]')
    expected_norm=$(echo "$expected_real" | tr '[:upper:]' '[:lower:]')
    if [ -n "$cwd_norm" ] && [ "$cwd_norm" = "$expected_norm" ]; then
      matched=true
      echo "Stopping process ${pid} listening on port ${port} (cwd: ${cwd})"
      kill -TERM -"${pid}" 2>/dev/null || kill "${pid}" 2>/dev/null || true
      for _ in {1..20}; do
        if kill -0 "${pid}" 2>/dev/null; then
          sleep 0.2
        else
          break
        fi
      done
      if kill -0 "${pid}" 2>/dev/null; then
        kill -KILL -"${pid}" 2>/dev/null || kill -9 "${pid}" 2>/dev/null || true
      fi
    fi
  done

  if [ "$matched" != "true" ]; then
    echo "Port ${port} is in use by another process (not managed by stack.sh)." >&2
  fi
}

compose_cmd() {
  if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    echo "docker compose"
    return 0
  fi
  if command -v docker-compose >/dev/null 2>&1; then
    echo "docker-compose"
    return 0
  fi
  return 1
}

run_compose() {
  local cmd
  if ! cmd="$(compose_cmd)"; then
    echo "Docker Compose not found. Install Docker Desktop or docker-compose." >&2
    exit 1
  fi
  # Pass env file to docker compose if it exists
  local env_args=""
  if [ -f "$ENV_FILE" ]; then
    env_args="--env-file $ENV_FILE"
  fi
  (cd "$WORKSPACE_DIR" && $cmd $env_args "$@")
}

start_portal() {
  if [ -f "$PORTAL_PID_FILE" ]; then
    local pid
    pid="$(cat "$PORTAL_PID_FILE" 2>/dev/null || true)"
    if [ -n "${pid}" ] && kill -0 "${pid}" 2>/dev/null; then
      echo "Portal API already running (pid ${pid})."
      return 0
    fi
    rm -f "$PORTAL_PID_FILE"
  fi

  if check_port_in_use "$PORTAL_PORT"; then
    echo "Portal API port ${PORTAL_PORT} is already in use. Stop the existing process or change PORTAL_PORT." >&2
    return 1
  fi

  if ! command -v npm >/dev/null 2>&1; then
    echo "npm not found. Skipping portal start." >&2
    return 0
  fi

  if [ ! -d "$PORTAL_DIR/node_modules" ]; then
    echo "Installing portal dependencies..."
    (cd "$PORTAL_DIR" && npm install)
  fi

  (cd "$PORTAL_DIR" && PORT="$PORTAL_PORT" HOST="$PORTAL_HOST" WORKSPACE_ENDPOINT_OVERRIDE="$WORKSPACE_ENDPOINT_OVERRIDE" nohup npm run dev > "$PORTAL_LOG_FILE" 2>&1 & echo $! > "$PORTAL_PID_FILE")
  if wait_for_url "http://localhost:$PORTAL_PORT/api/health"; then
    echo "Portal API started on http://localhost:$PORTAL_PORT. Logs: $PORTAL_LOG_FILE"
    return 0
  fi

  echo "Portal API failed to start. Check logs at $PORTAL_LOG_FILE" >&2
  rm -f "$PORTAL_PID_FILE"
  return 1
}

stop_portal() {
  if [ ! -f "$PORTAL_PID_FILE" ]; then
    echo "Portal API not running (pid file missing)."
    kill_port_owner "$PORTAL_PORT" "$PORTAL_DIR"
    return 0
  fi

  local pid
  pid="$(cat "$PORTAL_PID_FILE" 2>/dev/null || true)"
  if [ -z "${pid}" ]; then
    rm -f "$PORTAL_PID_FILE"
    echo "Portal pid file was empty. Removed."
    return 0
  fi

  if kill -0 "${pid}" 2>/dev/null; then
    kill -TERM -"${pid}" 2>/dev/null || kill "${pid}" 2>/dev/null || true
    for _ in {1..20}; do
      if kill -0 "${pid}" 2>/dev/null; then
        sleep 0.2
      else
        break
      fi
    done
    if kill -0 "${pid}" 2>/dev/null; then
      echo "Portal did not stop, sending SIGKILL."
      kill -KILL -"${pid}" 2>/dev/null || kill -9 "${pid}" 2>/dev/null || true
    fi
  fi

  kill_port_owner "$PORTAL_PORT" "$PORTAL_DIR"

  rm -f "$PORTAL_PID_FILE"
  echo "Portal API stopped."
}

start_frontend() {
  if [ -f "$VITE_PID_FILE" ]; then
    local pid
    pid="$(cat "$VITE_PID_FILE" 2>/dev/null || true)"
    if [ -n "${pid}" ] && kill -0 "${pid}" 2>/dev/null; then
      echo "Frontend already running (pid ${pid})."
      return 0
    fi
    rm -f "$VITE_PID_FILE"
  fi

  if check_port_in_use "$FRONTEND_PORT"; then
    echo "Frontend port ${FRONTEND_PORT} is already in use. Stop the existing process or change FRONTEND_PORT." >&2
    return 1
  fi

  if ! command -v npm >/dev/null 2>&1; then
    echo "npm not found. Skipping frontend start." >&2
    return 0
  fi

  if [ ! -d "$FRONTEND_DIR/node_modules" ]; then
    echo "Installing frontend dependencies..."
    (cd "$FRONTEND_DIR" && npm install)
  fi

  (cd "$FRONTEND_DIR" && PORTAL_PORT="$PORTAL_PORT" FRONTEND_PORT="$FRONTEND_PORT" nohup npm run dev > "$VITE_LOG_FILE" 2>&1 & echo $! > "$VITE_PID_FILE")
  if wait_for_url "http://localhost:$FRONTEND_PORT"; then
    echo "Frontend started on http://localhost:$FRONTEND_PORT. Logs: $VITE_LOG_FILE"
    return 0
  fi

  echo "Frontend failed to start. Check logs at $VITE_LOG_FILE" >&2
  rm -f "$VITE_PID_FILE"
  return 1
}

stop_frontend() {
  if [ ! -f "$VITE_PID_FILE" ]; then
    echo "Frontend not running (pid file missing)."
    kill_port_owner "$FRONTEND_PORT" "$FRONTEND_DIR"
    return 0
  fi

  local pid
  pid="$(cat "$VITE_PID_FILE" 2>/dev/null || true)"
  if [ -z "${pid}" ]; then
    rm -f "$VITE_PID_FILE"
    echo "Frontend pid file was empty. Removed."
    return 0
  fi

  if kill -0 "${pid}" 2>/dev/null; then
    kill -TERM -"${pid}" 2>/dev/null || kill "${pid}" 2>/dev/null || true
    for _ in {1..20}; do
      if kill -0 "${pid}" 2>/dev/null; then
        sleep 0.2
      else
        break
      fi
    done
    if kill -0 "${pid}" 2>/dev/null; then
      echo "Frontend did not stop, sending SIGKILL."
      kill -KILL -"${pid}" 2>/dev/null || kill -9 "${pid}" 2>/dev/null || true
    fi
  fi

  kill_port_owner "$FRONTEND_PORT" "$FRONTEND_DIR"

  rm -f "$VITE_PID_FILE"
  echo "Frontend stopped."
}

start_workspace() {
  echo "Starting workspace containers..."
  run_compose up -d --build
}

stop_workspace() {
  echo "Stopping workspace containers..."
  run_compose down
}

status() {
  echo "=== Workspace Containers ==="
  run_compose ps || true
  echo ""
  echo "=== Portal API ==="
  if [ -f "$PORTAL_PID_FILE" ]; then
    local pid
    pid="$(cat "$PORTAL_PID_FILE" 2>/dev/null || true)"
    if [ -n "${pid}" ] && kill -0 "${pid}" 2>/dev/null; then
      echo "Running (pid ${pid}) - http://localhost:$PORTAL_PORT"
    else
      echo "Not running."
    fi
  else
    echo "Not running."
  fi
  echo ""
  echo "=== Frontend ==="
  if [ -f "$VITE_PID_FILE" ]; then
    local pid
    pid="$(cat "$VITE_PID_FILE" 2>/dev/null || true)"
    if [ -n "${pid}" ] && kill -0 "${pid}" 2>/dev/null; then
      echo "Running (pid ${pid}) - http://localhost:$FRONTEND_PORT"
    else
      echo "Not running."
    fi
  else
    echo "Not running."
  fi
}

usage() {
  cat <<EOF
Usage: $(basename "$0") <command>

Commands:
  start       Start all services (workspace, portal API, frontend)
  stop        Stop all services
  restart     Restart all services
  status      Show status of all services

  portal      Start portal API only
  frontend    Start frontend only
  workspace   Start workspace containers only
EOF
}

command="${1:-}"
case "${command}" in
  start)
    start_workspace
    start_portal
    start_frontend
    echo ""
    echo "All services started!"
    echo "  Frontend: http://localhost:$FRONTEND_PORT"
    echo "  API:      http://localhost:$PORTAL_PORT"
    ;;
  stop|shutdown)
    stop_frontend
    stop_portal
    stop_workspace
    ;;
  restart)
    stop_frontend
    stop_portal
    stop_workspace
    start_workspace
    start_portal
    start_frontend
    ;;
  status)
    status
    ;;
  portal)
    start_portal
    ;;
  frontend)
    start_frontend
    ;;
  workspace)
    start_workspace
    ;;
  *)
    usage
    exit 1
    ;;
esac
