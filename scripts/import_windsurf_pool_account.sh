#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DB_PATH="${DB_PATH:-$ROOT_DIR/one-api.db}"
CHANNEL_NAME="${CHANNEL_NAME:-windsurf-pool-proxy}"
BASE_URL="${WINDSURF_POOL_BASE_URL:-}"
CHANNEL_KEY="${WINDSURF_POOL_CHANNEL_KEY:-}"

usage() {
  cat <<'EOF'
Usage:
  bash scripts/import_windsurf_pool_account.sh --token <token> [--label <label>]
  bash scripts/import_windsurf_pool_account.sh --api-key <api_key> [--label <label>]
  bash scripts/import_windsurf_pool_account.sh --email <email> --password <password>
  bash scripts/import_windsurf_pool_account.sh --accounts-file <json-file>

Optional env:
  DB_PATH
  CHANNEL_NAME
  WINDSURF_POOL_BASE_URL
  WINDSURF_POOL_CHANNEL_KEY

Examples:
  bash scripts/import_windsurf_pool_account.sh --token 'ws_xxx'
  bash scripts/import_windsurf_pool_account.sh --api-key 'key_xxx' --label 'manual-1'
  bash scripts/import_windsurf_pool_account.sh --email 'me@example.com' --password 'secret'
EOF
}

require_tool() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "$1 not found"
    exit 1
  fi
}

resolve_channel_meta() {
  require_tool sqlite3
  local row
  row="$(sqlite3 -separator $'\t' "$DB_PATH" "select base_url, key from channels where name = '$CHANNEL_NAME' limit 1;")"
  if [[ -z "$row" ]]; then
    echo "channel not found: $CHANNEL_NAME"
    exit 1
  fi
  local db_base_url db_key
  IFS=$'\t' read -r db_base_url db_key <<< "$row"
  if [[ -z "$BASE_URL" ]]; then
    BASE_URL="$db_base_url"
  fi
  if [[ -z "$CHANNEL_KEY" ]]; then
    CHANNEL_KEY="$db_key"
  fi
  if [[ -z "$BASE_URL" || -z "$CHANNEL_KEY" ]]; then
    echo "missing base_url or channel key"
    exit 1
  fi
}

json_escape() {
  python3 - <<'PY' "$1"
import json, sys
print(json.dumps(sys.argv[1]))
PY
}

build_payload() {
  local token="$1"
  local api_key="$2"
  local email="$3"
  local password="$4"
  local label="$5"
  local accounts_file="$6"

  if [[ -n "$accounts_file" ]]; then
    python3 - <<'PY' "$accounts_file"
from pathlib import Path
import sys
print(Path(sys.argv[1]).read_text())
PY
    return
  fi

  if [[ -n "$token" ]]; then
    printf '{"token":%s' "$(json_escape "$token")"
    if [[ -n "$label" ]]; then
      printf ',"label":%s' "$(json_escape "$label")"
    fi
    printf '}'
    return
  fi

  if [[ -n "$api_key" ]]; then
    printf '{"api_key":%s' "$(json_escape "$api_key")"
    if [[ -n "$label" ]]; then
      printf ',"label":%s' "$(json_escape "$label")"
    fi
    printf '}'
    return
  fi

  if [[ -n "$email" && -n "$password" ]]; then
    printf '{"email":%s,"password":%s' "$(json_escape "$email")" "$(json_escape "$password")"
    if [[ -n "$label" ]]; then
      printf ',"label":%s' "$(json_escape "$label")"
    fi
    printf '}'
    return
  fi

  usage
  exit 1
}

TOKEN=""
ACCOUNT_API_KEY=""
EMAIL=""
PASSWORD=""
LABEL=""
ACCOUNTS_FILE=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --token)
      TOKEN="${2:-}"
      shift 2
      ;;
    --api-key)
      ACCOUNT_API_KEY="${2:-}"
      shift 2
      ;;
    --email)
      EMAIL="${2:-}"
      shift 2
      ;;
    --password)
      PASSWORD="${2:-}"
      shift 2
      ;;
    --label)
      LABEL="${2:-}"
      shift 2
      ;;
    --accounts-file)
      ACCOUNTS_FILE="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1"
      usage
      exit 1
      ;;
  esac
done

if [[ -n "$ACCOUNTS_FILE" && ! -f "$ACCOUNTS_FILE" ]]; then
  echo "accounts file not found: $ACCOUNTS_FILE"
  exit 1
fi

require_tool curl
require_tool python3
resolve_channel_meta

PAYLOAD="$(build_payload "$TOKEN" "$ACCOUNT_API_KEY" "$EMAIL" "$PASSWORD" "$LABEL" "$ACCOUNTS_FILE")"
TARGET_URL="${BASE_URL%/}/auth/login"

echo "posting account import to: $TARGET_URL"
curl -sS --max-time 30 \
  -H "Authorization: Bearer $CHANNEL_KEY" \
  -H "Content-Type: application/json" \
  -d "$PAYLOAD" \
  "$TARGET_URL"
echo
