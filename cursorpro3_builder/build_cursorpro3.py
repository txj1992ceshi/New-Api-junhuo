#!/usr/bin/env python3
from __future__ import annotations

import shutil
import subprocess
from pathlib import Path


SRC_APP = Path("/Applications/CursorPro.app")
OUT_APP = Path("/Applications/CursorPro 3.app")
BUILDER_DIR = Path("/Users/jj/Documents/Playground/cursorpro3_builder")
OLD_BUNDLE_ID = b"com.yuxin.CursorPro"
NEW_BUNDLE_ID = b"com.yuxin.CursorPr3"


def run(*args: str) -> None:
    subprocess.run(list(args), check=True)


def main() -> None:
    if OUT_APP.exists():
        shutil.rmtree(OUT_APP)
    shutil.copytree(SRC_APP, OUT_APP, symlinks=True)

    info = OUT_APP / "Contents/Info.plist"
    run("/usr/libexec/PlistBuddy", "-c", "Set :CFBundleName CursorPro 3", str(info))
    run("/usr/libexec/PlistBuddy", "-c", "Set :CFBundleDisplayName CursorPro 3", str(info))
    run("/usr/libexec/PlistBuddy", "-c", "Set :CFBundleIdentifier com.yuxin.CursorPr3", str(info))
    run("/usr/libexec/PlistBuddy", "-c", "Set :CFBundleExecutable CursorPro", str(info))

    binary = OUT_APP / "Contents/MacOS/CursorPro"
    blob = binary.read_bytes()
    count = blob.count(OLD_BUNDLE_ID)
    if count == 0:
        raise RuntimeError("embedded bundle id not found in binary")
    binary.write_bytes(blob.replace(OLD_BUNDLE_ID, NEW_BUNDLE_ID))

    resources_dir = OUT_APP / "Contents/Resources"
    control = BUILDER_DIR / "cursorpro3_control.py"
    target_control = resources_dir / "cursorpro3_control.py"
    shutil.copy2(control, target_control)
    target_control.chmod(0o755)

    run("codesign", "--force", "--deep", "--sign", "-", str(OUT_APP))
    print(OUT_APP)


if __name__ == "__main__":
    main()
