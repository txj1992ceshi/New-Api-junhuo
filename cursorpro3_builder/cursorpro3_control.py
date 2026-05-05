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
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
from typing import Any


APP_NAME = os.environ.get("CURSORPRO3_APP_NAME", "CursorPro3")
APP_BUNDLE_ID = os.environ.get("CURSORPRO3_BUNDLE_ID", "com.yuxin.CursorPro")
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
AX_MAIN_BUTTON_TITLE = "一键换号"
AX_MODAL_BUTTON_TITLES = ["我知道了", "关闭", "×", "✕", "确定", "确认", "继续", "知道了", "稍后", "取消"]


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


def app_has_accessibility_permission() -> bool:
    swift_code = r"""
import AppKit
import ApplicationServices
import Foundation

let options = [kAXTrustedCheckOptionPrompt.takeUnretainedValue() as String: false] as CFDictionary
let trusted = AXIsProcessTrustedWithOptions(options)
print(trusted ? "true" : "false")
"""
    proc = subprocess.run(["swift", "-e", swift_code], capture_output=True, text=True)
    return proc.returncode == 0 and proc.stdout.strip().lower() == "true"


def run_ax_bridge(action: str, *, allow_activate: bool = False) -> dict[str, Any]:
    swift_code = r"""
import AppKit
import ApplicationServices
import Foundation

let args = CommandLine.arguments
guard args.count >= 4 else {
    fputs("invalid ax bridge arguments\n", stderr)
    exit(2)
}

let bundleID = args[1]
let action = args[2]
let allowActivate = args[3] == "1"
let modalTitles: [String]
if args.count >= 5, let data = args[4].data(using: .utf8), let values = try? JSONSerialization.jsonObject(with: data) as? [String] {
    modalTitles = values
} else {
    modalTitles = []
}

let options = [kAXTrustedCheckOptionPrompt.takeUnretainedValue() as String: false] as CFDictionary
guard AXIsProcessTrustedWithOptions(options) else {
    let payload: [String: Any] = [
        "ok": false,
        "error_code": "ax_permission_missing",
        "error_message": "Accessibility permission is required for the current Python host."
    ]
    let data = try! JSONSerialization.data(withJSONObject: payload)
    print(String(data: data, encoding: .utf8)!)
    exit(0)
}

guard let runningApp = NSRunningApplication.runningApplications(withBundleIdentifier: bundleID).first else {
    let payload: [String: Any] = [
        "ok": false,
        "error_code": "window_inaccessible",
        "error_message": "Application is not running."
    ]
    let data = try! JSONSerialization.data(withJSONObject: payload)
    print(String(data: data, encoding: .utf8)!)
    exit(0)
}

@discardableResult
func copyAttr(_ element: AXUIElement, _ attr: String, _ value: inout CFTypeRef?) -> AXError {
    return AXUIElementCopyAttributeValue(element, attr as CFString, &value)
}

func attrString(_ element: AXUIElement, _ attr: String) -> String? {
    var value: CFTypeRef?
    if copyAttr(element, attr, &value) != .success {
        return nil
    }
    if let text = value as? String {
        return text.trimmingCharacters(in: .whitespacesAndNewlines)
    }
    if let num = value as? NSNumber {
        return num.stringValue
    }
    return nil
}

func attrBool(_ element: AXUIElement, _ attr: String) -> Bool? {
    var value: CFTypeRef?
    if copyAttr(element, attr, &value) != .success {
        return nil
    }
    if let raw = value as? Bool {
        return raw
    }
    if let raw = value as? NSNumber {
        return raw.boolValue
    }
    return nil
}

func attrElements(_ element: AXUIElement, _ attr: String) -> [AXUIElement] {
    var value: CFTypeRef?
    if copyAttr(element, attr, &value) != .success {
        return []
    }
    return value as? [AXUIElement] ?? []
}

func attrElement(_ element: AXUIElement, _ attr: String) -> AXUIElement? {
    var value: CFTypeRef?
    if copyAttr(element, attr, &value) != .success {
        return nil
    }
    return value as! AXUIElement?
}

func elementSummary(_ element: AXUIElement) -> [String: Any] {
    [
        "role": attrString(element, kAXRoleAttribute as String) ?? "",
        "title": attrString(element, kAXTitleAttribute as String) ?? "",
        "description": attrString(element, kAXDescriptionAttribute as String) ?? "",
        "enabled": attrBool(element, kAXEnabledAttribute as String) ?? false
    ]
}

let appElement = AXUIElementCreateApplication(runningApp.processIdentifier)

func findMainWindow() -> AXUIElement? {
    if let focused = attrElement(appElement, kAXFocusedWindowAttribute as String) {
        return focused
    }
    if let main = attrElement(appElement, kAXMainWindowAttribute as String) {
        return main
    }
    return attrElements(appElement, kAXWindowsAttribute as String).first
}

func findWebArea(_ element: AXUIElement) -> AXUIElement? {
    let role = attrString(element, kAXRoleAttribute as String) ?? ""
    let desc = attrString(element, kAXDescriptionAttribute as String) ?? ""
    if role == "AXWebArea", desc == "CursorPro" || desc.isEmpty {
        return element
    }
    for child in attrElements(element, kAXChildrenAttribute as String) {
        if let found = findWebArea(child) {
            return found
        }
    }
    return nil
}

func findButtons(_ element: AXUIElement, matches: (String, String) -> Bool) -> [AXUIElement] {
    var results: [AXUIElement] = []
    let role = attrString(element, kAXRoleAttribute as String) ?? ""
    let title = attrString(element, kAXTitleAttribute as String) ?? ""
    let desc = attrString(element, kAXDescriptionAttribute as String) ?? ""
    if role == "AXButton", matches(title, desc) {
        results.append(element)
    }
    for child in attrElements(element, kAXChildrenAttribute as String) {
        results.append(contentsOf: findButtons(child, matches: matches))
    }
    return results
}

func preferredButtonLabel(_ title: String, _ desc: String) -> String {
    return !title.isEmpty ? title : desc
}

func containsBlockingModal(_ element: AXUIElement) -> Bool {
    let role = attrString(element, kAXRoleAttribute as String) ?? ""
    if role == "AXSheet" || role == "AXDialog" {
        return true
    }
    if attrBool(element, "AXModal") == true {
        return true
    }
    for child in attrElements(element, kAXChildrenAttribute as String) {
        if containsBlockingModal(child) {
            return true
        }
    }
    return false
}

func findSystemModalRoots(_ element: AXUIElement) -> [AXUIElement] {
    let blockingRoles = Set(["AXSheet", "AXDialog"])
    let role = attrString(element, kAXRoleAttribute as String) ?? ""
    let modal = attrBool(element, "AXModal") == true
    if blockingRoles.contains(role) || modal {
        return [element]
    }
    var searchRoots: [AXUIElement] = []
    for child in attrElements(element, kAXChildrenAttribute as String) {
        searchRoots.append(contentsOf: findSystemModalRoots(child))
    }
    return searchRoots
}

func findModalButtons(window: AXUIElement, webArea: AXUIElement?) -> [AXUIElement] {
    var searchRoots = findSystemModalRoots(window)
    if let webArea {
        searchRoots.append(webArea)
    }
    var weightedMatches: [(Int, AXUIElement)] = []
    for root in searchRoots {
        let buttons = findButtons(root, matches: { title, desc in
            let candidate = preferredButtonLabel(title, desc)
            return modalTitles.contains(candidate) && candidate != "一键换号"
        }))
        for button in buttons {
            let candidate = preferredButtonLabel(
                attrString(button, kAXTitleAttribute as String) ?? "",
                attrString(button, kAXDescriptionAttribute as String) ?? ""
            )
            guard let priority = modalTitles.firstIndex(of: candidate) else {
                continue
            }
            weightedMatches.append((priority, button))
        }
    }
    weightedMatches.sort { lhs, rhs in
        if lhs.0 != rhs.0 {
            return lhs.0 < rhs.0
        }
        let leftSummary = preferredButtonLabel(
            attrString(lhs.1, kAXTitleAttribute as String) ?? "",
            attrString(lhs.1, kAXDescriptionAttribute as String) ?? ""
        )
        let rightSummary = preferredButtonLabel(
            attrString(rhs.1, kAXTitleAttribute as String) ?? "",
            attrString(rhs.1, kAXDescriptionAttribute as String) ?? ""
        )
        return leftSummary < rightSummary
    }
    return weightedMatches.map { $0.1 }
}

func press(_ element: AXUIElement) -> Bool {
    let result = AXUIElementPerformAction(element, kAXPressAction as CFString)
    return result == .success
}

func restoreWindow(_ window: AXUIElement) {
    _ = AXUIElementPerformAction(window, kAXRaiseAction as CFString)
    _ = runningApp.activate(options: [.activateIgnoringOtherApps])
}

guard let window = findMainWindow() else {
    let payload: [String: Any] = [
        "ok": false,
        "error_code": "window_inaccessible",
        "error_message": "Unable to access the main CursorPro window."
    ]
    let data = try! JSONSerialization.data(withJSONObject: payload)
    print(String(data: data, encoding: .utf8)!)
    exit(0)
}

let webArea = findWebArea(window)
let buttonMatches = webArea.map {
    findButtons($0, matches: { title, desc in
        if title == "一键换号" {
            return true
        }
        return title.isEmpty && desc == "一键换号"
    })
} ?? []
let mainButton = buttonMatches.first
let mainEnabled = mainButton.flatMap { attrBool($0, kAXEnabledAttribute as String) } ?? false
let modalButtons = findModalButtons(window: window, webArea: webArea)
let modalDetected = containsBlockingModal(window) || !modalButtons.isEmpty
let payloadBase: [String: Any] = [
    "ok": true,
    "frontmost": runningApp.isActive,
    "modal_detected": modalDetected,
    "button_found": mainButton != nil,
    "button_enabled": mainEnabled,
    "button": mainButton.map(elementSummary) as Any,
    "modal_button_count": modalButtons.count,
    "modal_buttons": modalButtons.map(elementSummary)
]

switch action {
case "inspect":
    let data = try! JSONSerialization.data(withJSONObject: payloadBase)
    print(String(data: data, encoding: .utf8)!)
case "press-main":
    var payload = payloadBase
    if mainButton == nil {
        payload["ok"] = false
        payload["error_code"] = "button_not_found"
        payload["error_message"] = "AX button '一键换号' was not found."
    } else if !mainEnabled {
        payload["ok"] = false
        payload["error_code"] = "button_disabled"
        payload["error_message"] = "AX button '一键换号' is disabled."
    } else if let mainButton, press(mainButton) {
        payload["pressed"] = true
    } else {
        payload["ok"] = false
        payload["error_code"] = "register_trigger_failed"
        payload["error_message"] = "Failed to AXPress the '一键换号' button."
    }
    let data = try! JSONSerialization.data(withJSONObject: payload)
    print(String(data: data, encoding: .utf8)!)
case "dismiss-modal":
    var payload = payloadBase
    if let button = modalButtons.first {
        if press(button) {
            payload["dismissed"] = true
            payload["dismissed_button"] = elementSummary(button)
        } else {
            payload["ok"] = false
            payload["error_code"] = "modal_blocking_unresolved"
            payload["error_message"] = "Detected a blocking modal, but failed to dismiss it."
        }
    } else {
        payload["ok"] = false
        payload["error_code"] = "modal_blocking_unresolved"
        payload["error_message"] = "Detected a blocking modal, but no whitelisted button was found."
    }
    let data = try! JSONSerialization.data(withJSONObject: payload)
    print(String(data: data, encoding: .utf8)!)
case "restore-window":
    if allowActivate {
        restoreWindow(window)
    }
    var payload = payloadBase
    payload["restored"] = allowActivate
    let data = try! JSONSerialization.data(withJSONObject: payload)
    print(String(data: data, encoding: .utf8)!)
default:
    let payload: [String: Any] = [
        "ok": false,
        "error_code": "register_trigger_failed",
        "error_message": "Unknown AX action."
    ]
    let data = try! JSONSerialization.data(withJSONObject: payload)
    print(String(data: data, encoding: .utf8)!)
}
"""
    proc = subprocess.run(
        [
            "swift",
            "-e",
            swift_code,
            APP_BUNDLE_ID,
            action,
            "1" if allow_activate else "0",
            json.dumps(AX_MODAL_BUTTON_TITLES, ensure_ascii=False),
        ],
        capture_output=True,
        text=True,
    )
    if proc.returncode != 0:
        raise RuntimeError(proc.stderr.strip() or proc.stdout.strip() or "failed to execute AX bridge")
    try:
        payload = json.loads(proc.stdout.strip() or "{}")
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"failed to parse AX bridge payload: {proc.stdout.strip()!r}") from exc
    return payload


