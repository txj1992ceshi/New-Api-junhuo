#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

: "${MAC_TAILSCALE_IP:?missing MAC_TAILSCALE_IP}"
REMOTE_SSH="${REMOTE_SSH:-ssh -o IdentitiesOnly=yes -i ~/.ssh/tencent_hk_mac root@198.13.35.85}"
REMOTE_DB_PATH="${REMOTE_DB_PATH:-/opt/new-api/data/one-api.db}"
PUBLIC_CONTRACT_MODELS="${PUBLIC_CONTRACT_MODELS:-gpt-5.5,gpt-5.4}"

remote_apply_script=$(cat <<EOF
set -euo pipefail
cd /opt/new-api/src
export DB_PATH='${REMOTE_DB_PATH}'
export PUBLIC_CONTRACT_MODELS='${PUBLIC_CONTRACT_MODELS}'

export CURSOR_POOL_BASE_URL='http://${MAC_TAILSCALE_IP}:3401'
export CURSOR_POOL_API_KEY='${CURSOR_POOL_API_KEY:-demo-cursor-key}'
export CURSOR_POOL_TUNNEL_HINT='tailscale://${MAC_TAILSCALE_IP}:3401'
export CURSOR_POOL_AUTH_STRATEGY='${CURSOR_POOL_AUTH_STRATEGY:-local_state_direct}'
export CURSOR_POOL_MODE='${CURSOR_POOL_MODE:-local_state_direct}'

export WINDSURF_POOL_BASE_URL='http://${MAC_TAILSCALE_IP}:3003'
export WINDSURF_POOL_API_KEY='${WINDSURF_POOL_API_KEY:-demo-windsurf-key}'
export WINDSURF_POOL_TUNNEL_HINT='tailscale://${MAC_TAILSCALE_IP}:3003'
export WINDSURF_POOL_AUTH_STRATEGY='${WINDSURF_POOL_AUTH_STRATEGY:-local_state_direct}'
export WINDSURF_POOL_MODE='${WINDSURF_POOL_MODE:-external_managed}'

export KIRO_POOL_BASE_URL='http://${MAC_TAILSCALE_IP}:3501'
export KIRO_POOL_API_KEY='${KIRO_POOL_API_KEY:-demo-kiro-key}'
export KIRO_POOL_TUNNEL_HINT='tailscale://${MAC_TAILSCALE_IP}:3501'
export KIRO_POOL_AUTH_STRATEGY='${KIRO_POOL_AUTH_STRATEGY:-local_state_direct}'
export KIRO_POOL_MODE='${KIRO_POOL_MODE:-external_managed}'

export CODEX_POOL_BASE_URL='http://${MAC_TAILSCALE_IP}:3601'
export CODEX_POOL_API_KEY='${CODEX_POOL_API_KEY:-demo-codex-key}'
export CODEX_POOL_TUNNEL_HINT='tailscale://${MAC_TAILSCALE_IP}:3601'
export CODEX_POOL_AUTH_STRATEGY='${CODEX_POOL_AUTH_STRATEGY:-provider_bridge}'
export CODEX_POOL_MODE='${CODEX_POOL_MODE:-provider_bridge}'

bash scripts/external_pool_channels.sh apply
EOF
)

echo "Applying remote channel cutover on server via: ${REMOTE_SSH}"
printf '%s\n' "${remote_apply_script}" | eval "${REMOTE_SSH} 'bash -s'"
echo
echo "Done. Recommended next checks from server:"
echo "  curl -H 'Authorization: Bearer ${CURSOR_POOL_API_KEY:-demo-cursor-key}' http://${MAC_TAILSCALE_IP}:3401/auth/status"
echo "  curl -H 'Authorization: Bearer ${WINDSURF_POOL_API_KEY:-demo-windsurf-key}' http://${MAC_TAILSCALE_IP}:3003/auth/status"
echo "  curl -H 'Authorization: Bearer ${KIRO_POOL_API_KEY:-demo-kiro-key}' http://${MAC_TAILSCALE_IP}:3501/auth/status"
echo "  curl -H 'Authorization: Bearer ${CODEX_POOL_API_KEY:-demo-codex-key}' http://${MAC_TAILSCALE_IP}:3601/auth/status"
