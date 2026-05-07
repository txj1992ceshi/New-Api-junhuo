#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUNTIME_DIR="${ROOT_DIR}/runtime/local-pools"
ACTION="${1:-status}"
TARGET="${2:-all}"
SESSION_NAME="${SESSION_NAME:-junhuo-local-pools}"
SUPERVISOR_SCRIPT="${ROOT_DIR}/scripts/run_local_pools_terminal_profile.sh"

mkdir -p "${RUNTIME_DIR}"

screen_exists() {
  screen -ls 2>/dev/null || true
}

session_exists() {
  local output
  output="$(screen -ls 2>/dev/null || true)"
  grep -q "[.]${SESSION_NAME}[[:space:]]" <<<"${output}"
}

case "${ACTION}" in
  start)
    if session_exists; then
      echo "screen session already exists: ${SESSION_NAME}"
    else
      screen -dmS "${SESSION_NAME}" bash -lc "cd '${ROOT_DIR}' && exec bash '${SUPERVISOR_SCRIPT}'"
      echo "started screen session: ${SESSION_NAME}"
    fi
    ;;
  stop)
    if session_exists; then
      screen -S "${SESSION_NAME}" -X quit
      echo "stopped screen session: ${SESSION_NAME}"
    else
      echo "screen session not running: ${SESSION_NAME}"
    fi
    ;;
  restart)
    if session_exists; then
      screen -S "${SESSION_NAME}" -X quit
      sleep 1
    fi
    screen -dmS "${SESSION_NAME}" bash -lc "cd '${ROOT_DIR}' && exec bash '${SUPERVISOR_SCRIPT}'"
    echo "restarted screen session: ${SESSION_NAME}"
    ;;
  status)
    if session_exists; then
      echo "screen session running: ${SESSION_NAME}"
    else
      echo "screen session not running: ${SESSION_NAME}"
    fi
    bash "${ROOT_DIR}/scripts/manage_local_pools.sh" status
    ;;
  attach)
    exec screen -r "${SESSION_NAME}"
    ;;
  logs)
    for provider in cursor windsurf kiro; do
      echo "[${provider}] stdout"
      tail -n 30 "${RUNTIME_DIR}/${provider}-terminal.out" 2>/dev/null || true
      echo
      echo "[${provider}] stderr"
      tail -n 30 "${RUNTIME_DIR}/${provider}-terminal.err" 2>/dev/null || true
      echo
    done
    ;;
  validate)
    bash "${ROOT_DIR}/scripts/manage_local_pools.sh" validate
    ;;
  *)
    echo "usage: $0 {start|stop|restart|status|attach|logs|validate} [cursor|windsurf|kiro|all]" >&2
    exit 1
    ;;
esac
