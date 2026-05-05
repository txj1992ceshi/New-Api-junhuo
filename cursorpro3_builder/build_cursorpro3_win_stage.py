#!/usr/bin/env python3
from __future__ import annotations

import shutil
from pathlib import Path


SRC_STAGE = Path("/Users/jj/Documents/Playground/CursorPro_unpack/win_stage")
DST_STAGE = Path("/Users/jj/Documents/Playground/cursorpro3_builder/win_stage")
WINDOWS_SUPPORT = Path("/Users/jj/Documents/Playground/cursorpro3_builder/windows")
OLD_BUNDLE_ID = b"com.yuxin.CursorPro"
NEW_BUNDLE_ID = b"com.yuxin.CursorPr3"


def main() -> None:
    if DST_STAGE.exists():
        shutil.rmtree(DST_STAGE)
    shutil.copytree(SRC_STAGE, DST_STAGE)

    exe = DST_STAGE / "CursorPro.exe"
    blob = exe.read_bytes()
    if OLD_BUNDLE_ID in blob:
        blob = blob.replace(OLD_BUNDLE_ID, NEW_BUNDLE_ID)
    exe.write_bytes(blob)

    src_icon = DST_STAGE / "CursorPro2.ico"
    dst_icon = DST_STAGE / "CursorPro3.ico"
    if src_icon.exists():
        shutil.copy2(src_icon, dst_icon)

    for extra in (
        "cursorpro3_control.ps1",
        "cursorpro3_register_worker.ps1",
        "start_cursorpro3_control.cmd",
        "stop_cursorpro3_control.cmd",
        "cursorpro3_endpoints.txt",
        "cursorpro3_layout.txt",
    ):
        shutil.copy2(WINDOWS_SUPPORT / extra, DST_STAGE / extra)

    print(DST_STAGE)


if __name__ == "__main__":
    main()
