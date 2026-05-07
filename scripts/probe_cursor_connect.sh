#!/usr/bin/env bash

set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:3401}"
API_KEY="${API_KEY:-demo-cursor-key}"
MODEL="${MODEL:-gpt-5}"
MODE="${MODE:-responses}"

auth_header="Authorization: Bearer ${API_KEY}"

print_line() {
  local label="$1"
  local value="$2"
  printf '%-20s %s\n' "$label" "$value"
}

http_call() {
  local method="$1"
  local url="$2"
  local body="${3:-}"
  if [[ -n "$body" ]]; then
    curl -sS -X "$method" \
      -H "$auth_header" \
      -H "Content-Type: application/json" \
      "$url" \
      -d "$body"
  else
    curl -sS -X "$method" \
      -H "$auth_header" \
      "$url"
  fi
}

echo
print_line "base_url" "$BASE_URL"
print_line "mode" "$MODE"
print_line "model" "$MODEL"

echo
echo "[auth/status]"
http_call GET "$BASE_URL/auth/status" || true
echo

echo
echo "[auth/accounts]"
http_call GET "$BASE_URL/auth/accounts" || true
echo

echo
echo "[v1/models]"
http_call GET "$BASE_URL/v1/models" || true
echo

if [[ "$MODE" == "chat" || "$MODE" == "chat_completions" ]]; then
  echo
  echo "[v1/chat/completions]"
  payload="$(printf '{"model":"%s","messages":[{"role":"user","content":"hello"}]}' "$MODEL")"
  http_call POST "$BASE_URL/v1/chat/completions" "$payload" || true
  echo
else
  echo
  echo "[v1/responses]"
  payload="$(printf '{"model":"%s","input":"hello"}' "$MODEL")"
  http_call POST "$BASE_URL/v1/responses" "$payload" || true
  echo
fi
