#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUNTIME_DIR="${ROOT_DIR}/runtime/local-pools"
ACTION="${1:-status}"
TARGET="${2:-all}"

mkdir -p "${RUNTIME_DIR}"

providers=(cursor windsurf kiro)

provider_port() {
  case "$1" in
    cursor) echo 3401 ;;
    windsurf) echo 3003 ;;
    kiro) echo 3501 ;;
    *)
      echo "unknown provider: $1" >&2
      return 1
      ;;
  esac
}

provider_key() {
  case "$1" in
    cursor) echo "demo-cursor-key" ;;
    windsurf) echo "demo-windsurf-key" ;;
    kiro) echo "demo-kiro-key" ;;
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
    *)
      echo "unknown provider: $1" >&2
      return 1
      ;;
  esac
}

provider_log_out() {
  echo "${RUNTIME_DIR}/$1-service.out"
}

provider_log_err() {
  echo "${RUNTIME_DIR}/$1-service.err"
}

provider_pid_file() {
  echo "${RUNTIME_DIR}/$1-service.pid"
}

provider_env_file() {
  echo "${RUNTIME_DIR}/$1.env"
}

print_line() {
  printf '%-18s %s\n' "$1" "$2"
}

resolve_targets() {
  if [[ "${TARGET}" == "all" ]]; then
    printf '%s\n' "${providers[@]}"
    return
  fi
  printf '%s\n' "${TARGET}"
}

load_env_file() {
  local provider="$1"
  local env_file
  env_file="$(provider_env_file "${provider}")"
  if [[ -f "${env_file}" ]]; then
    # shellcheck disable=SC1090
    source "${env_file}"
  fi
}

is_pid_alive() {
  local pid="$1"
  [[ -n "${pid}" ]] && kill -0 "${pid}" 2>/dev/null
}

pid_from_file() {
  local provider="$1"
  local pid_file
  pid_file="$(provider_pid_file "${provider}")"
  if [[ -f "${pid_file}" ]]; then
    tr -d '[:space:]' < "${pid_file}"
  fi
}

port_pid() {
  local port="$1"
  lsof -tiTCP:"${port}" -sTCP:LISTEN 2>/dev/null | head -n 1 || true
}

wait_for_health() {
  local provider="$1"
  local port="$2"
  local api_key="$3"
  local url="http://127.0.0.1:${port}/healthz"
  local status_url="http://127.0.0.1:${port}/auth/status"
  local i
  for i in {1..20}; do
    if curl -fsS "${url}" >/dev/null 2>&1 && \
      curl -fsS -H "Authorization: Bearer ${api_key}" "${status_url}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "[${provider}] health check timed out" >&2
  return 1
}

start_one() {
  local provider="$1"
  local script port pid_file out_file err_file api_key existing_pid env_file launch_cmd
  script="$(provider_script "${provider}")"
  port="$(provider_port "${provider}")"
  pid_file="$(provider_pid_file "${provider}")"
  out_file="$(provider_log_out "${provider}")"
  err_file="$(provider_log_err "${provider}")"
  api_key="$(provider_key "${provider}")"
  env_file="$(provider_env_file "${provider}")"
  existing_pid="$(port_pid "${port}")"

  if [[ -n "${existing_pid}" ]] && is_pid_alive "${existing_pid}"; then
    echo "[${provider}] already listening on ${port} (pid=${existing_pid})"
    echo "${existing_pid}" > "${pid_file}"
    return 0
  fi

  launch_cmd="cd '${ROOT_DIR}'; "
  if [[ -f "${env_file}" ]]; then
    launch_cmd="${launch_cmd}source '${env_file}'; "
  fi
  launch_cmd="${launch_cmd}nohup bash '${script}' >>'${out_file}' 2>>'${err_file}' < /dev/null &!; sleep 1"
  zsh -ic "${launch_cmd}"

  wait_for_health "${provider}" "${port}" "${api_key}"
  echo "$(port_pid "${port}")" > "${pid_file}"
  echo "[${provider}] started on ${port} (pid=$(pid_from_file "${provider}"))"
}

stop_one() {
  local provider="$1"
  local pid port pid_file
  pid_file="$(provider_pid_file "${provider}")"
  pid="$(pid_from_file "${provider}")"
  port="$(provider_port "${provider}")"

  if [[ -n "${pid}" ]] && is_pid_alive "${pid}"; then
    kill "${pid}" 2>/dev/null || true
    sleep 1
  fi

  local listener_pid
  listener_pid="$(port_pid "${port}")"
  if [[ -n "${listener_pid}" ]] && is_pid_alive "${listener_pid}"; then
    kill "${listener_pid}" 2>/dev/null || true
    sleep 1
  fi

  rm -f "${pid_file}"
  echo "[${provider}] stopped"
}

status_one() {
  local provider="$1"
  local port pid listener_pid api_key auth_json
  port="$(provider_port "${provider}")"
  pid="$(pid_from_file "${provider}")"
  listener_pid="$(port_pid "${port}")"
  api_key="$(provider_key "${provider}")"

  echo "[${provider}]"
  print_line port "${port}"
  print_line pid_file "${pid:-none}"
  print_line listener_pid "${listener_pid:-none}"

  if [[ -n "${listener_pid}" ]]; then
    auth_json="$(curl -fsS -H "Authorization: Bearer ${api_key}" "http://127.0.0.1:${port}/auth/status" 2>/dev/null || true)"
    print_line health "up"
    print_line auth_status "${auth_json:-unavailable}"
  else
    print_line health "down"
  fi
}

logs_one() {
  local provider="$1"
  echo "[${provider}] stdout"
  tail -n 40 "$(provider_log_out "${provider}")" 2>/dev/null || true
  echo
  echo "[${provider}] stderr"
  tail -n 40 "$(provider_log_err "${provider}")" 2>/dev/null || true
}

validate_all() {
  bash "${ROOT_DIR}/scripts/validate_external_pool_channels.sh"
}

case "${ACTION}" in
  start)
    while IFS= read -r provider; do
      start_one "${provider}"
    done < <(resolve_targets)
    ;;
  stop)
    while IFS= read -r provider; do
      stop_one "${provider}"
    done < <(resolve_targets)
    ;;
  restart)
    while IFS= read -r provider; do
      stop_one "${provider}"
      start_one "${provider}"
    done < <(resolve_targets)
    ;;
  status)
    while IFS= read -r provider; do
      status_one "${provider}"
      echo
    done < <(resolve_targets)
    ;;
  logs)
    while IFS= read -r provider; do
      logs_one "${provider}"
      echo
    done < <(resolve_targets)
    ;;
  validate)
    validate_all
    ;;
  *)
    echo "usage: $0 {start|stop|restart|status|logs|validate} [cursor|windsurf|kiro|all]" >&2
    exit 1
    ;;
esac
