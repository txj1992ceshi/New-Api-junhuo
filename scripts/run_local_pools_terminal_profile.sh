#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUNTIME_DIR="${ROOT_DIR}/runtime/local-pools"
CHECK_INTERVAL="${CHECK_INTERVAL:-5}"
RESTART_DELAY="${RESTART_DELAY:-2}"

mkdir -p "${RUNTIME_DIR}"

timestamp() {
  date '+%Y-%m-%d %H:%M:%S'
}

is_healthy() {
  local port="$1"
  local api_key="$2"
  curl -fsS "http://127.0.0.1:${port}/healthz" >/dev/null 2>&1 &&
    curl -fsS -H "Authorization: Bearer ${api_key}" "http://127.0.0.1:${port}/auth/status" >/dev/null 2>&1
}

watch_cursor() {
  local out_file="${RUNTIME_DIR}/cursor-terminal.out"
  local err_file="${RUNTIME_DIR}/cursor-terminal.err"
  while true; do
    if is_healthy 3401 "demo-cursor-key"; then
      sleep "${CHECK_INTERVAL}"
      continue
    fi
    echo "[$(timestamp)] [cursor] restarting direct local pool" >>"${out_file}"
    (
      cd "${ROOT_DIR}"
      export CURSOR_PROVIDER_MODE=direct
      export CURSOR_DIRECT_PROTOCOL=connect
      export CURSOR_DIRECT_BASE_URL='https://api2.cursor.sh'
      export CURSOR_CONNECT_MODELS_PATH='/agent.v1.AgentService/GetUsableModels'
      export CURSOR_CONNECT_RESPONSES_PATH='/agent.v1.AgentService/Run'
      export CURSOR_CONNECT_CHAT_COMPLETIONS_PATH='/agent.v1.AgentService/Run'
      export CURSOR_CONNECT_PAYLOAD_MODE='agent_run'
      export CURSOR_AUTH_STRATEGY=local_state_direct
      export INFERENCE_MODE=responses
      exec bash "${ROOT_DIR}/scripts/start_cursor_local_pool.sh"
    ) >>"${out_file}" 2>>"${err_file}" || true
    sleep "${RESTART_DELAY}"
  done
}

watch_windsurf() {
  local out_file="${RUNTIME_DIR}/windsurf-terminal.out"
  local err_file="${RUNTIME_DIR}/windsurf-terminal.err"
  while true; do
    if curl -fsS -H 'Authorization: Bearer demo-windsurf-key' "http://127.0.0.1:3003/auth/status" >/dev/null 2>&1; then
      sleep "${CHECK_INTERVAL}"
      continue
    fi
    echo "[$(timestamp)] [windsurf] restarting WindsurfAPI main pool" >>"${out_file}"
    (
      cd "${ROOT_DIR}/WindsurfAPI"
      set -a
      source ./.env
      set +a
      export PORT=3003
      export API_KEY='demo-windsurf-key'
      export DASHBOARD_PASSWORD='demo-windsurf-dashboard'
      exec node src/index.js
    ) >>"${out_file}" 2>>"${err_file}" || true
    sleep "${RESTART_DELAY}"
  done
}

watch_kiro() {
  local out_file="${RUNTIME_DIR}/kiro-terminal.out"
  local err_file="${RUNTIME_DIR}/kiro-terminal.err"
  while true; do
    if is_healthy 3501 "demo-kiro-key"; then
      sleep "${CHECK_INTERVAL}"
      continue
    fi
    echo "[$(timestamp)] [kiro] restarting local pool" >>"${out_file}"
    (
      cd "${ROOT_DIR}"
      export KIRO_AUTH_STRATEGY=local_state_direct
      export INFERENCE_MODE=responses
      exec bash "${ROOT_DIR}/scripts/start_kiro_local_pool.sh"
    ) >>"${out_file}" 2>>"${err_file}" || true
    sleep "${RESTART_DELAY}"
  done
}

cleanup() {
  kill 0 2>/dev/null || true
}

trap cleanup EXIT INT TERM

echo "[$(timestamp)] terminal profile supervisor starting"

watch_cursor &
watch_windsurf &
watch_kiro &

wait
