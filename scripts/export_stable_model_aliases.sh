#!/bin/zsh

set -euo pipefail

ROOT_DIR="/Users/jj/Documents/Playground"
DB_PATH="${DB_PATH:-$ROOT_DIR/one-api.db}"

if [[ ! -f "$DB_PATH" ]]; then
  echo "db not found: $DB_PATH" >&2
  exit 1
fi

sqlite3 -json "$DB_PATH" <<'SQL'
select
  id,
  name,
  priority,
  status,
  models,
  test_model,
  model_mapping,
  settings
from channels
where name in ('cursor-pool-proxy', 'windsurf-pool-proxy', 'kiro-pool-proxy')
order by priority desc, id asc;
SQL
