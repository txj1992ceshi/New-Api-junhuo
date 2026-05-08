#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DB_PATH="${DB_PATH:-$ROOT_DIR/one-api.db}"
MODE="${1:-check}"

now_ts() {
  date +%s
}

require_sqlite() {
  if ! command -v sqlite3 >/dev/null 2>&1; then
    echo "sqlite3 not found"
    exit 1
  fi
}

sql_scalar() {
  local sql="$1"
  sqlite3 "$DB_PATH" "$sql"
}

sql_quote() {
  printf "'%s'" "$(printf '%s' "${1:-}" | sed "s/'/''/g")"
}

json_escape() {
  printf '%s' "${1:-}" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

normalize_csv() {
  printf '%s' "${1:-}" | awk -F',' '
    {
      out=""
      for (i = 1; i <= NF; i++) {
        gsub(/^[ \t]+|[ \t]+$/, "", $i)
        if ($i == "") continue
        if (out != "") out = out ","
        out = out $i
      }
      print out
    }
  '
}

port_listening() {
  local port="$1"
  if ! command -v lsof >/dev/null 2>&1; then
    echo "unknown"
    return
  fi
  if lsof -iTCP -sTCP:LISTEN -n -P 2>/dev/null | grep -q ":$port "; then
    echo "yes"
    return
  fi
  echo "no"
}

print_check_line() {
  local label="$1"
  local value="$2"
  printf '%-32s %s\n' "$label" "$value"
}

build_other_info_json() {
  local kind="$1"
  local base_url="$2"
  local api_key="$3"
  local status_path="$4"
  local accounts_path="$5"
  local dashboard_path="$6"
  local authorize_url="$7"
  local authorize_hint="$8"
  local auth_start_path="$9"
  local auth_complete_path="${10}"
  local auth_header="${11}"
  local auth_scheme="${12}"
  local tunnel_hint="${13}"
  local pool_mode="${14}"
  local auth_strategy="${15}"

  printf '{'
  printf '"%s_pool_proxy":true' "$kind"
  printf ',"%s_pool_base_url":"%s"' "$kind" "$(json_escape "$base_url")"
  printf ',"%s_pool_api_key":"%s"' "$kind" "$(json_escape "$api_key")"
  printf ',"%s_pool_status_path":"%s"' "$kind" "$(json_escape "$status_path")"
  printf ',"%s_pool_accounts_path":"%s"' "$kind" "$(json_escape "$accounts_path")"
  printf ',"%s_pool_dashboard_path":"%s"' "$kind" "$(json_escape "$dashboard_path")"
  if [[ -n "$authorize_url" ]]; then
    printf ',"%s_pool_authorize_url":"%s"' "$kind" "$(json_escape "$authorize_url")"
  fi
  if [[ -n "$authorize_hint" ]]; then
    printf ',"%s_pool_authorize_hint":"%s"' "$kind" "$(json_escape "$authorize_hint")"
  fi
  if [[ -n "$auth_start_path" ]]; then
    printf ',"%s_pool_auth_start_path":"%s"' "$kind" "$(json_escape "$auth_start_path")"
  fi
  if [[ -n "$auth_complete_path" ]]; then
    printf ',"%s_pool_auth_complete_path":"%s"' "$kind" "$(json_escape "$auth_complete_path")"
  fi
  printf ',"%s_pool_auth_header":"%s"' "$kind" "$(json_escape "$auth_header")"
  printf ',"%s_pool_auth_scheme":"%s"' "$kind" "$(json_escape "$auth_scheme")"
  if [[ -n "$tunnel_hint" ]]; then
    printf ',"%s_pool_tunnel_hint":"%s"' "$kind" "$(json_escape "$tunnel_hint")"
  fi
  if [[ -n "$pool_mode" ]]; then
    printf ',"%s_pool_mode":"%s"' "$kind" "$(json_escape "$pool_mode")"
  fi
  if [[ -n "$auth_strategy" ]]; then
    printf ',"%s_pool_auth_strategy":"%s"' "$kind" "$(json_escape "$auth_strategy")"
  fi
  printf '}'
}

build_settings_json() {
  local public_models_csv="$1"
  local responses_mapping_json="$2"

  local normalized_public_models
  normalized_public_models="$(normalize_csv "$public_models_csv")"

  python3 - "$normalized_public_models" "$responses_mapping_json" <<'PY'
import json, sys
public_models_csv = sys.argv[1] if len(sys.argv) > 1 else ""
mapping_raw = sys.argv[2] if len(sys.argv) > 2 else ""
payload = {}
public_models = [item.strip() for item in public_models_csv.split(",") if item.strip()]
if public_models:
    payload["public_models"] = public_models
try:
    mapping = json.loads(mapping_raw) if mapping_raw.strip() else {}
except Exception:
    mapping = {}
if isinstance(mapping, dict) and mapping:
    payload["responses_model_mapping"] = mapping
print(json.dumps(payload, ensure_ascii=False, separators=(",", ":")) if payload else "")
PY
}

upsert_channel() {
  local kind="$1"
  local name="$2"
  local base_url="$3"
  local api_key="$4"
  local models="$5"
  local priority="$6"
  local weight="$7"
  local group_name="$8"
  local status_path="$9"
  local accounts_path="${10}"
  local dashboard_path="${11}"
  local authorize_url="${12}"
  local authorize_hint="${13}"
  local auth_start_path="${14}"
  local auth_complete_path="${15}"
  local auth_header="${16}"
  local auth_scheme="${17}"
  local tunnel_hint="${18}"
  local pool_mode="${19}"
  local auth_strategy="${20}"
  local test_model="${21}"
  local settings_json="${22}"

  local other_info
  other_info="$(build_other_info_json "$kind" "$base_url" "$api_key" "$status_path" "$accounts_path" "$dashboard_path" "$authorize_url" "$authorize_hint" "$auth_start_path" "$auth_complete_path" "$auth_header" "$auth_scheme" "$tunnel_hint" "$pool_mode" "$auth_strategy")"

  local id
  id="$(sql_scalar "SELECT id FROM channels WHERE name = $(sql_quote "$name") LIMIT 1;")"

  if [[ -n "$id" ]]; then
    sqlite3 "$DB_PATH" "
      UPDATE channels
      SET
        type = 1,
        key = $(sql_quote "$api_key"),
        status = 1,
        base_url = $(sql_quote "$base_url"),
        models = $(sql_quote "$models"),
        test_model = $(sql_quote "$test_model"),
        \"group\" = $(sql_quote "$group_name"),
        priority = $priority,
        weight = $weight,
        auto_ban = 1,
        other_info = $(sql_quote "$other_info"),
        settings = $(sql_quote "$settings_json")
      WHERE id = $id;
    "
    echo "updated channel: $name (id=$id)"
    return
  fi

  sqlite3 "$DB_PATH" "
    INSERT INTO channels (
      type, key, status, name, weight, created_time, base_url, models, test_model, \"group\",
      priority, auto_ban, other_info, channel_info, settings
    ) VALUES (
      1,
      $(sql_quote "$api_key"),
      1,
      $(sql_quote "$name"),
      $weight,
      $(now_ts),
      $(sql_quote "$base_url"),
      $(sql_quote "$models"),
      $(sql_quote "$test_model"),
      $(sql_quote "$group_name"),
      $priority,
      1,
      $(sql_quote "$other_info"),
      '{}',
      $(sql_quote "$settings_json")
    );
  "
  echo "created channel: $name"
}

sync_channel_abilities() {
  local channel_name="$1"
  local group_name="$2"
  local models_csv="$3"
  local priority="$4"
  local weight="$5"

  local channel_id
  channel_id="$(sql_scalar "SELECT id FROM channels WHERE name = $(sql_quote "$channel_name") LIMIT 1;")"
  if [[ -z "$channel_id" ]]; then
    echo "skip abilities sync, channel not found: $channel_name"
    return
  fi

  sqlite3 "$DB_PATH" "DELETE FROM abilities WHERE channel_id = $channel_id;"

  local IFS=','
  read -r -a models_array <<< "$models_csv"
  for model_name in "${models_array[@]}"; do
    model_name="$(printf '%s' "$model_name" | xargs)"
    if [[ -z "$model_name" ]]; then
      continue
    fi
    sqlite3 "$DB_PATH" "
      INSERT OR REPLACE INTO abilities (\"group\", model, channel_id, enabled, priority, weight, tag)
      VALUES (
        $(sql_quote "$group_name"),
        $(sql_quote "$model_name"),
        $channel_id,
        1,
        $priority,
        $weight,
        ''
      );
    "
  done
}

run_check() {
  require_sqlite
  if [[ ! -f "$DB_PATH" ]]; then
    echo "db not found: $DB_PATH"
    exit 1
  fi

  echo "External pool channel readiness"
  echo "db: $DB_PATH"
  echo

  print_check_line "channels count" "$(sql_scalar 'SELECT count(*) FROM channels;')"
  print_check_line "users count" "$(sql_scalar 'SELECT count(*) FROM users;')"
  print_check_line "tokens count" "$(sql_scalar 'SELECT count(*) FROM tokens;')"
  print_check_line "options count" "$(sql_scalar 'SELECT count(*) FROM options;')"
  echo

  print_check_line "port 3003 listening" "$(port_listening 3003)"
  print_check_line "port 3401 listening" "$(port_listening 3401)"
  print_check_line "port 3501 listening" "$(port_listening 3501)"
  print_check_line "port 3601 listening" "$(port_listening 3601)"
  echo

  print_check_line "CURSOR_POOL_BASE_URL" "${CURSOR_POOL_BASE_URL:-missing}"
  print_check_line "CURSOR_POOL_API_KEY" "$( [[ -n "${CURSOR_POOL_API_KEY:-}" ]] && echo set || echo missing )"
  print_check_line "CURSOR_POOL_AUTHORIZE_URL" "${CURSOR_POOL_AUTHORIZE_URL:-missing}"
  print_check_line "CURSOR_POOL_MODE" "${CURSOR_POOL_MODE:-local_state_direct}"
  print_check_line "CURSOR_POOL_AUTH_STRATEGY" "${CURSOR_POOL_AUTH_STRATEGY:-local_state_direct}"
  print_check_line "WINDSURF_POOL_BASE_URL" "${WINDSURF_POOL_BASE_URL:-missing}"
  print_check_line "WINDSURF_POOL_API_KEY" "$( [[ -n "${WINDSURF_POOL_API_KEY:-}" ]] && echo set || echo missing )"
  print_check_line "WINDSURF_POOL_AUTHORIZE_URL" "${WINDSURF_POOL_AUTHORIZE_URL:-missing}"
  print_check_line "WINDSURF_POOL_MODE" "${WINDSURF_POOL_MODE:-external_managed}"
  print_check_line "WINDSURF_POOL_AUTH_STRATEGY" "${WINDSURF_POOL_AUTH_STRATEGY:-local_state_direct}"
  print_check_line "KIRO_POOL_BASE_URL" "${KIRO_POOL_BASE_URL:-missing}"
  print_check_line "KIRO_POOL_API_KEY" "$( [[ -n "${KIRO_POOL_API_KEY:-}" ]] && echo set || echo missing )"
  print_check_line "KIRO_POOL_AUTHORIZE_URL" "${KIRO_POOL_AUTHORIZE_URL:-missing}"
  print_check_line "KIRO_POOL_MODE" "${KIRO_POOL_MODE:-external_managed}"
  print_check_line "KIRO_POOL_AUTH_STRATEGY" "${KIRO_POOL_AUTH_STRATEGY:-local_state_direct}"
  print_check_line "CODEX_POOL_BASE_URL" "${CODEX_POOL_BASE_URL:-missing}"
  print_check_line "CODEX_POOL_API_KEY" "$( [[ -n "${CODEX_POOL_API_KEY:-}" ]] && echo set || echo missing )"
  print_check_line "CODEX_POOL_AUTHORIZE_URL" "${CODEX_POOL_AUTHORIZE_URL:-missing}"
  print_check_line "CODEX_POOL_MODE" "${CODEX_POOL_MODE:-provider_bridge}"
  print_check_line "CODEX_POOL_AUTH_STRATEGY" "${CODEX_POOL_AUTH_STRATEGY:-provider_bridge}"
}

run_apply() {
  require_sqlite
  if [[ ! -f "$DB_PATH" ]]; then
    echo "db not found: $DB_PATH"
    exit 1
  fi

  : "${CURSOR_POOL_BASE_URL:?missing CURSOR_POOL_BASE_URL}"
  : "${CURSOR_POOL_API_KEY:?missing CURSOR_POOL_API_KEY}"
  : "${WINDSURF_POOL_BASE_URL:?missing WINDSURF_POOL_BASE_URL}"
  : "${WINDSURF_POOL_API_KEY:?missing WINDSURF_POOL_API_KEY}"
  : "${KIRO_POOL_BASE_URL:?missing KIRO_POOL_BASE_URL}"
  : "${KIRO_POOL_API_KEY:?missing KIRO_POOL_API_KEY}"
  : "${CODEX_POOL_BASE_URL:?missing CODEX_POOL_BASE_URL}"
  : "${CODEX_POOL_API_KEY:?missing CODEX_POOL_API_KEY}"

  local contract_models="${PUBLIC_CONTRACT_MODELS:-gpt-5.5,gpt-5.4}"
  local cursor_models="${CURSOR_MODELS:-gpt-5.5,gpt-5.4,cursor-default,cursor-gpt5-mini,cursor-gpt4o-mini}"
  local windsurf_models="${WINDSURF_MODELS:-gpt-5.5,gpt-5.4,claude-sonnet}"
  local kiro_models="${KIRO_MODELS:-gpt-5.5,gpt-5.4,kiro-sonnet,kiro-haiku,kiro-deepseek,kiro-auto}"
  local codex_models="${CODEX_MODELS:-gpt-5.5,gpt-5.4,codex-default,codex-gpt5,codex-gpt5-mini,codex-gpt54,codex-o3-mini}"

  local cursor_settings windsurf_settings kiro_settings codex_settings
  cursor_settings="$(build_settings_json \
    "${CURSOR_PUBLIC_MODELS:-$contract_models}" \
    "${CURSOR_RESPONSES_MODEL_MAPPING:-{\"gpt-5.5\":\"default\",\"gpt-5.4\":\"gpt-5-mini\",\"cursor-default\":\"default\",\"cursor-gpt5-mini\":\"gpt-5-mini\",\"cursor-gpt4o-mini\":\"gpt-4o-mini\"}}")"
  windsurf_settings="$(build_settings_json \
    "${WINDSURF_PUBLIC_MODELS:-$contract_models}" \
    "${WINDSURF_RESPONSES_MODEL_MAPPING:-{\"gpt-5.5\":\"gpt-5-mini\",\"gpt-5.4\":\"gemini-2.5-flash\"}}")"
  kiro_settings="$(build_settings_json \
    "${KIRO_PUBLIC_MODELS:-$contract_models}" \
    "${KIRO_RESPONSES_MODEL_MAPPING:-{\"gpt-5.5\":\"claude-sonnet-4.5\",\"gpt-5.4\":\"claude-haiku-4.5\",\"kiro-sonnet\":\"claude-sonnet-4.5\",\"kiro-haiku\":\"claude-haiku-4.5\",\"kiro-deepseek\":\"deepseek-3.2\",\"kiro-auto\":\"auto\"}}")"
  codex_settings="$(build_settings_json \
    "${CODEX_PUBLIC_MODELS:-$contract_models}" \
    "${CODEX_RESPONSES_MODEL_MAPPING:-{\"gpt-5.5\":\"gpt-5.5\",\"gpt-5.4\":\"gpt-5.4\",\"codex-default\":\"gpt-5.4\",\"codex-gpt5\":\"gpt-5\",\"codex-gpt5-mini\":\"gpt-5-mini\",\"codex-gpt54\":\"gpt-5.4\",\"codex-o3-mini\":\"o3-mini\"}}")"

  upsert_channel \
    "cursor" \
    "${CURSOR_CHANNEL_NAME:-cursor-pool-proxy}" \
    "$CURSOR_POOL_BASE_URL" \
    "$CURSOR_POOL_API_KEY" \
    "$cursor_models" \
    "${CURSOR_PRIORITY:-80}" \
    "${CURSOR_WEIGHT:-100}" \
    "${CURSOR_GROUP:-default}" \
    "${CURSOR_POOL_STATUS_PATH:-/auth/status}" \
    "${CURSOR_POOL_ACCOUNTS_PATH:-/auth/accounts}" \
    "${CURSOR_POOL_DASHBOARD_PATH:-/dashboard}" \
    "${CURSOR_POOL_AUTHORIZE_URL:-}" \
    "${CURSOR_POOL_AUTHORIZE_HINT:-}" \
    "${CURSOR_POOL_AUTH_START_PATH:-/auth/start}" \
    "${CURSOR_POOL_AUTH_COMPLETE_PATH:-/auth/complete}" \
    "${CURSOR_POOL_AUTH_HEADER:-Authorization}" \
    "${CURSOR_POOL_AUTH_SCHEME:-Bearer}" \
    "${CURSOR_POOL_TUNNEL_HINT:-}" \
    "${CURSOR_POOL_MODE:-local_state_direct}" \
    "${CURSOR_POOL_AUTH_STRATEGY:-local_state_direct}" \
    "${CURSOR_TEST_MODEL:-gpt-5.5}" \
    "$cursor_settings"
  sync_channel_abilities \
    "${CURSOR_CHANNEL_NAME:-cursor-pool-proxy}" \
    "${CURSOR_GROUP:-default}" \
    "$cursor_models" \
    "${CURSOR_PRIORITY:-80}" \
    "${CURSOR_WEIGHT:-100}"

  upsert_channel \
    "windsurf" \
    "${WINDSURF_CHANNEL_NAME:-windsurf-pool-proxy}" \
    "$WINDSURF_POOL_BASE_URL" \
    "$WINDSURF_POOL_API_KEY" \
    "$windsurf_models" \
    "${WINDSURF_PRIORITY:-70}" \
    "${WINDSURF_WEIGHT:-100}" \
    "${WINDSURF_GROUP:-default}" \
    "${WINDSURF_POOL_STATUS_PATH:-/auth/status}" \
    "${WINDSURF_POOL_ACCOUNTS_PATH:-/auth/accounts}" \
    "${WINDSURF_POOL_DASHBOARD_PATH:-/dashboard}" \
    "${WINDSURF_POOL_AUTHORIZE_URL:-}" \
    "${WINDSURF_POOL_AUTHORIZE_HINT:-}" \
    "${WINDSURF_POOL_AUTH_START_PATH:-/auth/start}" \
    "${WINDSURF_POOL_AUTH_COMPLETE_PATH:-/auth/complete}" \
    "${WINDSURF_POOL_AUTH_HEADER:-Authorization}" \
    "${WINDSURF_POOL_AUTH_SCHEME:-Bearer}" \
    "${WINDSURF_POOL_TUNNEL_HINT:-}" \
    "${WINDSURF_POOL_MODE:-external_managed}" \
    "${WINDSURF_POOL_AUTH_STRATEGY:-local_state_direct}" \
    "${WINDSURF_TEST_MODEL:-gpt-5.5}" \
    "$windsurf_settings"
  sync_channel_abilities \
    "${WINDSURF_CHANNEL_NAME:-windsurf-pool-proxy}" \
    "${WINDSURF_GROUP:-default}" \
    "$windsurf_models" \
    "${WINDSURF_PRIORITY:-70}" \
    "${WINDSURF_WEIGHT:-100}"

  upsert_channel \
    "kiro" \
    "${KIRO_CHANNEL_NAME:-kiro-pool-proxy}" \
    "$KIRO_POOL_BASE_URL" \
    "$KIRO_POOL_API_KEY" \
    "$kiro_models" \
    "${KIRO_PRIORITY:-60}" \
    "${KIRO_WEIGHT:-100}" \
    "${KIRO_GROUP:-default}" \
    "${KIRO_POOL_STATUS_PATH:-/auth/status}" \
    "${KIRO_POOL_ACCOUNTS_PATH:-/auth/accounts}" \
    "${KIRO_POOL_DASHBOARD_PATH:-/dashboard}" \
    "${KIRO_POOL_AUTHORIZE_URL:-}" \
    "${KIRO_POOL_AUTHORIZE_HINT:-}" \
    "${KIRO_POOL_AUTH_START_PATH:-/auth/start}" \
    "${KIRO_POOL_AUTH_COMPLETE_PATH:-/auth/complete}" \
    "${KIRO_POOL_AUTH_HEADER:-Authorization}" \
    "${KIRO_POOL_AUTH_SCHEME:-Bearer}" \
    "${KIRO_POOL_TUNNEL_HINT:-}" \
    "${KIRO_POOL_MODE:-external_managed}" \
    "${KIRO_POOL_AUTH_STRATEGY:-local_state_direct}" \
    "${KIRO_TEST_MODEL:-gpt-5.5}" \
    "$kiro_settings"
  sync_channel_abilities \
    "${KIRO_CHANNEL_NAME:-kiro-pool-proxy}" \
    "${KIRO_GROUP:-default}" \
    "$kiro_models" \
    "${KIRO_PRIORITY:-60}" \
    "${KIRO_WEIGHT:-100}"

  upsert_channel \
    "codex" \
    "${CODEX_CHANNEL_NAME:-codex-pool-proxy}" \
    "$CODEX_POOL_BASE_URL" \
    "$CODEX_POOL_API_KEY" \
    "$codex_models" \
    "${CODEX_PRIORITY:-50}" \
    "${CODEX_WEIGHT:-100}" \
    "${CODEX_GROUP:-default}" \
    "${CODEX_POOL_STATUS_PATH:-/auth/status}" \
    "${CODEX_POOL_ACCOUNTS_PATH:-/auth/accounts}" \
    "${CODEX_POOL_DASHBOARD_PATH:-/dashboard}" \
    "${CODEX_POOL_AUTHORIZE_URL:-}" \
    "${CODEX_POOL_AUTHORIZE_HINT:-}" \
    "${CODEX_POOL_AUTH_START_PATH:-/auth/start}" \
    "${CODEX_POOL_AUTH_COMPLETE_PATH:-/auth/complete}" \
    "${CODEX_POOL_AUTH_HEADER:-Authorization}" \
    "${CODEX_POOL_AUTH_SCHEME:-Bearer}" \
    "${CODEX_POOL_TUNNEL_HINT:-}" \
    "${CODEX_POOL_MODE:-provider_bridge}" \
    "${CODEX_POOL_AUTH_STRATEGY:-provider_bridge}" \
    "${CODEX_TEST_MODEL:-gpt-5.5}" \
    "$codex_settings"
  sync_channel_abilities \
    "${CODEX_CHANNEL_NAME:-codex-pool-proxy}" \
    "${CODEX_GROUP:-default}" \
    "$codex_models" \
    "${CODEX_PRIORITY:-50}" \
    "${CODEX_WEIGHT:-100}"

  echo "external pool channels applied and abilities synced"
}

case "$MODE" in
  check)
    run_check
    ;;
  apply)
    run_apply
    ;;
  *)
    echo "usage: $0 [check|apply]"
    exit 1
    ;;
esac
