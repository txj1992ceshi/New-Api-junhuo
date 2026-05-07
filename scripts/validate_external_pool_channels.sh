#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DB_PATH="${DB_PATH:-$ROOT_DIR/one-api.db}"
TMP_DIR="${TMPDIR:-/tmp}/newapi-external-pool-validate"
STATUS_TIMEOUT="${STATUS_TIMEOUT:-10}"
MODELS_TIMEOUT="${MODELS_TIMEOUT:-15}"
CURSOR_MODELS_TIMEOUT="${CURSOR_MODELS_TIMEOUT:-60}"
RESPONSES_TIMEOUT="${RESPONSES_TIMEOUT:-60}"
mkdir -p "$TMP_DIR"

if ! command -v sqlite3 >/dev/null 2>&1; then
  echo "sqlite3 not found"
  exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "curl not found"
  exit 1
fi

mask_value() {
  local raw="${1:-}"
  if [[ -z "$raw" ]]; then
    echo "(empty)"
    return
  fi
  local len="${#raw}"
  if (( len <= 8 )); then
    echo "********"
    return
  fi
  printf '%s***%s' "${raw:0:4}" "${raw:len-4:4}"
}

print_line() {
  local label="$1"
  local value="$2"
  printf '%-22s %s\n' "$label" "$value"
}

http_probe() {
  local method="$1"
  local url="$2"
  local auth_header="$3"
  local timeout_seconds="$4"
  local body="${5:-}"
  local headers_file="$TMP_DIR/headers.$$"
  local body_file="$TMP_DIR/body.$$"

  rm -f "$headers_file" "$body_file"
  if [[ -n "$body" ]]; then
    curl -sS --max-time "$timeout_seconds" -X "$method" \
      -H "$auth_header" \
      -H "Content-Type: application/json" \
      -D "$headers_file" \
      -o "$body_file" \
      "$url" \
      -d "$body" || true
  else
    curl -sS --max-time "$timeout_seconds" -X "$method" \
      -H "$auth_header" \
      -D "$headers_file" \
      -o "$body_file" \
      "$url" || true
  fi

  local status
  status="$(awk 'toupper($1) ~ /^HTTP/ { code=$2 } END { print code }' "$headers_file" 2>/dev/null || true)"
  echo "${status:-000}"
  cat "$body_file" 2>/dev/null || true
}

resolve_first_model_from_models() {
  local kind="$1"
  local models_body="$2"
  python3 - "$kind" "$models_body" <<'PY'
import json, sys
kind = (sys.argv[1] or "").strip()
raw = sys.argv[2]
try:
    data = json.loads(raw)
except Exception:
    print("")
    raise SystemExit(0)
items = data.get("data") or []
preferred_map = {
    "cursor": [
        "default",
        "auto",
        "gpt-4.1-mini",
        "gpt-4o-mini",
        "gpt-5-mini",
    ],
    "windsurf": [
        "gpt-5-mini",
        "gpt-4o-mini",
        "gpt-4.1-mini",
    ],
    "kiro": [
        "claude-sonnet-4.5",
        "claude-sonnet-4",
        "claude-haiku-4.5",
        "deepseek-3.2",
        "glm-5",
        "qwen3-coder-next",
        "auto",
    ],
}
preferred = preferred_map.get(kind) or [
    "gpt-5.4-high",
    "gpt-5.4-medium",
    "gpt-5.2-codex-medium",
    "claude-4.5-sonnet-thinking",
    "claude-4.5-sonnet",
]
ids = [item.get("id", "") for item in items if isinstance(item, dict)]
for name in preferred:
    if name in ids:
        print(name)
        raise SystemExit(0)
print(ids[0] if ids else "")
PY
}

