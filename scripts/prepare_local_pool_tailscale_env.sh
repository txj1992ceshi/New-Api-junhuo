#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUNTIME_DIR="${ROOT_DIR}/runtime/local-pools"
MODE="${1:-show}"
TAILSCALE_BIND_HOST="${TAILSCALE_BIND_HOST:-0.0.0.0}"

mkdir -p "${RUNTIME_DIR}"

write_env() {
  local provider="$1"
  shift
  local env_file="${RUNTIME_DIR}/${provider}.env"
  : >"${env_file}"
  while [[ "$#" -gt 0 ]]; do
    printf '%s\n' "$1" >>"${env_file}"
    shift
  done
  echo "wrote ${env_file}"
}

show_example() {
  cat <<EOF
Use these env files before restarting the local pools:

  cursor  -> ${RUNTIME_DIR}/cursor.env
  windsurf -> ${RUNTIME_DIR}/windsurf.env
  kiro    -> ${RUNTIME_DIR}/kiro.env
  codex   -> ${RUNTIME_DIR}/codex.env

Suggested bind host:
  TAILSCALE_BIND_HOST=${TAILSCALE_BIND_HOST}

Example content:
  POOL_LISTEN_HOST=${TAILSCALE_BIND_HOST}

For Windsurf add:
  HOST=${TAILSCALE_BIND_HOST}
  BIND_HOST=${TAILSCALE_BIND_HOST}
EOF
}

apply_env() {
  write_env cursor \
    "POOL_LISTEN_HOST=${TAILSCALE_BIND_HOST}"
  write_env kiro \
    "POOL_LISTEN_HOST=${TAILSCALE_BIND_HOST}"
  write_env codex \
    "POOL_LISTEN_HOST=${TAILSCALE_BIND_HOST}"
  write_env windsurf \
    "POOL_LISTEN_HOST=${TAILSCALE_BIND_HOST}" \
    "HOST=${TAILSCALE_BIND_HOST}" \
    "BIND_HOST=${TAILSCALE_BIND_HOST}"
}

case "${MODE}" in
  show)
    show_example
    ;;
  apply)
    apply_env
    ;;
  *)
    echo "usage: $0 [show|apply]" >&2
    exit 1
    ;;
esac
