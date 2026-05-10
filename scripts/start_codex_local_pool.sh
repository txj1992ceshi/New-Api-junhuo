#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

export PROVIDER=codex
export PORT="${PORT:-3601}"
export POOL_LISTEN_HOST="${POOL_LISTEN_HOST:-127.0.0.1}"
export API_KEY="${API_KEY:-demo-codex-key}"
export DASHBOARD_PASSWORD="${DASHBOARD_PASSWORD:-demo-codex-dashboard}"
export DATA_DIR="${DATA_DIR:-$ROOT_DIR/runtime/local-pools}"
export SNAPSHOT_PATH="${SNAPSHOT_PATH:-}"
export CODEX_AUTH_STRATEGY="${CODEX_AUTH_STRATEGY:-provider_bridge}"
export INFERENCE_MODE="${INFERENCE_MODE:-dual}"
export CURSORPRO4_LICENSE_STATUS="${CURSORPRO4_LICENSE_STATUS:-activated}"
export CURSORPRO4_BRIDGE_BASE_URL="${CURSORPRO4_BRIDGE_BASE_URL:-http://127.0.0.1:8327}"

exec node "$ROOT_DIR/local-pool-service/server.mjs"
