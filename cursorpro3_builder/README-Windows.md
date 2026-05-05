# CursorPro 3 Windows Notes

## 安装后目录

默认安装目录：

```text
C:\Program Files\CursorPro3 x64
```

安装目录内关键文件：

- `CursorPro.exe`
- `cursorpro3_control.ps1`
- `cursorpro3_register_worker.ps1`
- `start_cursorpro3_control.cmd`
- `stop_cursorpro3_control.cmd`
- `tutorial_local.html`

## 运行时数据目录

CursorPro 3 控制层使用独立目录：

```text
%APPDATA%\CursorPro3
```

其中：

- `control_state.json`：控制层任务状态
- `control_server.pid`：控制层进程 pid
- `logs\control_server.log`：控制层日志
- `exports\codex\*.json`：导出的 token 镜像

原始 token 读取目录保持兼容：

```text
%APPDATA%\NVIDIA_NV\codex_tokens
```

## 控制层接口

默认只监听本机：

```text
http://127.0.0.1:18765
```

接口：

- `GET /v1/health`
- `GET /v1/register/status`
- `GET /v1/tokens`
- `POST /v1/tokens/export`
- `POST /v1/register/trigger`

## Windows 上机验证顺序

1. 安装 `CursorPro3 x64-setup.exe`
2. 启动 `CursorPro3 x64`
3. 手动确认主界面能打开，且主按钮仍是 `一键换号`
4. 运行 `Start CursorPro3 Control`
5. 用浏览器或 curl 打开 `http://127.0.0.1:18765/v1/health`
6. 检查 `%APPDATA%\CursorPro3\exports\codex`
7. 再测 `POST /v1/tokens/export`
8. 最后才测 `POST /v1/register/trigger`

## 最后点火验证

真正上线前必须在 Windows 机器上补做这三项：

1. `POST /v1/tokens/export` 后导出文件是否完整
2. `POST /v1/register/trigger` 是否能精准触发 `CursorPro3` 自己的一键换号
3. 触发后是否真的有新 token 或更新 token 落到 `%APPDATA%\NVIDIA_NV\codex_tokens`
