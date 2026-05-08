#!/bin/zsh

set -euo pipefail

ROOT_DIR="/Users/jj/Documents/Playground"
DB_PATH="${DB_PATH:-$ROOT_DIR/one-api.db}"
TOKEN_ID="${TOKEN_ID:-}"
TOKEN_NAME="${TOKEN_NAME:-}"
PROFILE="${PROFILE:-full}"

if [[ ! -f "$DB_PATH" ]]; then
  echo "db not found: $DB_PATH" >&2
  exit 1
fi

if [[ -z "$TOKEN_ID" && -z "$TOKEN_NAME" ]]; then
  echo "set TOKEN_ID or TOKEN_NAME" >&2
  exit 1
fi

case "$PROFILE" in
  full)
    MODEL_LIMITS="gpt-5.5,gpt-5.4,cursor-default,cursor-gpt5-mini,cursor-gpt4o-mini,kiro-sonnet,kiro-haiku,kiro-deepseek,kiro-auto,codex-default,codex-gpt5,codex-gpt5-mini,codex-gpt54,codex-o3-mini"
    ;;
  conservative)
    MODEL_LIMITS="gpt-5.5,gpt-5.4,cursor-default,kiro-sonnet,codex-default"
    ;;
  *)
    echo "unknown PROFILE: $PROFILE (use full or conservative)" >&2
    exit 1
    ;;
esac

if [[ -n "$TOKEN_ID" ]]; then
  WHERE_CLAUSE="id = ${TOKEN_ID}"
else
  ESCAPED_NAME="${TOKEN_NAME//\'/''}"
  WHERE_CLAUSE="name = '${ESCAPED_NAME}'"
fi

sqlite3 "$DB_PATH" <<SQL
update tokens
set model_limits_enabled = 1,
    model_limits = '${MODEL_LIMITS}'
where ${WHERE_CLAUSE};

select
  id,
  name,
  model_limits_enabled,
  model_limits
from tokens
where ${WHERE_CLAUSE};
SQL
