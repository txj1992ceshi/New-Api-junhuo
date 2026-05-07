#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

export PROVIDER=kiro
export PORT="${PORT:-3501}"
export API_KEY="${API_KEY:-demo-kiro-key}"
export DASHBOARD_PASSWORD="${DASHBOARD_PASSWORD:-demo-kiro-dashboard}"
export DATA_DIR="${DATA_DIR:-$ROOT_DIR/runtime/local-pools}"
export SNAPSHOT_PATH="${SNAPSHOT_PATH:-}"
export KIRO_AUTH_STRATEGY="${KIRO_AUTH_STRATEGY:-local_state_direct}"
export INFERENCE_MODE="${INFERENCE_MODE:-responses}"

exec node "$ROOT_DIR/local-pool-service/server.mjs"
