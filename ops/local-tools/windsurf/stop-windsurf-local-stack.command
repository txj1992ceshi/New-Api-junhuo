#!/bin/bash

ROOT_DIR="$(cd "$(dirname "$0")/../../.." && pwd)"

cd "$ROOT_DIR" || exit 1

"$ROOT_DIR/ops/local-tools/windsurf/stop-windsurf-local-stack.sh"
STATUS=$?

echo
read -r -p "Press Enter to close this window..."
exit "$STATUS"
