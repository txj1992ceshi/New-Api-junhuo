#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
MANAGER_DIR="$ROOT_DIR/windsurf-manager"
STATE_DIR="$MANAGER_DIR/control_state"
TUNNEL_PID_FILE="$STATE_DIR/windsurf-tunnel.pid"
CONTROL_PID_FILE="$STATE_DIR/windsurf-control.pid"

stop_from_pid_file() {
  local label="$1"
  local pid_file="$2"
  if [[ ! -f "$pid_file" ]]; then
    echo "$label is not recorded."
    return
  fi
  local pid
  pid="$(cat "$pid_file" 2>/dev/null || true)"
  if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
    kill "$pid" 2>/dev/null || true
    echo "Stopped $label (pid $pid)"
  else
    echo "$label was not running."
  fi
  rm -f "$pid_file"
}

stop_from_pid_file "Windsurf control server" "$CONTROL_PID_FILE"
stop_from_pid_file "Windsurf SSH tunnel" "$TUNNEL_PID_FILE"
