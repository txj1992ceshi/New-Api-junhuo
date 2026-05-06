#!/bin/zsh
set -euo pipefail

STATE_ROOT="$HOME/Library/Application Support/CursorPro3"
LOG_DIR="$STATE_ROOT/logs"
PID_FILE="$STATE_ROOT/control_server.pid"
ROOT_DIR="$(cd "$(dirname "$0")/../../.." && pwd)"
mkdir -p "$LOG_DIR"

APP_NAME="${CURSORPRO3_APP_NAME:-CursorPro3}"
APP_BUNDLE_ID="${CURSORPRO3_BUNDLE_ID:-com.yuxin.CursorPro}"
APP_PATH="${CURSORPRO3_APP_PATH:-/Applications/CursorPro3.app}"
if [[ ! -d "$APP_PATH" && -d "/Applications/CursorPro 3.app" ]]; then
  APP_PATH="/Applications/CursorPro 3.app"
fi

if [[ -f "$PID_FILE" ]]; then
  PID="$(<"$PID_FILE")"
  if [[ -n "$PID" ]] && kill -0 "$PID" 2>/dev/null; then
    echo "CursorPro3 control server already running ($PID)."
    exit 0
  fi
  rm -f "$PID_FILE"
fi

nohup env CURSORPRO3_APP_NAME="$APP_NAME" \
  CURSORPRO3_BUNDLE_ID="$APP_BUNDLE_ID" \
  CURSORPRO3_APP_PATH="$APP_PATH" \
  /usr/bin/python3 "$ROOT_DIR/cursorpro3_builder/cursorpro3_control.py" \
  >> "$LOG_DIR/control_server.log" 2>&1 < /dev/null &

HEALTH_URL="${CURSORPRO3_CONTROL_HEALTH_URL:-http://127.0.0.1:18765/v1/health}"
for _ in {1..20}; do
  if curl -fsS -m 2 "$HEALTH_URL" >/dev/null 2>&1; then
    NEW_PID=""
    if [[ -f "$PID_FILE" ]]; then
      NEW_PID="$(<"$PID_FILE")"
    fi
    echo "CursorPro3 control server started. pid=${NEW_PID:-unknown}"
    exit 0
  fi
  sleep 0.5
done

echo "CursorPro3 control server failed health check: $HEALTH_URL" >&2
echo "See log: $LOG_DIR/control_server.log" >&2
exit 1
