#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

export PROVIDER=codex
export PORT="${PORT:-3601}"
export API_KEY="${API_KEY:-demo-codex-key}"
export DASHBOARD_PASSWORD="${DASHBOARD_PASSWORD:-demo-codex-dashboard}"
export DATA_DIR="${DATA_DIR:-$ROOT_DIR/runtime/local-pools}"
export SNAPSHOT_PATH="${SNAPSHOT_PATH:-}"
export CODEX_AUTH_STRATEGY="${CODEX_AUTH_STRATEGY:-provider_bridge}"
export INFERENCE_MODE="${INFERENCE_MODE:-dual}"

exec node "$ROOT_DIR/local-pool-service/server.mjs"