resolve_first_model_from_accounts() {
  local kind="$1"
  local accounts_body="$2"
  python3 - "$kind" "$accounts_body" <<'PY'
import json, sys
kind = (sys.argv[1] or "").strip()
raw = sys.argv[2]
try:
    data = json.loads(raw)
except Exception:
    print("")
    raise SystemExit(0)
accounts = data.get("accounts") or data.get("data") or []
if not isinstance(accounts, list):
    print("")
    raise SystemExit(0)
available = []
for account in accounts:
    if not isinstance(account, dict):
        continue
    models = account.get("availableModels") or account.get("available_models") or []
    if isinstance(models, list):
        available.extend([str(item).strip() for item in models if str(item).strip()])
    if kind != "windsurf":
        capabilities = account.get("capabilities") or {}
        if isinstance(capabilities, dict):
            for model_name, meta in capabilities.items():
                if not isinstance(meta, dict):
                    continue
                if meta.get("ok") is True:
                    name = str(model_name).strip()
                    if name:
                        available.append(name)
preferred_map = {
    "cursor": [
        "default",
        "auto",
        "gpt-4.1-mini",
        "gpt-4o-mini",
        "gpt-5-mini",
    ],
    "windsurf": [
        "gpt-5-mini",
        "gpt-4o-mini",
        "gpt-4.1-mini",
    ],
    "kiro": [
        "claude-sonnet-4.5",
        "claude-sonnet-4",
        "claude-haiku-4.5",
        "deepseek-3.2",
        "glm-5",
        "qwen3-coder-next",
        "auto",
    ],
}
preferred = preferred_map.get(kind) or [
    "swe-1.5-fast",
    "gemini-2.5-flash",
    "glm-4.7",
    "gpt-5.2-low",
    "gpt-5-mini",
    "gpt-4.1-mini",
    "gpt-4o-mini",
]
for name in preferred:
    if name in available:
        print(name)
        raise SystemExit(0)
seen = []
for name in available:
    if name not in seen:
        seen.append(name)
print(seen[0] if seen else "")
PY
}

extract_available_models_summary() {
  local accounts_body="$1"
  python3 - "$accounts_body" <<'PY'
import json, sys
raw = sys.argv[1]
try:
    data = json.loads(raw)
except Exception:
    print("")
    raise SystemExit(0)
accounts = data.get("accounts") or data.get("data") or []
if not isinstance(accounts, list):
    print("")
    raise SystemExit(0)
available = []
for account in accounts:
    if not isinstance(account, dict):
        continue
    models = account.get("availableModels") or account.get("available_models") or []
    if isinstance(models, list):
        for name in models:
            name = str(name).strip()
            if name and name not in available:
                available.append(name)
print(",".join(available))
PY
}

build_candidate_models() {
  local kind="$1"
  local accounts_body="$2"
  local models_body="$3"
  python3 - "$kind" "$accounts_body" "$models_body" <<'PY'
import json, sys

kind = (sys.argv[1] or "").strip()
accounts_raw = sys.argv[2]
models_raw = sys.argv[3]

preferred_map = {
    "cursor": ["default", "auto", "gpt-4.1-mini", "gpt-4o-mini", "gpt-5-mini"],
    "windsurf": ["gpt-5-mini", "gpt-4o-mini", "gpt-4.1-mini"],
    "kiro": ["claude-sonnet-4.5", "claude-sonnet-4", "claude-haiku-4.5", "deepseek-3.2", "glm-5", "qwen3-coder-next", "auto"],
}

def load_json(raw):
    try:
        return json.loads(raw)
    except Exception:
        return {}

accounts_data = load_json(accounts_raw)
models_data = load_json(models_raw)

accounts = accounts_data.get("accounts") or accounts_data.get("data") or []
if not isinstance(accounts, list):
    accounts = []
model_items = models_data.get("data") or []
if not isinstance(model_items, list):
    model_items = []

seen = set()
ordered = []
def add(name):
    name = str(name or "").strip()
    if not name or name in seen:
        return
    seen.add(name)
    ordered.append(name)

for account in accounts:
    if not isinstance(account, dict):
        continue
    for key in ("availableModels", "available_models", "tierModels", "tier_models"):
        value = account.get(key) or []
        if isinstance(value, list):
            for name in value:
                add(name)
    if kind != "windsurf":
        capabilities = account.get("capabilities") or {}
        if isinstance(capabilities, dict):
            for model_name, meta in capabilities.items():
                if not isinstance(meta, dict):
                    continue
                if meta.get("ok") is True:
                    add(model_name)

for name in preferred_map.get(kind) or []:
    add(name)

if kind != "windsurf":
    for item in model_items:
        if isinstance(item, dict):
            add(item.get("id"))

print("\n".join(ordered))
PY
}

resolve_channel_inference_mode() {
  local kind="$1"
  local other_info="${2:-}"
  python3 - "$kind" "$other_info" <<'PY'
import json, sys
kind = (sys.argv[1] or "").strip()
raw = sys.argv[2] if len(sys.argv) > 2 else ""
mode = "responses"
try:
    payload = json.loads(raw) if raw else {}
except Exception:
    payload = {}
if isinstance(payload, dict):
    candidate = payload.get(f"{kind}_pool_inference_mode") or payload.get("pool_inference_mode")
    if isinstance(candidate, str):
        candidate = candidate.strip().lower()
        if candidate in {"responses", "chat_completions", "dual"}:
            mode = candidate
print(mode)
PY
}

