#!/usr/bin/env bash

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
APP_SRC="$ROOT/WindsurfAPI"

if [[ ! -d "$APP_SRC" ]]; then
  echo "WindsurfAPI source not found: $APP_SRC" >&2
  exit 1
fi

REMOTE_HOST="${REMOTE_HOST:?set REMOTE_HOST, e.g. 198.13.35.85}"
REMOTE_USER="${REMOTE_USER:-root}"
SSH_KEY="${SSH_KEY:-$HOME/.ssh/tencent_hk_mac}"
REMOTE_BASE="${REMOTE_BASE:-/opt/windsurf-api}"
REMOTE_PORT="${REMOTE_PORT:-3003}"
REMOTE_LS_PORT="${REMOTE_LS_PORT:-42100}"
REMOTE_PUBLISH_HOST="${REMOTE_PUBLISH_HOST:-127.0.0.1}"
CONTAINER_HOST="${CONTAINER_HOST:-0.0.0.0}"

if ! command -v openssl >/dev/null 2>&1; then
  echo "openssl is required to generate secrets" >&2
  exit 1
fi

WINDSURF_API_KEY="${WINDSURF_API_KEY:-$(openssl rand -hex 24)}"
DASHBOARD_PASSWORD="${DASHBOARD_PASSWORD:-$(openssl rand -base64 24 | tr -dc 'A-Za-z0-9' | head -c 24)}"

SSH_BASE=(ssh -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new -i "$SSH_KEY")
SCP_BASE=(scp -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new -i "$SSH_KEY")

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

ARCHIVE="$TMP_DIR/windsurf-api-src.tar.gz"

export COPYFILE_DISABLE=1

tar \
  --exclude='.git' \
  --exclude='.DS_Store' \
  --exclude='.data' \
  --exclude='.env' \
  --exclude='node_modules' \
  -C "$APP_SRC" \
  -czf "$ARCHIVE" .

REMOTE_ARCHIVE="/tmp/windsurf-api-src-$(date +%s).tar.gz"

echo "==> upload WindsurfAPI source"
"${SCP_BASE[@]}" "$ARCHIVE" "${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_ARCHIVE}"

echo "==> deploy on Vultr"
"${SSH_BASE[@]}" "${REMOTE_USER}@${REMOTE_HOST}" \
  "WINDSURF_API_KEY='$WINDSURF_API_KEY' DASHBOARD_PASSWORD='$DASHBOARD_PASSWORD' REMOTE_BASE='$REMOTE_BASE' REMOTE_PORT='$REMOTE_PORT' REMOTE_LS_PORT='$REMOTE_LS_PORT' REMOTE_PUBLISH_HOST='$REMOTE_PUBLISH_HOST' CONTAINER_HOST='$CONTAINER_HOST' REMOTE_ARCHIVE='$REMOTE_ARCHIVE' bash -s" <<'EOF'
set -euo pipefail

APP_DIR="$REMOTE_BASE/app"
DATA_DIR="$REMOTE_BASE/data"
OPT_DIR="$REMOTE_BASE/opt/windsurf"
TMP_DIR="$REMOTE_BASE/tmp"

mkdir -p "$APP_DIR" "$DATA_DIR" "$OPT_DIR" "$TMP_DIR"

rm -rf "$APP_DIR"/*
tar -xzf "$REMOTE_ARCHIVE" -C "$APP_DIR"
rm -f "$REMOTE_ARCHIVE"

cat > "$APP_DIR/.env" <<ENVEOF
PORT=${REMOTE_PORT}
HOST=${CONTAINER_HOST}
API_KEY=${WINDSURF_API_KEY}
DATA_DIR=/data
LS_BINARY_PATH=/opt/windsurf/language_server_linux_x64
LS_PORT=${REMOTE_LS_PORT}
DASHBOARD_PASSWORD=${DASHBOARD_PASSWORD}
CODEIUM_API_URL=https://server.self-serve.windsurf.com
DEFAULT_MODEL=claude-4.5-sonnet-thinking
MAX_TOKENS=8192
LOG_LEVEL=info
ENVEOF

cat > "$APP_DIR/compose.local.yml" <<COMPOSEEOF
services:
  windsurf-api:
    container_name: windsurf-api
    build:
      context: .
      dockerfile: Dockerfile
    restart: unless-stopped
    init: true
    env_file:
      - .env
    ports:
      - "${REMOTE_PUBLISH_HOST}:${REMOTE_PORT}:3003"
    volumes:
      - ${DATA_DIR}:/data
      - ${OPT_DIR}:/opt/windsurf
      - ${TMP_DIR}:/tmp/windsurf-workspace
COMPOSEEOF

cd "$APP_DIR"
docker compose -f compose.local.yml up -d --build

for _ in $(seq 1 30); do
  if curl -fsS "http://${REMOTE_PUBLISH_HOST}:${REMOTE_PORT}/health" >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

curl -fsS "http://${REMOTE_PUBLISH_HOST}:${REMOTE_PORT}/health" >/dev/null
EOF

echo "==> done"
echo "REMOTE_BASE=$REMOTE_BASE"
echo "REMOTE_PORT=$REMOTE_PORT"
echo "REMOTE_PUBLISH_HOST=$REMOTE_PUBLISH_HOST"
echo "CONTAINER_HOST=$CONTAINER_HOST"
