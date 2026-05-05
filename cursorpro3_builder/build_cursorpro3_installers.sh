#!/bin/zsh
set -euo pipefail

ROOT="/Users/jj/Documents/Playground/cursorpro3_builder"
OUT_DIR="/Users/jj/Documents/CursorPro3"
mkdir -p "$OUT_DIR"

python3 "$ROOT/build_cursorpro3_win_stage.py"
zsh "$ROOT/build_cursorpro3_dmg.sh" "/Applications/CursorPro 3.app" "$OUT_DIR/CursorPro3.dmg" "CursorPro 3"
makensis "$ROOT/build_cursorpro3_win.nsi"

echo "Built:"
echo "  $OUT_DIR/CursorPro3.dmg"
echo "  $OUT_DIR/CursorPro3 x64-setup.exe"