def inspect_ax_state(*, allow_activate: bool = False) -> dict[str, Any]:
    return run_ax_bridge("inspect", allow_activate=allow_activate)


def restore_ax_window() -> dict[str, Any]:
    return run_ax_bridge("restore-window", allow_activate=True)


def press_ax_main_button() -> dict[str, Any]:
    return run_ax_bridge("press-main", allow_activate=False)


def dismiss_ax_modal() -> dict[str, Any]:
    return run_ax_bridge("dismiss-modal", allow_activate=False)


def trigger_ax_register_flow() -> dict[str, Any]:
    state.set_state(error_message="ax_button_scan_started")
    inspect = inspect_ax_state(allow_activate=False)
    if not inspect.get("ok", False):
        if inspect.get("error_code") == "ax_permission_missing":
            raise RuntimeError("ax_permission_missing")
        state.set_state(error_message="ax_window_restore_started")
        restore = restore_ax_window()
        if not restore.get("ok", False):
            raise RuntimeError(restore.get("error_code") or "window_inaccessible")
        inspect = inspect_ax_state(allow_activate=True)
        if not inspect.get("ok", False):
            raise RuntimeError(inspect.get("error_code") or "window_inaccessible")

    if inspect.get("button_found") and inspect.get("button_enabled"):
        state.set_state(error_message="ax_button_pressed")
        press_result = press_ax_main_button()
        if press_result.get("ok") and press_result.get("pressed"):
            log("triggered one-click register via AXPress")
            return press_result
        raise RuntimeError(press_result.get("error_code") or "register_trigger_failed")

    if inspect.get("modal_detected") or inspect.get("button_found") or inspect.get("button_enabled") is False:
        state.set_state(error_message="ax_web_modal_fallback_started")
        dismiss_result = dismiss_ax_modal()
        if not dismiss_result.get("ok") or not dismiss_result.get("dismissed"):
            raise RuntimeError(dismiss_result.get("error_code") or "modal_blocking_unresolved")
        time.sleep(0.4)
        inspect = inspect_ax_state(allow_activate=False)
        if inspect.get("button_found") and inspect.get("button_enabled"):
            state.set_state(error_message="ax_button_pressed")
            press_result = press_ax_main_button()
            if press_result.get("ok") and press_result.get("pressed"):
                log("triggered one-click register via AXPress after modal dismissal")
                return press_result
            raise RuntimeError(press_result.get("error_code") or "register_trigger_failed")
        raise RuntimeError("modal_blocking_unresolved")

    state.set_state(error_message="ax_window_restore_started")
    restore = restore_ax_window()
    if not restore.get("ok", False):
        raise RuntimeError(restore.get("error_code") or "window_inaccessible")
    inspect = inspect_ax_state(allow_activate=True)
    if inspect.get("button_found") and inspect.get("button_enabled"):
        state.set_state(error_message="ax_button_pressed")
        press_result = press_ax_main_button()
        if press_result.get("ok") and press_result.get("pressed"):
            log("triggered one-click register via AXPress after window restore")
            return press_result
        raise RuntimeError(press_result.get("error_code") or "register_trigger_failed")
    raise RuntimeError("button_not_found")


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
        if not app_has_accessibility_permission():
            raise RuntimeError("ax_permission_missing")
        trigger_ax_register_flow()

        deadline = time.time() + REGISTER_TIMEOUT_SECONDS
        state.set_state(error_message="waiting_for_token_yield")
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
        error_code = str(exc).strip() or "register_trigger_failed"
        if error_code not in {
            "button_not_found",
            "button_disabled",
            "modal_blocking_unresolved",
            "window_inaccessible",
            "ax_permission_missing",
            "register_timeout",
            "register_trigger_failed",
        }:
            error_code = "register_trigger_failed"
        state.set_state(
            status="failed",
            finished_at=now_iso(),
            created_count=0,
            updated_count=0,
            error_code=error_code,
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
