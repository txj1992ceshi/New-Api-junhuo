#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUNTIME_DIR="${ROOT_DIR}/runtime/local-pools"
TARGET="${1:-all}"
CHECK_INTERVAL="${CHECK_INTERVAL:-5}"
RESTART_DELAY="${RESTART_DELAY:-2}"

mkdir -p "${RUNTIME_DIR}"

providers=(cursor windsurf kiro codex)
watcher_pids=()

resolve_targets() {
  if [[ "${TARGET}" == "all" ]]; then
    printf '%s\n' "${providers[@]}"
    return
  fi
  printf '%s\n' "${TARGET}"
}

provider_port() {
  case "$1" in
    cursor) echo 3401 ;;
    windsurf) echo 3003 ;;
    kiro) echo 3501 ;;
    codex) echo 3601 ;;
    *)
      echo "unknown provider: $1" >&2
      return 1
      ;;
  esac
}

provider_script() {
  case "$1" in
    cursor) echo "${ROOT_DIR}/scripts/start_cursor_local_pool.sh" ;;
    windsurf) echo "${ROOT_DIR}/scripts/start_windsurf_local_pool.sh" ;;
    kiro) echo "${ROOT_DIR}/scripts/start_kiro_local_pool.sh" ;;
    codex) echo "${ROOT_DIR}/scripts/start_codex_local_pool.sh" ;;
    *)
      echo "unknown provider: $1" >&2
      return 1
      ;;
  esac
}

provider_api_key() {
  case "$1" in
    cursor) echo "demo-cursor-key" ;;
    windsurf) echo "demo-windsurf-key" ;;
    kiro) echo "demo-kiro-key" ;;
    codex) echo "demo-codex-key" ;;
    *)
      echo "unknown provider: $1" >&2
      return 1
      ;;
  esac
}

provider_log_out() {
  echo "${RUNTIME_DIR}/$1-supervisor.out"
}

provider_log_err() {
  echo "${RUNTIME_DIR}/$1-supervisor.err"
}

provider_env_file() {
  echo "${RUNTIME_DIR}/$1.env"
}

timestamp() {
  date '+%Y-%m-%d %H:%M:%S'
}

is_healthy() {
  local provider="$1"
  local port api_key
  port="$(provider_port "${provider}")"
  api_key="$(provider_api_key "${provider}")"
  curl -fsS "http://127.0.0.1:${port}/healthz" >/dev/null 2>&1 &&
    curl -fsS -H "Authorization: Bearer ${api_key}" "http://127.0.0.1:${port}/auth/status" >/dev/null 2>&1
}

run_provider_once() {
  local provider="$1"
  local script out_file err_file env_file
  script="$(provider_script "${provider}")"
  out_file="$(provider_log_out "${provider}")"
  err_file="$(provider_log_err "${provider}")"
  env_file="$(provider_env_file "${provider}")"

  (
    cd "${ROOT_DIR}"
    if [[ -f "${env_file}" ]]; then
      # shellcheck disable=SC1090
      source "${env_file}"
    fi
    if [[ -n "${LOCAL_POOL_START_COMMAND:-}" ]]; then
      echo "[$(timestamp)] [${provider}] launching custom command" >>"${out_file}"
      exec bash -lc "${LOCAL_POOL_START_COMMAND}" >>"${out_file}" 2>>"${err_file}"
    fi
    echo "[$(timestamp)] [${provider}] launching ${script}" >>"${out_file}"
    exec bash "${script}" >>"${out_file}" 2>>"${err_file}"
  )
}

watch_provider() {
  local provider="$1"
  local exit_code
  while true; do
    if is_healthy "${provider}"; then
      sleep "${CHECK_INTERVAL}"
      continue
    fi

    echo "[$(timestamp)] [${provider}] health check failed, starting/restarting" >>"$(provider_log_out "${provider}")"
    if run_provider_once "${provider}"; then
      echo "[$(timestamp)] [${provider}] process exited cleanly; will re-check" >>"$(provider_log_out "${provider}")"
    else
      exit_code=$?
      echo "[$(timestamp)] [${provider}] process exited with code ${exit_code}; retrying in ${RESTART_DELAY}s" >>"$(provider_log_err "${provider}")"
      sleep "${RESTART_DELAY}"
    fi
  done
}

cleanup() {
  local pid
  for pid in "${watcher_pids[@]:-}"; do
    kill "${pid}" 2>/dev/null || true
  done
  wait || true
}

trap cleanup EXIT INT TERM

echo "[$(timestamp)] local pool supervisor starting (target=${TARGET})"
while IFS= read -r provider; do
  watch_provider "${provider}" &
  watcher_pids+=($!)
done < <(resolve_targets)

wait
