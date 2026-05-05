#!/bin/zsh
set -euo pipefail

APP_PATH="${1:-/Applications/CursorPro 3.app}"
OUT_DMG="${2:-/Users/jj/Documents/CursorPro3/CursorPro3.dmg}"
VOL_NAME="${3:-CursorPro 3}"

if [[ ! -d "$APP_PATH" ]]; then
  echo "App not found: $APP_PATH" >&2
  exit 1
fi

OUT_DIR="$(dirname "$OUT_DMG")"
mkdir -p "$OUT_DIR"

WORK_DIR="$(mktemp -d /tmp/cursorpro3_dmg_work.XXXXXX)"
STAGE_DIR="$WORK_DIR/stage"
RW_DMG="$WORK_DIR/CursorPro3-rw.dmg"
TMP_OUT_DMG="$WORK_DIR/CursorPro3.dmg"
APP_NAME="$(basename "$APP_PATH")"

cleanup() {
  hdiutil detach "/Volumes/$VOL_NAME" -force >/dev/null 2>&1 || true
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

mkdir -p "$STAGE_DIR"
ditto "$APP_PATH" "$STAGE_DIR/$APP_NAME"
ln -s /Applications "$STAGE_DIR/Applications"

SIZE_MB=$(du -sm "$STAGE_DIR" | awk '{print $1 + 32}')
rm -f "$RW_DMG" "$TMP_OUT_DMG" "$OUT_DMG"

hdiutil create -srcfolder "$STAGE_DIR" -volname "$VOL_NAME" -fs HFS+ -fsargs "-c c=64,a=16,e=16" -format UDRW -size "${SIZE_MB}m" "$RW_DMG" >/dev/null
hdiutil attach "$RW_DMG" -mountpoint "/Volumes/$VOL_NAME" -nobrowse -noverify -noautoopen >/dev/null

osascript <<APPLESCRIPT
tell application "Finder"
  tell disk "$VOL_NAME"
    open
    set current view of container window to icon view
    set toolbar visible of container window to false
    set statusbar visible of container window to false
    set pathbar visible of container window to false
    set bounds of container window to {140, 120, 820, 520}
    set theViewOptions to the icon view options of container window
    set arrangement of theViewOptions to not arranged
    set icon size of theViewOptions to 96
    set text size of theViewOptions to 16
    set label position of theViewOptions to bottom
    set position of item "$APP_NAME" of container window to {180, 245}
    set position of item "Applications" of container window to {520, 245}
    close
    open
    update without registering applications
    delay 2
    close
  end tell
end tell
APPLESCRIPT

sync
hdiutil detach "/Volumes/$VOL_NAME" >/dev/null
sleep 2
hdiutil convert "$RW_DMG" -format UDZO -imagekey zlib-level=9 -ov -o "$TMP_OUT_DMG" >/dev/null
mv "$TMP_OUT_DMG" "$OUT_DMG"
echo "$OUT_DMG"
