#!/usr/bin/env bash

set -euo pipefail

HOME_DIR="${HOME:?HOME is required}"
CURSOR_DB_PATH="${CURSOR_DB_PATH:-$HOME_DIR/Library/Application Support/Cursor/User/globalStorage/state.vscdb}"
WINDSURF_DB_PATH="${WINDSURF_DB_PATH:-$HOME_DIR/Library/Application Support/Windsurf/User/globalStorage/state.vscdb}"
KIRO_AUTH_PATH="${KIRO_AUTH_PATH:-$HOME_DIR/.aws/sso/cache/kiro-auth-token.json}"
OUTPUT_DIR=""

usage() {
  cat <<'EOF'
Usage:
  bash scripts/export_local_client_auth_snapshots.sh --output-dir /absolute/or/relative/path

Exports raw local auth snapshots for Cursor, Windsurf and Kiro into the target directory:
  - cursor-local-auth.json
  - windsurf-local-auth.json
  - kiro-local-auth.json

Notes:
  - This script writes sensitive access/refresh tokens.
  - Nothing is exported unless --output-dir is provided.
EOF
}

require_tool() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "$1 not found"
    exit 1
  fi
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --output-dir)
      OUTPUT_DIR="${2:-}"
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

if [[ -z "$OUTPUT_DIR" ]]; then
  usage
  exit 1
fi

require_tool sqlite3
require_tool python3

mkdir -p "$OUTPUT_DIR"

export CURSOR_DB_PATH
export WINDSURF_DB_PATH
export KIRO_AUTH_PATH
export OUTPUT_DIR

python3 - <<'PY'
from pathlib import Path
import json
import os
import sqlite3

cursor_db_path = Path(os.environ["CURSOR_DB_PATH"])
kiro_auth_path = Path(os.environ["KIRO_AUTH_PATH"])
windsurf_db_path = Path(os.environ["WINDSURF_DB_PATH"])
output_dir = Path(os.environ["OUTPUT_DIR"])
output_dir.mkdir(parents=True, exist_ok=True)

exported = []

if cursor_db_path.exists():
    conn = sqlite3.connect(str(cursor_db_path))
    try:
        def get_value(key: str) -> str:
            row = conn.execute(
                "SELECT value FROM ItemTable WHERE key = ? LIMIT 1", (key,)
            ).fetchone()
            return (row[0] if row and row[0] else "").strip()

        cursor_payload = {
            "provider": "cursor",
            "source": "local_cursor_state_vscdb",
            "source_path": str(cursor_db_path),
            "email": get_value("cursorAuth/cachedEmail"),
            "sign_up_type": get_value("cursorAuth/cachedSignUpType"),
            "membership_type": get_value("cursorAuth/stripeMembershipType"),
            "subscription_status": get_value("cursorAuth/stripeSubscriptionStatus"),
            "auth_id": get_value("cursorAuth/authId"),
            "access_token": get_value("cursorAuth/accessToken"),
            "refresh_token": get_value("cursorAuth/refreshToken"),
        }
        if cursor_payload["email"] and cursor_payload["access_token"]:
            path = output_dir / "cursor-local-auth.json"
            path.write_text(json.dumps(cursor_payload, ensure_ascii=False, indent=2))
            exported.append(str(path))
    finally:
        conn.close()

if windsurf_db_path.exists():
    conn = sqlite3.connect(str(windsurf_db_path))
    try:
        def get_ws_value(key: str) -> str:
            row = conn.execute(
                "SELECT value FROM ItemTable WHERE key = ? LIMIT 1", (key,)
            ).fetchone()
            return (row[0] if row and row[0] else "").strip()

        auth = json.loads(get_ws_value("windsurfAuthStatus") or "{}")
        config = json.loads(get_ws_value("codeium.windsurf") or "{}")
        plan = json.loads(get_ws_value("windsurf.settings.cachedPlanInfo") or "{}")
        windsurf_payload = {
            "provider": "windsurf",
            "source": "local_windsurf_state_vscdb",
            "source_path": str(windsurf_db_path),
            "email": config.get("lastLoginEmail", ""),
            "api_server_url": config.get("apiServerUrl", ""),
            "plan_name": plan.get("planName", ""),
            "access_token": auth.get("apiKey", ""),
            "raw": {
                "auth_status": auth,
                "config": config,
                "plan": plan,
            },
        }
        if windsurf_payload["email"] and windsurf_payload["access_token"]:
            path = output_dir / "windsurf-local-auth.json"
            path.write_text(json.dumps(windsurf_payload, ensure_ascii=False, indent=2))
            exported.append(str(path))
    finally:
        conn.close()

if kiro_auth_path.exists():
    raw = json.loads(kiro_auth_path.read_text())
    kiro_payload = {
        "provider": "kiro",
        "source": "local_kiro_auth_token",
        "source_path": str(kiro_auth_path),
        "email": raw.get("email", ""),
        "auth_method": raw.get("authMethod", raw.get("auth_method", "")),
        "login_provider": raw.get("provider", raw.get("loginProvider", "")),
        "region": raw.get("region", raw.get("idc_region", "")),
        "client_id": raw.get("clientId", raw.get("client_id", "")),
        "expires_at": raw.get("expiresAt", raw.get("expires_at", "")),
        "access_token": raw.get("accessToken", raw.get("access_token", "")),
        "refresh_token": raw.get("refreshToken", raw.get("refresh_token", "")),
        "token_type": raw.get("tokenType", raw.get("token_type", "")),
        "raw": raw,
    }
    if kiro_payload["email"] and kiro_payload["access_token"]:
        path = output_dir / "kiro-local-auth.json"
        path.write_text(json.dumps(kiro_payload, ensure_ascii=False, indent=2))
        exported.append(str(path))

print(json.dumps({"exported": exported}, ensure_ascii=False))
PY
