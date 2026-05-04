#!/usr/bin/env python3
from __future__ import annotations

import json
import os
import signal
import socketserver
import subprocess
import threading
import time
import uuid
import ctypes
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
from typing import Any


APP_NAME = os.environ.get("CURSORPRO3_APP_NAME", "CursorPro 3")
APP_BUNDLE_ID = os.environ.get("CURSORPRO3_BUNDLE_ID", "com.yuxin.CursorPr3")
APP_BUNDLE_PATH = Path(os.environ.get("CURSORPRO3_APP_PATH", Path(__file__).resolve().parents[2]))
SOURCE_TOKEN_DIR = Path.home() / "Library/Application Support/NVIDIA_NV/codex_tokens"
STATE_ROOT = Path.home() / "Library/Application Support/CursorPro3"
EXPORT_DIR = STATE_ROOT / "exports" / "codex"
LOG_DIR = STATE_ROOT / "logs"
STATE_FILE = STATE_ROOT / "control_state.json"
PID_FILE = STATE_ROOT / "control_server.pid"
LOG_FILE = LOG_DIR / "control_server.log"
PORT = int(os.environ.get("CURSORPRO3_CONTROL_PORT", "18765"))
HOST = "127.0.0.1"
REGISTER_TIMEOUT_SECONDS = int(os.environ.get("CURSORPRO3_REGISTER_TIMEOUT_SECONDS", "300"))
POLL_INTERVAL_SECONDS = float(os.environ.get("CURSORPRO3_POLL_INTERVAL_SECONDS", "2"))
SOURCE_SYNC_INTERVAL_SECONDS = float(os.environ.get("CURSORPRO3_SOURCE_SYNC_INTERVAL_SECONDS", "5"))


def now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def parse_iso_datetime(raw: Any) -> datetime | None:
    text = str(raw or "").strip()
    if not text:
        return None
    try:
        return datetime.fromisoformat(text)
    except Exception:
        return None


def log(message: str) -> None:
    LOG_DIR.mkdir(parents=True, exist_ok=True)
    line = f"[{now_iso()}] {message}\n"
    with LOG_FILE.open("a", encoding="utf-8") as fh:
        fh.write(line)


class AppState:
    def __init__(self) -> None:
        self.lock = threading.Lock()
        self.task_state: dict[str, Any] = {
            "task_id": None,
            "status": "idle",
            "started_at": None,
            "finished_at": None,
            "created_count": 0,
            "updated_count": 0,
            "error_code": None,
            "error_message": None,
            "last_export_at": None,
            "last_export_count": 0,
            "source_token_count": 0,
            "source_latest_file": None,
            "source_latest_mtime": None,
            "export_token_count": 0,
            "export_latest_file": None,
            "export_latest_mtime": None,
            "last_sync_at": None,
            "last_sync_result": None,
            "last_source_to_export_reason": None,
            "sync_lag_seconds": None,
            "last_source_signature": {},
        }
        self.load()

    def load(self) -> None:
        if STATE_FILE.exists():
            try:
                self.task_state.update(json.loads(STATE_FILE.read_text(encoding="utf-8")))
            except Exception as exc:
                log(f"failed to load state file: {exc}")
        self.reconcile_running_after_restart()

    def save(self) -> None:
        STATE_ROOT.mkdir(parents=True, exist_ok=True)
        tmp = STATE_FILE.with_suffix(".tmp")
        tmp.write_text(json.dumps(self.task_state, indent=2, ensure_ascii=False), encoding="utf-8")
        tmp.replace(STATE_FILE)

    def snapshot(self) -> dict[str, Any]:
        self.reconcile_running_timeout()
        with self.lock:
            return dict(self.task_state)

    def set_state(self, **updates: Any) -> dict[str, Any]:
        with self.lock:
            self.task_state.update(updates)
            self.save()
            return dict(self.task_state)

    def reconcile_running_timeout(self) -> dict[str, Any] | None:
        with self.lock:
            if self.task_state.get("status") != "running":
                return None
            started_at = parse_iso_datetime(self.task_state.get("started_at"))
            finished_at = self.task_state.get("finished_at")
            if finished_at:
                self.task_state["status"] = "failed"
                self.task_state["error_code"] = self.task_state.get("error_code") or "register_state_inconsistent"
                self.task_state["error_message"] = self.task_state.get("error_message") or "Register task was marked running after it had already finished."
                self.save()
                log("recovered inconsistent register task state: running with finished_at set")
                return dict(self.task_state)
            if started_at is None:
                self.task_state.update(
                    status="failed",
                    finished_at=now_iso(),
                    created_count=0,
                    updated_count=0,
                    error_code="register_state_inconsistent",
                    error_message="Register task was marked running without a valid started_at timestamp.",
                )
                self.save()
                log("recovered inconsistent register task state: missing started_at")
                return dict(self.task_state)
            grace_seconds = max(5.0, POLL_INTERVAL_SECONDS * 2)
            deadline_ts = started_at.timestamp() + REGISTER_TIMEOUT_SECONDS + grace_seconds
            if time.time() <= deadline_ts:
                return None
            self.task_state.update(
                status="failed",
                finished_at=now_iso(),
                created_count=0,
                updated_count=0,
                error_code="register_timeout",
                error_message="Previous register task exceeded the timeout window and was auto-recovered.",
            )
            self.save()
            log("recovered stale register task after timeout window elapsed")
            return dict(self.task_state)

    def reconcile_running_after_restart(self) -> dict[str, Any] | None:
        with self.lock:
            if self.task_state.get("status") != "running":
                return None
            self.task_state.update(
                status="failed",
                finished_at=now_iso(),
                created_count=0,
                updated_count=0,
                error_code="register_interrupted",
                error_message="Previous register task was interrupted when the control server restarted.",
            )
            self.save()
            log("recovered orphaned register task after control server restart")
            return dict(self.task_state)


