#!/bin/zsh
set -euo pipefail

PID_FILE="$HOME/Library/Application Support/CursorPro3/control_server.pid"

if [[ -f "$PID_FILE" ]]; then
  PID="$(cat "$PID_FILE")"
  if kill -0 "$PID" 2>/dev/null; then
    kill "$PID"
    echo "Stopped CursorPro3 control server ($PID)."
    exit 0
  fi
fi

echo "CursorPro3 control server is not running."
