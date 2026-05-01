#!/bin/zsh

set -euo pipefail

APP_DIR="/Users/jj/Documents/Playground/local-runs/new-api"
BIN_PATH="$APP_DIR/bin/local-new-api"
LOG_DIR="$APP_DIR/logs"

mkdir -p "$LOG_DIR"
cd "$APP_DIR"

exec env GIN_MODE=release "$BIN_PATH" --port 3000 >>"$LOG_DIR/local-run.out" 2>>"$LOG_DIR/local-run.err"