state = AppState()


def summarize_source_tokens() -> tuple[list[dict[str, Any]], dict[str, dict[str, Any]]]:
    items: list[dict[str, Any]] = []
    raw_by_file: dict[str, dict[str, Any]] = {}
    if not SOURCE_TOKEN_DIR.exists():
        return items, raw_by_file
    for path in sorted(SOURCE_TOKEN_DIR.glob("*.json")):
        try:
            raw = json.loads(path.read_text(encoding="utf-8"))
        except Exception as exc:
            log(f"failed to parse token file {path}: {exc}")
            continue
        raw_by_file[path.name] = raw
        items.append(
            {
                "filename": path.name,
                "provider": raw.get("type", "codex"),
                "account_id": raw.get("account_id"),
                "email": raw.get("email"),
                "token_type": "oauth_token_bundle",
                "expires_at": raw.get("expired"),
                "source": "cursorpro3",
                "status": "new",
                "updated_at": datetime.fromtimestamp(path.stat().st_mtime, timezone.utc).isoformat(),
            }
        )
    return items, raw_by_file


def summarize_dir_snapshot(directory: Path) -> dict[str, Any]:
    count = 0
    latest_file = None
    latest_mtime = None
    latest_ts = 0.0
    if not directory.exists():
        return {
            "count": 0,
            "latest_file": None,
            "latest_mtime": None,
        }
    for path in sorted(directory.glob("*.json")):
        try:
            stat = path.stat()
        except OSError:
            continue
        count += 1
        if stat.st_mtime > latest_ts:
            latest_ts = stat.st_mtime
            latest_file = path.name
            latest_mtime = datetime.fromtimestamp(stat.st_mtime, timezone.utc).isoformat()
    return {
        "count": count,
        "latest_file": latest_file,
        "latest_mtime": latest_mtime,
    }


def build_source_signature() -> dict[str, list[int]]:
    signature: dict[str, list[int]] = {}
    if not SOURCE_TOKEN_DIR.exists():
        return signature
    for path in sorted(SOURCE_TOKEN_DIR.glob("*.json")):
        try:
            stat = path.stat()
        except OSError:
            continue
        signature[path.name] = [stat.st_size, int(stat.st_mtime_ns)]
    return signature


def refresh_sync_state(
    *,
    sync_result: str | None = None,
    sync_reason: str | None = None,
    source_signature: dict[str, list[int]] | None = None,
    sync_at: str | None = None,
) -> dict[str, Any]:
    source_snapshot = summarize_dir_snapshot(SOURCE_TOKEN_DIR)
    export_snapshot = summarize_dir_snapshot(EXPORT_DIR)

    lag_seconds = None
    if source_snapshot["latest_mtime"] and export_snapshot["latest_mtime"]:
        try:
            src = datetime.fromisoformat(source_snapshot["latest_mtime"])
            exp = datetime.fromisoformat(export_snapshot["latest_mtime"])
            lag_seconds = max(0.0, (src - exp).total_seconds())
        except Exception:
            lag_seconds = None

    updates: dict[str, Any] = {
        "source_token_count": source_snapshot["count"],
        "source_latest_file": source_snapshot["latest_file"],
        "source_latest_mtime": source_snapshot["latest_mtime"],
        "export_token_count": export_snapshot["count"],
        "export_latest_file": export_snapshot["latest_file"],
        "export_latest_mtime": export_snapshot["latest_mtime"],
        "sync_lag_seconds": lag_seconds,
    }
    if sync_result is not None:
        updates["last_sync_result"] = sync_result
    if sync_reason is not None:
        updates["last_source_to_export_reason"] = sync_reason
    if sync_at is not None:
        updates["last_sync_at"] = sync_at
    if source_signature is not None:
        updates["last_source_signature"] = source_signature
    return state.set_state(**updates)


