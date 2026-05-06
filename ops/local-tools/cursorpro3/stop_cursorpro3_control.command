#!/bin/zsh
set -euo pipefail

PID_FILE="$HOME/Library/Application Support/CursorPro3/control_server.pid"
STOPPED=0

if [[ -f "$PID_FILE" ]]; then
  PID="$(<"$PID_FILE")"
  if [[ -n "$PID" ]] && kill -0 "$PID" 2>/dev/null; then
    kill "$PID"
    for _ in {1..20}; do
      if ! kill -0 "$PID" 2>/dev/null; then
        break
      fi
      sleep 0.2
    done
    echo "Stopped CursorPro3 control server ($PID)."
    STOPPED=1
  fi
  rm -f "$PID_FILE"
fi

PIDS="$(pgrep -f "cursorpro3_builder/cursorpro3_control.py|CursorPro3.app/Contents/Resources/cursorpro3_control.py" || true)"
if [[ -n "$PIDS" ]]; then
  while IFS= read -r pid; do
    [[ -z "$pid" ]] && continue
    kill "$pid" 2>/dev/null || true
    STOPPED=1
  done <<< "$PIDS"
fi

if [[ "$STOPPED" -eq 1 ]]; then
  echo "CursorPro3 control server stopped."
else
  echo "CursorPro3 control server is not running."
fi
