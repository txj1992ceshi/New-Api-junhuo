#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUNTIME_DIR="${ROOT_DIR}/runtime/local-pools"
LAUNCH_AGENTS_DIR="${HOME}/Library/LaunchAgents"
UID_VALUE="$(id -u)"
ACTION="${1:-status}"
TARGET="${2:-all}"

mkdir -p "${RUNTIME_DIR}" "${LAUNCH_AGENTS_DIR}"

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

provider_label() {
  echo "com.jj.junhuo.local-pool.$1"
}

provider_env_file() {
  echo "${RUNTIME_DIR}/$1.env"
}

provider_out_log() {
  echo "${RUNTIME_DIR}/$1-launchd.out"
}

provider_err_log() {
  echo "${RUNTIME_DIR}/$1-launchd.err"
}

provider_plist() {
  echo "${LAUNCH_AGENTS_DIR}/$(provider_label "$1").plist"
}

resolve_targets() {
  if [[ "${TARGET}" == "all" ]]; then
    printf '%s\n' "${providers[@]}"
    return
  fi
  printf '%s\n' "${TARGET}"
}

write_plist() {
  local provider="$1"
  local label plist script env_file out_log err_log
  label="$(provider_label "${provider}")"
  plist="$(provider_plist "${provider}")"
  script="$(provider_script "${provider}")"
  env_file="$(provider_env_file "${provider}")"
  out_log="$(provider_out_log "${provider}")"
  err_log="$(provider_err_log "${provider}")"

  cat > "${plist}" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>${label}</string>

  <key>ProgramArguments</key>
  <array>
    <string>/bin/bash</string>
    <string>-lc</string>
    <string>cd '${ROOT_DIR}'; if [ -f '${env_file}' ]; then source '${env_file}'; fi; exec bash '${script}'</string>
  </array>

  <key>WorkingDirectory</key>
  <string>${ROOT_DIR}</string>

  <key>RunAtLoad</key>
  <true/>

  <key>KeepAlive</key>
  <true/>

  <key>StandardOutPath</key>
  <string>${out_log}</string>

  <key>StandardErrorPath</key>
  <string>${err_log}</string>
</dict>
</plist>
EOF

  echo "[${provider}] wrote ${plist}"
}

bootout_if_present() {
  local label="$1"
  launchctl bootout "gui/${UID_VALUE}/${label}" >/dev/null 2>&1 || true
}

install_one() {
  local provider="$1"
  local label plist
  label="$(provider_label "${provider}")"
  plist="$(provider_plist "${provider}")"
  write_plist "${provider}"
  bootout_if_present "${label}"
  launchctl bootstrap "gui/${UID_VALUE}" "${plist}"
  launchctl kickstart -k "gui/${UID_VALUE}/${label}"
  echo "[${provider}] launchd installed and started"
}

uninstall_one() {
  local provider="$1"
  local label plist
  label="$(provider_label "${provider}")"
  plist="$(provider_plist "${provider}")"
  bootout_if_present "${label}"
  rm -f "${plist}"
  echo "[${provider}] launchd removed"
}

start_one() {
  local provider="$1"
  local label plist
  label="$(provider_label "${provider}")"
  plist="$(provider_plist "${provider}")"
  if [[ ! -f "${plist}" ]]; then
    install_one "${provider}"
    return
  fi
  launchctl print "gui/${UID_VALUE}/${label}" >/dev/null 2>&1 || launchctl bootstrap "gui/${UID_VALUE}" "${plist}"
  launchctl kickstart -k "gui/${UID_VALUE}/${label}"
  echo "[${provider}] launchd started"
}

stop_one() {
  local provider="$1"
  local label
  label="$(provider_label "${provider}")"
  bootout_if_present "${label}"
  echo "[${provider}] launchd stopped"
}

status_one() {
  local provider="$1"
  local label
  label="$(provider_label "${provider}")"
  echo "[${provider}]"
  if launchctl print "gui/${UID_VALUE}/${label}" >/dev/null 2>&1; then
    launchctl print "gui/${UID_VALUE}/${label}" | sed -n '1,80p'
  else
    echo "not loaded"
  fi
}

logs_one() {
  local provider="$1"
  echo "[${provider}] stdout"
  tail -n 40 "$(provider_out_log "${provider}")" 2>/dev/null || true
  echo
  echo "[${provider}] stderr"
  tail -n 40 "$(provider_err_log "${provider}")" 2>/dev/null || true
}

case "${ACTION}" in
  install)
    while IFS= read -r provider; do
      install_one "${provider}"
    done < <(resolve_targets)
    ;;
  uninstall)
    while IFS= read -r provider; do
      uninstall_one "${provider}"
    done < <(resolve_targets)
    ;;
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
  *)
    echo "usage: $0 {install|uninstall|start|stop|restart|status|logs} [cursor|windsurf|kiro|all]" >&2
    exit 1
    ;;
esac