def export_tokens() -> dict[str, Any]:
    EXPORT_DIR.mkdir(parents=True, exist_ok=True)
    items, raw_by_file = summarize_source_tokens()
    exported = 0
    for item in items:
        filename = item["filename"]
        raw = raw_by_file[filename]
        payload = {
            **item,
            "exported_at": now_iso(),
            "raw": {
                "access_token": raw.get("access_token"),
                "refresh_token": raw.get("refresh_token"),
                "id_token": raw.get("id_token"),
            },
        }
        out = EXPORT_DIR / filename
        tmp = out.with_suffix(".tmp")
        tmp.write_text(json.dumps(payload, indent=2, ensure_ascii=False), encoding="utf-8")
        tmp.replace(out)
        exported += 1
    result = {"exported_count": exported, "exported_at": now_iso()}
    state.set_state(last_export_at=result["exported_at"], last_export_count=exported)
    refresh_sync_state(sync_result="export_written")
    log(f"exported {exported} token files")
    return result


def sync_source_tokens(force: bool = False, reason: str = "manual") -> dict[str, Any]:
    now = now_iso()
    before_signature = build_source_signature()
    snapshot = state.snapshot()
    last_signature = snapshot.get("last_source_signature") or {}
    source_changed = before_signature != last_signature

    if force or source_changed:
        result = export_tokens()
        sync_result = "export_written" if result.get("exported_count", 0) > 0 else "source_token_detected"
        payload = refresh_sync_state(
            sync_result=sync_result,
            sync_reason=reason,
            source_signature=before_signature,
            sync_at=now,
        )
        return {
            "changed": source_changed,
            "forced": force,
            "reason": reason,
            "result": payload.get("last_sync_result"),
            "state": payload,
        }

    payload = refresh_sync_state(
        sync_result="noop",
        sync_reason=reason,
        source_signature=before_signature,
        sync_at=now,
    )
    return {
        "changed": False,
        "forced": force,
        "reason": reason,
        "result": payload.get("last_sync_result"),
        "state": payload,
    }


def snapshot_files() -> dict[str, tuple[int, int]]:
    snap: dict[str, tuple[int, int]] = {}
    if not SOURCE_TOKEN_DIR.exists():
        return snap
    for path in SOURCE_TOKEN_DIR.glob("*.json"):
        st = path.stat()
        snap[path.name] = (st.st_size, int(st.st_mtime_ns))
    return snap


def diff_snapshots(before: dict[str, tuple[int, int]], after: dict[str, tuple[int, int]]) -> tuple[int, int]:
    created = 0
    updated = 0
    for name, meta in after.items():
        if name not in before:
            created += 1
        elif before[name] != meta:
            updated += 1
    return created, updated


def is_app_running() -> bool:
    script = f'tell application id "{APP_BUNDLE_ID}" to return running'
    proc = subprocess.run(["osascript", "-e", script], capture_output=True, text=True)
    return proc.returncode == 0 and proc.stdout.strip().lower() == "true"


def launch_app() -> None:
    subprocess.run(["open", str(APP_BUNDLE_PATH)], check=False)


