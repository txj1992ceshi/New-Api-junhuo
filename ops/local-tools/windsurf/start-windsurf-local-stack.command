#!/bin/bash

ROOT_DIR="$(cd "$(dirname "$0")/../../.." && pwd)"

cd "$ROOT_DIR" || exit 1

"$ROOT_DIR/ops/local-tools/windsurf/start-windsurf-local-stack.sh"
STATUS=$?

echo
if [ "$STATUS" -eq 0 ]; then
  echo "Opening Windsurf control panel..."
  open "http://127.0.0.1:3310"
fi

echo
read -r -p "Press Enter to close this window..."
exit "$STATUS"
