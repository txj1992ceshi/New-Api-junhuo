#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

export PROVIDER="${PROVIDER:-windsurf}"
export PORT="${PORT:-3003}"
export API_KEY="${API_KEY:-demo-windsurf-key}"
export DASHBOARD_PASSWORD="${DASHBOARD_PASSWORD:-demo-windsurf-dashboard}"
export DATA_DIR="${DATA_DIR:-$ROOT_DIR/runtime/local-pools}"
export SNAPSHOT_PATH="${SNAPSHOT_PATH:-}"
export WINDSURF_AUTH_STRATEGY="${WINDSURF_AUTH_STRATEGY:-local_state_direct}"
export INFERENCE_MODE="${INFERENCE_MODE:-responses}"

exec node "$ROOT_DIR/local-pool-service/server.mjs"