def get_window_geometry() -> tuple[float, float, float, float]:
    swift_code = r"""
import AppKit
import CoreGraphics
import Foundation

let args = CommandLine.arguments
guard args.count == 3 else {
    fputs("invalid geometry arguments\n", stderr)
    exit(2)
}

let bundleID = args[1]
let fallbackName = args[2]
let runningApp = NSRunningApplication.runningApplications(withBundleIdentifier: bundleID).first
let targetPID = runningApp?.processIdentifier ?? 0

guard let rawWindowList = CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID) as? [[String: Any]] else {
    fputs("failed to read window list\n", stderr)
    exit(3)
}

func numberValue(_ raw: Any?) -> Double? {
    if let v = raw as? Double { return v }
    if let v = raw as? CGFloat { return Double(v) }
    if let v = raw as? Int { return Double(v) }
    if let v = raw as? NSNumber { return v.doubleValue }
    return nil
}

for window in rawWindowList {
    let ownerPID = (window[kCGWindowOwnerPID as String] as? NSNumber)?.int32Value ?? 0
    if targetPID != 0 && ownerPID != targetPID {
        continue
    }
    if targetPID == 0 {
        let ownerName = (window[kCGWindowOwnerName as String] as? String) ?? ""
        if !ownerName.localizedCaseInsensitiveContains(fallbackName) {
            continue
        }
    }
    let layer = (window[kCGWindowLayer as String] as? NSNumber)?.intValue ?? 0
    if layer != 0 {
        continue
    }
    guard let bounds = window[kCGWindowBounds as String] as? [String: Any],
          let x = numberValue(bounds["X"]),
          let y = numberValue(bounds["Y"]),
          let width = numberValue(bounds["Width"]),
          let height = numberValue(bounds["Height"]),
          width > 0,
          height > 0 else {
        continue
    }
    print("\(x),\(y),\(width),\(height)")
    exit(0)
}

fputs("failed to locate app window\n", stderr)
exit(4)
"""
    proc = subprocess.run(
        ["swift", "-e", swift_code, APP_BUNDLE_ID, APP_NAME],
        capture_output=True,
        text=True,
    )
    if proc.returncode != 0:
        raise RuntimeError(proc.stderr.strip() or proc.stdout.strip() or "failed to locate app window")
    parts = [item.strip() for item in proc.stdout.strip().split(",") if item.strip()]
    if len(parts) != 4:
        raise RuntimeError(f"unexpected window geometry payload: {proc.stdout.strip()!r}")
    try:
        return tuple(float(item) for item in parts)  # type: ignore[return-value]
    except ValueError as exc:
        raise RuntimeError(f"failed to parse window geometry: {proc.stdout.strip()!r}") from exc


def perform_native_click(x: float, y: float) -> None:
    class CGPoint(ctypes.Structure):
        _fields_ = [("x", ctypes.c_double), ("y", ctypes.c_double)]

    application_services = ctypes.cdll.LoadLibrary(
        "/System/Library/Frameworks/ApplicationServices.framework/ApplicationServices"
    )
    core_foundation = ctypes.cdll.LoadLibrary(
        "/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation"
    )

    application_services.CGEventCreateMouseEvent.restype = ctypes.c_void_p
    application_services.CGEventCreateMouseEvent.argtypes = [
        ctypes.c_void_p,
        ctypes.c_uint32,
        CGPoint,
        ctypes.c_uint32,
    ]
    application_services.CGEventPost.argtypes = [ctypes.c_uint32, ctypes.c_void_p]
    core_foundation.CFRelease.argtypes = [ctypes.c_void_p]

    point = CGPoint(float(x), float(y))
    kCGHIDEventTap = 0
    kCGEventLeftMouseDown = 1
    kCGEventLeftMouseUp = 2
    kCGMouseButtonLeft = 0

    down = application_services.CGEventCreateMouseEvent(
        None, kCGEventLeftMouseDown, point, kCGMouseButtonLeft
    )
    up = application_services.CGEventCreateMouseEvent(
        None, kCGEventLeftMouseUp, point, kCGMouseButtonLeft
    )
    if not down or not up:
        raise RuntimeError("failed to create mouse events")
    try:
        application_services.CGEventPost(kCGHIDEventTap, down)
        application_services.CGEventPost(kCGHIDEventTap, up)
    finally:
        core_foundation.CFRelease(down)
        core_foundation.CFRelease(up)


def click_main_button() -> None:
    subprocess.run(["osascript", "-e", f'tell application id "{APP_BUNDLE_ID}" to activate'], check=False)
    time.sleep(0.6)
    left, top, width, height = get_window_geometry()
    target_x = left + (width / 2)
    target_y = top + (height * 0.5)
    perform_native_click(target_x, target_y)


def run_register_task() -> None:
    with state.lock:
        if state.task_state.get("status") == "running":
            return
        task_id = str(uuid.uuid4())
        state.task_state.update(
            {
                "task_id": task_id,
                "status": "running",
                "started_at": now_iso(),
                "finished_at": None,
                "created_count": 0,
                "updated_count": 0,
                "error_code": None,
                "error_message": None,
            }
        )
        state.save()

    before = snapshot_files()
    try:
        if not is_app_running():
            launch_app()
            deadline = time.time() + 20
            while time.time() < deadline and not is_app_running():
                time.sleep(0.5)
        click_main_button()
        log("triggered one-click register via UI automation")

        deadline = time.time() + REGISTER_TIMEOUT_SECONDS
        created = 0
        updated = 0
        while time.time() < deadline:
            time.sleep(POLL_INTERVAL_SECONDS)
            after = snapshot_files()
            created, updated = diff_snapshots(before, after)
            if created > 0 or updated > 0:
                export_tokens()
                state.set_state(
                    status="succeeded",
                    finished_at=now_iso(),
                    created_count=created,
                    updated_count=updated,
                    error_code=None,
                    error_message=None,
                )
                return
        export_tokens()
        state.set_state(
            status="failed",
            finished_at=now_iso(),
            created_count=0,
            updated_count=0,
            error_code="register_timeout",
            error_message="No token file changes were detected before timeout.",
        )
    except Exception as exc:
        state.set_state(
            status="failed",
            finished_at=now_iso(),
            created_count=0,
            updated_count=0,
            error_code="register_trigger_failed",
            error_message=str(exc),
        )
        log(f"register task failed: {exc}")