should_fallback_to_chat() {
  local code="${1:-000}"
  local body_lc="${2:-}"
  if [[ "$code" == "404" ]]; then
    return 0
  fi
  [[ "$body_lc" == *"path disabled"* ]] && return 0
  [[ "$body_lc" == *"not found"* ]] && return 0
  [[ "$body_lc" == *"disabled by inference_mode"* ]] && return 0
  [[ "$body_lc" == *"disabled by inference mode"* ]] && return 0
  return 1
}

probe_inference_with_mode() {
  local base_url="$1"
  local auth_header="$2"
  local smoke_model="$3"
  local inference_mode="$4"
  local response_code="000"
  local response_body=""
  local response_result=""

  if [[ "$inference_mode" == "chat_completions" ]]; then
    local payload
    payload="$(printf '{"model":"%s","messages":[{"role":"user","content":"hello"}]}' "$smoke_model")"
    response_result="$(http_probe POST "$base_url/v1/chat/completions" "$auth_header" "$RESPONSES_TIMEOUT" "$payload")"
    response_code="$(printf '%s\n' "$response_result" | sed -n '1p')"
    response_body="$(printf '%s\n' "$response_result" | sed -n '2,$p')"
    printf '%s\n%s\n' "$response_code" "$response_body"
    return
  fi

  local responses_payload
  responses_payload="$(printf '{"model":"%s","input":"hello"}' "$smoke_model")"
  response_result="$(http_probe POST "$base_url/v1/responses" "$auth_header" "$RESPONSES_TIMEOUT" "$responses_payload")"
  response_code="$(printf '%s\n' "$response_result" | sed -n '1p')"
  response_body="$(printf '%s\n' "$response_result" | sed -n '2,$p')"

  if [[ "$inference_mode" == "dual" ]]; then
    local response_body_lc
    response_body_lc="$(printf '%s' "$response_body" | tr '[:upper:]' '[:lower:]')"
    if should_fallback_to_chat "$response_code" "$response_body_lc"; then
      local chat_payload chat_result
      chat_payload="$(printf '{"model":"%s","messages":[{"role":"user","content":"hello"}]}' "$smoke_model")"
      chat_result="$(http_probe POST "$base_url/v1/chat/completions" "$auth_header" "$RESPONSES_TIMEOUT" "$chat_payload")"
      response_code="$(printf '%s\n' "$chat_result" | sed -n '1p')"
      response_body="$(printf '%s\n' "$chat_result" | sed -n '2,$p')"
    fi
  fi

  printf '%s\n%s\n' "$response_code" "$response_body"
}

should_try_next_model() {
  local body_lc="${1:-}"
  [[ "$body_lc" == *"model_not_entitled"* ]] && return 0
  [[ "$body_lc" == *"not entitled"* ]] && return 0
  [[ "$body_lc" == *"model_not_found"* ]] && return 0
  [[ "$body_lc" == *"unsupported model"* ]] && return 0
  [[ "$body_lc" == *"invalid model"* ]] && return 0
  [[ "$body_lc" == *"model_deprecated"* ]] && return 0
  [[ "$body_lc" == *"已被 windsurf 上游废弃"* ]] && return 0
  [[ "$body_lc" == *"不可用（未订阅或已被封禁）"* ]] && return 0
  return 1
}

