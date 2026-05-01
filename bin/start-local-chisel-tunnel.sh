#!/bin/zsh
set -euo pipefail

ROOT_DIR="/Users/jj/Documents/Playground/local-runs/new-api"
ENV_FILE="$ROOT_DIR/config/junhuo-chisel.env"
LOG_DIR="$ROOT_DIR/logs"
CHISEL_BIN="$ROOT_DIR/bin/chisel"

mkdir -p "$LOG_DIR"

if [[ -f "$ENV_FILE" ]]; then
  source "$ENV_FILE"
fi

exec "$CHISEL_BIN" client \
  --keepalive 25s \
  --auth "$CHISEL_AUTH" \
  "$CHISEL_SERVER_URL" \
  "$CHISEL_REMOTE"
