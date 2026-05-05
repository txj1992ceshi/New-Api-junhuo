#!/bin/zsh
set -euo pipefail

STATE_ROOT="$HOME/Library/Application Support/CursorPro3"
LOG_DIR="$STATE_ROOT/logs"
ROOT_DIR="$(cd "$(dirname "$0")/../../.." && pwd)"
mkdir -p "$LOG_DIR"

nohup env CURSORPRO3_APP_NAME='CursorPro3' \
  CURSORPRO3_BUNDLE_ID='com.yuxin.CursorPro' \
  CURSORPRO3_APP_PATH='/Applications/CursorPro3.app' \
  /usr/bin/python3 "$ROOT_DIR/cursorpro3_builder/cursorpro3_control.py" \
  >> "$LOG_DIR/control_server.log" 2>&1 < /dev/null &

echo "CursorPro3 control server started."