validate_channel() {
  local name="$1"
  local smoke_model="${2:-}"

  local row
  row="$(sqlite3 -separator $'\t' "$DB_PATH" "select id,name,base_url,key,status,models,other_info from channels where name = '$name' limit 1;")"
  if [[ -z "$row" ]]; then
    echo
    echo "[$name]"
    echo "channel not found in database"
    return
  fi

  IFS=$'\t' read -r id channel_name base_url api_key status models other_info <<< "$row"
  local auth_header="Authorization: Bearer $api_key"
  local kind
  case "$channel_name" in
    cursor-*) kind="cursor" ;;
    kiro-*) kind="kiro" ;;
    *) kind="windsurf" ;;
  esac
  local models_timeout="$MODELS_TIMEOUT"
  if [[ "$kind" == "cursor" ]]; then
    models_timeout="$CURSOR_MODELS_TIMEOUT"
  fi

  echo
  echo "[$channel_name]"
  print_line "id" "$id"
  print_line "status" "$status"
  print_line "base_url" "$base_url"
  print_line "api_key" "$(mask_value "$api_key")"
  print_line "models" "$models"

  local status_result status_code status_body
  status_result="$(http_probe GET "$base_url/auth/status" "$auth_header" "$STATUS_TIMEOUT")"
  status_code="$(printf '%s\n' "$status_result" | sed -n '1p')"
  status_body="$(printf '%s\n' "$status_result" | sed -n '2,$p')"
  print_line "status_http" "$status_code"
  print_line "status_body" "$status_body"

  local models_result models_code models_body
  models_result="$(http_probe GET "$base_url/v1/models" "$auth_header" "$models_timeout")"
  models_code="$(printf '%s\n' "$models_result" | sed -n '1p')"
  models_body="$(printf '%s\n' "$models_result" | sed -n '2,$p')"
  print_line "models_http" "$models_code"

  local model_count
  model_count="$(python3 - "$models_body" <<'PY'
import json, sys
try:
    data = json.loads(sys.argv[1])
    print(len(data.get("data") or []))
except Exception:
    print(0)
PY
)"
  print_line "models_count" "$model_count"

  local accounts_result accounts_code accounts_body account_available_models
  accounts_result="$(http_probe GET "$base_url/auth/accounts" "$auth_header" "$models_timeout")"
  accounts_code="$(printf '%s\n' "$accounts_result" | sed -n '1p')"
  accounts_body="$(printf '%s\n' "$accounts_result" | sed -n '2,$p')"
  print_line "accounts_http" "$accounts_code"
  if [[ "$accounts_code" == "200" ]]; then
    account_available_models="$(extract_available_models_summary "$accounts_body")"
    if [[ -n "$account_available_models" ]]; then
      print_line "account_models" "$account_available_models"
    fi
  fi

  local candidate_models=""
  if [[ -z "$smoke_model" ]]; then
    candidate_models="$(build_candidate_models "$kind" "$accounts_body" "$models_body")"
    if [[ "$accounts_code" == "200" ]]; then
      smoke_model="$(resolve_first_model_from_accounts "$kind" "$accounts_body")"
    fi
    if [[ -z "$smoke_model" ]]; then
      smoke_model="$(resolve_first_model_from_models "$kind" "$models_body")"
    fi
  fi
  if [[ -z "$smoke_model" ]]; then
    print_line "inference_mode" "skipped(no model)"
    print_line "inference_http" "skipped(no model)"
    print_line "inference_body" ""
    return
  fi

  local inference_mode
  inference_mode="$(resolve_channel_inference_mode "$kind" "$other_info")"
  local response_result response_code response_body
  local selected_model="$smoke_model"
  if [[ -n "$candidate_models" ]]; then
    while IFS= read -r candidate; do
      [[ -z "$candidate" ]] && continue
      selected_model="$candidate"
      response_result="$(probe_inference_with_mode "$base_url" "$auth_header" "$candidate" "$inference_mode")"
      response_code="$(printf '%s\n' "$response_result" | sed -n '1p')"
      response_body="$(printf '%s\n' "$response_result" | sed -n '2,$p')"
      if [[ "$response_code" =~ ^2 ]]; then
        break
      fi
      local response_body_lc
      response_body_lc="$(printf '%s' "$response_body" | tr '[:upper:]' '[:lower:]')"
      if ! should_try_next_model "$response_body_lc"; then
        break
      fi
    done <<< "$candidate_models"
  else
    response_result="$(probe_inference_with_mode "$base_url" "$auth_header" "$smoke_model" "$inference_mode")"
    response_code="$(printf '%s\n' "$response_result" | sed -n '1p')"
    response_body="$(printf '%s\n' "$response_result" | sed -n '2,$p')"
  fi
  print_line "smoke_model" "$selected_model"
  print_line "responses_timeout" "${RESPONSES_TIMEOUT}s"
  print_line "inference_mode" "$inference_mode"
  print_line "inference_http" "$response_code"
  print_line "inference_body" "$response_body"
}

validate_channel "cursor-pool-proxy" "${CURSOR_SMOKE_MODEL:-}"
validate_channel "windsurf-pool-proxy" "${WINDSURF_SMOKE_MODEL:-}"
validate_channel "kiro-pool-proxy" "${KIRO_SMOKE_MODEL:-}"