def start_register_task() -> tuple[dict[str, Any], int]:
    state.reconcile_running_timeout()
    snap = state.snapshot()
    if snap.get("status") == "running":
        return snap, 409
    thread = threading.Thread(target=run_register_task, daemon=True)
    thread.start()
    time.sleep(0.1)
    return state.snapshot(), 202


def health_payload() -> dict[str, Any]:
    return {
        "ok": True,
        "app_name": APP_NAME,
        "app_running": is_app_running(),
        "source_token_dir": str(SOURCE_TOKEN_DIR),
        "source_token_dir_exists": SOURCE_TOKEN_DIR.exists(),
        "export_dir": str(EXPORT_DIR),
        "export_dir_exists": EXPORT_DIR.exists(),
        "token_status": refresh_sync_state(),
        "state": state.snapshot(),
    }


def run_source_sync_loop() -> None:
    while True:
        try:
            sync_source_tokens(force=False, reason="source_poll")
        except Exception as exc:
            log(f"background source sync failed: {exc}")
        time.sleep(SOURCE_SYNC_INTERVAL_SECONDS)


class Handler(BaseHTTPRequestHandler):
    server_version = "CursorPro3Control/0.1"

    def log_message(self, fmt: str, *args: Any) -> None:
        log(fmt % args)

    def _send(self, status_code: int, payload: dict[str, Any]) -> None:
        data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        self.send_response(status_code)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def _read_json(self) -> dict[str, Any]:
        length = int(self.headers.get("Content-Length", "0"))
        if not length:
            return {}
        raw = self.rfile.read(length)
        if not raw:
            return {}
        return json.loads(raw.decode("utf-8"))

    def do_GET(self) -> None:
        if self.path == "/v1/health":
            self._send(200, health_payload())
            return
        if self.path == "/v1/register/status":
            self._send(200, state.snapshot())
            return
        if self.path in {"/v1/tokens/status", "/v1/token/status"}:
            self._send(200, refresh_sync_state())
            return
        if self.path == "/v1/tokens":
            items, _ = summarize_source_tokens()
            self._send(200, {"items": items, "count": len(items)})
            return
        self._send(404, {"error": "not_found"})

    def do_POST(self) -> None:
        if self.path == "/v1/register/trigger":
            _ = self._read_json()
            payload, code = start_register_task()
            self._send(code, payload)
            return
        if self.path == "/v1/tokens/export":
            _ = self._read_json()
            result = sync_source_tokens(force=True, reason="manual_export")
            self._send(200, result)
            return
        if self.path == "/v1/tokens/sync":
            payload = self._read_json()
            result = sync_source_tokens(force=bool(payload.get("force")), reason=str(payload.get("reason") or "manual_sync"))
            self._send(200, result)
            return
        self._send(404, {"error": "not_found"})


class ThreadedHTTPServer(socketserver.ThreadingMixIn, HTTPServer):
    daemon_threads = True


def handle_signal(signum: int, _frame: Any) -> None:
    log(f"received signal {signum}, exiting")
    try:
        PID_FILE.unlink(missing_ok=True)
    except Exception:
        pass
    raise SystemExit(0)


def main() -> None:
    STATE_ROOT.mkdir(parents=True, exist_ok=True)
    LOG_DIR.mkdir(parents=True, exist_ok=True)
    PID_FILE.write_text(str(os.getpid()), encoding="utf-8")
    signal.signal(signal.SIGTERM, handle_signal)
    signal.signal(signal.SIGINT, handle_signal)
    try:
        sync_source_tokens(force=True, reason="startup")
    except Exception as exc:
        log(f"initial export failed: {exc}")
    threading.Thread(target=run_source_sync_loop, daemon=True).start()
    httpd = ThreadedHTTPServer((HOST, PORT), Handler)
    log(f"control server listening on http://{HOST}:{PORT}")
    try:
        httpd.serve_forever()
    finally:
        PID_FILE.unlink(missing_ok=True)


if __name__ == "__main__":
    main()
