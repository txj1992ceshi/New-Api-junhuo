#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
MANAGER_DIR="$ROOT_DIR/windsurf-manager"
STATE_DIR="$MANAGER_DIR/control_state"
LOG_DIR="$STATE_DIR/logs"
TUNNEL_PID_FILE="$STATE_DIR/windsurf-tunnel.pid"
CONTROL_PID_FILE="$STATE_DIR/windsurf-control.pid"
TUNNEL_LOG="$LOG_DIR/windsurf-tunnel.log"
CONTROL_LOG="$LOG_DIR/windsurf-control.log"

REMOTE_HOST="${REMOTE_HOST:-198.13.35.85}"
REMOTE_USER="${REMOTE_USER:-root}"
SSH_KEY="${SSH_KEY:-$HOME/.ssh/tencent_hk_mac}"
LOCAL_PORT="${LOCAL_PORT:-3003}"
REMOTE_PORT="${REMOTE_PORT:-3003}"
CONTROL_PORT="${CONTROL_PORT:-3310}"

mkdir -p "$LOG_DIR" "$MANAGER_DIR/auth_output"

is_pid_running() {
  local pid="${1:-}"
  [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null
}

start_tunnel() {
  if [[ -f "$TUNNEL_PID_FILE" ]]; then
    local pid
    pid="$(cat "$TUNNEL_PID_FILE" 2>/dev/null || true)"
    if is_pid_running "$pid"; then
      echo "Windsurf SSH tunnel already running (pid $pid)"
      return
    fi
    rm -f "$TUNNEL_PID_FILE"
  fi

  nohup ssh \
    -o IdentitiesOnly=yes \
    -o StrictHostKeyChecking=accept-new \
    -o ExitOnForwardFailure=yes \
    -o ServerAliveInterval=30 \
    -o ServerAliveCountMax=3 \
    -i "$SSH_KEY" \
    -N \
    -L "${LOCAL_PORT}:127.0.0.1:${REMOTE_PORT}" \
    "${REMOTE_USER}@${REMOTE_HOST}" \
    >"$TUNNEL_LOG" 2>&1 </dev/null &

  local pid=$!
  echo "$pid" >"$TUNNEL_PID_FILE"
  sleep 2
  if ! is_pid_running "$pid"; then
    echo "Failed to start Windsurf SSH tunnel. Check $TUNNEL_LOG" >&2
    exit 1
  fi
  echo "Windsurf SSH tunnel started (pid $pid)"
}

start_control() {
  if [[ ! -x "$MANAGER_DIR/.venv/bin/python" ]]; then
    echo "Missing venv python: $MANAGER_DIR/.venv/bin/python" >&2
    exit 1
  fi

  if lsof -i "tcp:${CONTROL_PORT}" -n -P 2>/dev/null | grep -q "LISTEN"; then
    local existing_pid
    existing_pid="$(lsof -ti "tcp:${CONTROL_PORT}" -sTCP:LISTEN 2>/dev/null | head -n 1 || true)"
    if [[ -n "$existing_pid" ]]; then
      echo "$existing_pid" >"$CONTROL_PID_FILE"
      echo "Windsurf control server already listening on ${CONTROL_PORT} (pid $existing_pid)"
      return
    fi
  fi

  if [[ -f "$CONTROL_PID_FILE" ]]; then
    local pid
    pid="$(cat "$CONTROL_PID_FILE" 2>/dev/null || true)"
    if is_pid_running "$pid"; then
      echo "Windsurf control server already running (pid $pid)"
      return
    fi
    rm -f "$CONTROL_PID_FILE"
  fi

  nohup "$MANAGER_DIR/.venv/bin/python" "$MANAGER_DIR/control_server.py" \
    >"$CONTROL_LOG" 2>&1 </dev/null &

  local pid=$!
  echo "$pid" >"$CONTROL_PID_FILE"
  sleep 2
  if ! is_pid_running "$pid"; then
    echo "Failed to start Windsurf control server. Check $CONTROL_LOG" >&2
    exit 1
  fi
  echo "Windsurf control server started (pid $pid)"
}

check_health() {
  local health_url="http://127.0.0.1:${CONTROL_PORT}/health"
  if command -v curl >/dev/null 2>&1; then
    echo
    echo "Control health:"
    curl -sS "$health_url" || true
    echo
  fi
}

start_tunnel
start_control

echo
echo "Windsurf local stack is ready."
echo "Control panel: http://127.0.0.1:${CONTROL_PORT}"
echo "Tunnel log:    $TUNNEL_LOG"
echo "Control log:   $CONTROL_LOG"

check_health
