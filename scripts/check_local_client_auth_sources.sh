#!/usr/bin/env bash

set -euo pipefail

HOME_DIR="${HOME:?HOME is required}"
CURSOR_DB_PATH="${CURSOR_DB_PATH:-$HOME_DIR/Library/Application Support/Cursor/User/globalStorage/state.vscdb}"
WINDSURF_DB_PATH="${WINDSURF_DB_PATH:-$HOME_DIR/Library/Application Support/Windsurf/User/globalStorage/state.vscdb}"
KIRO_AUTH_PATH="${KIRO_AUTH_PATH:-$HOME_DIR/.aws/sso/cache/kiro-auth-token.json}"

require_tool() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "$1 not found"
    exit 1
  fi
}

print_line() {
  local label="$1"
  local value="$2"
  printf '%-28s %s\n' "$label" "$value"
}

mask_value() {
  local raw="${1:-}"
  if [[ -z "$raw" ]]; then
    echo "(empty)"
    return
  fi
  local len="${#raw}"
  if (( len <= 10 )); then
    echo "********"
    return
  fi
  printf '%s***%s (len=%d)' "${raw:0:6}" "${raw:len-4:4}" "$len"
}

check_cursor() {
  echo
  echo "[Cursor local auth]"
  print_line "state_db" "$CURSOR_DB_PATH"
  if [[ ! -f "$CURSOR_DB_PATH" ]]; then
    print_line "ready" "no (state.vscdb missing)"
    return
  fi

  local query_output
  query_output="$(sqlite3 "$CURSOR_DB_PATH" "
    select
      coalesce((select value from ItemTable where key='cursorAuth/cachedEmail' limit 1), ''),
      coalesce((select value from ItemTable where key='cursorAuth/cachedSignUpType' limit 1), ''),
      coalesce((select value from ItemTable where key='cursorAuth/stripeMembershipType' limit 1), ''),
      coalesce((select value from ItemTable where key='cursorAuth/accessToken' limit 1), ''),
      coalesce((select value from ItemTable where key='cursorAuth/refreshToken' limit 1), '')
    ;
  ")"

  local email signup_type membership_type access_token refresh_token
  IFS='|' read -r email signup_type membership_type access_token refresh_token <<< "$query_output"

  print_line "email" "${email:-"(missing)"}"
  print_line "sign_up_type" "${signup_type:-"(missing)"}"
  print_line "membership_type" "${membership_type:-"(missing)"}"
  print_line "access_token" "$(mask_value "$access_token")"
  print_line "refresh_token" "$(mask_value "$refresh_token")"

  if [[ -n "${email:-}" && -n "${access_token:-}" ]]; then
    print_line "ready" "yes (cockpit-tools import_cursor_from_local source present)"
  else
    print_line "ready" "no (missing email or access token)"
  fi
}

check_kiro() {
  echo
  echo "[Kiro local auth]"
  print_line "auth_file" "$KIRO_AUTH_PATH"
  if [[ ! -f "$KIRO_AUTH_PATH" ]]; then
    print_line "ready" "no (kiro-auth-token.json missing)"
    return
  fi

  local parsed
  parsed="$(python3 - "$KIRO_AUTH_PATH" <<'PY'
from pathlib import Path
import json, sys
p = Path(sys.argv[1])
data = json.loads(p.read_text())
fields = [
    data.get("email", ""),
    data.get("provider", ""),
    data.get("authMethod", ""),
    data.get("region", ""),
    data.get("expiresAt", ""),
    data.get("accessToken", "") or data.get("access_token", ""),
    data.get("refreshToken", "") or data.get("refresh_token", ""),
]
print("|".join(str(x) for x in fields))
PY
)"

  local email provider auth_method region expires_at access_token refresh_token
  IFS='|' read -r email provider auth_method region expires_at access_token refresh_token <<< "$parsed"

  print_line "email" "${email:-"(missing)"}"
  print_line "provider" "${provider:-"(missing)"}"
  print_line "auth_method" "${auth_method:-"(missing)"}"
  print_line "region" "${region:-"(missing)"}"
  print_line "expires_at" "${expires_at:-"(missing)"}"
  print_line "access_token" "$(mask_value "$access_token")"
  print_line "refresh_token" "$(mask_value "$refresh_token")"

  if [[ -n "${email:-}" && -n "${access_token:-}" ]]; then
    print_line "ready" "yes (cockpit-tools import_kiro_from_local source present)"
  else
    print_line "ready" "no (missing email or access token)"
  fi
}

check_windsurf() {
  echo
  echo "[Windsurf local auth]"
  print_line "state_db" "$WINDSURF_DB_PATH"
  if [[ ! -f "$WINDSURF_DB_PATH" ]]; then
    print_line "ready" "no (state.vscdb missing)"
    return
  fi

  local parsed
  parsed="$(python3 - "$WINDSURF_DB_PATH" <<'PY'
import sqlite3, sys, json
conn = sqlite3.connect(sys.argv[1])
def get_value(key: str) -> str:
    row = conn.execute("SELECT value FROM ItemTable WHERE key = ? LIMIT 1", (key,)).fetchone()
    return (row[0] if row and row[0] else "").strip()
auth = json.loads(get_value("windsurfAuthStatus") or "{}")
config = json.loads(get_value("codeium.windsurf") or "{}")
plan = json.loads(get_value("windsurf.settings.cachedPlanInfo") or "{}")
fields = [
    config.get("lastLoginEmail", ""),
    plan.get("planName", ""),
    config.get("apiServerUrl", ""),
    auth.get("apiKey", ""),
]
print("|".join(str(x) for x in fields))
PY
)"

  local email plan_name api_server_url api_key
  IFS='|' read -r email plan_name api_server_url api_key <<< "$parsed"
  print_line "email" "${email:-"(missing)"}"
  print_line "plan_name" "${plan_name:-"(missing)"}"
  print_line "api_server_url" "${api_server_url:-"(missing)"}"
  print_line "api_key" "$(mask_value "$api_key")"

  if [[ -n "${email:-}" && -n "${api_key:-}" ]]; then
    print_line "ready" "yes (local_state_direct source present)"
  else
    print_line "ready" "no (missing email or api key)"
  fi
}

require_tool sqlite3
require_tool python3

check_cursor
check_windsurf
check_kiro
