# `junhuo` Cursor / Kiro 本地最小池服务 MVP

这套 MVP 先解决一件事：

- 把本机已登录的 `Cursor / Kiro` 登录态包装成 `3401 / 3501` 两条最小外部池服务

当前版本实现了：

- `GET /auth/status`
- `GET /auth/accounts`
- `POST /auth/login`
- `GET /v1/models`
- `GET /dashboard`
- `GET/POST /dashboard/login`

当前版本对 `Cursor` 已补到：

- `local_state_direct` 授权入池
- `cli` 兜底推理模式
- `direct + rest` 推理模式
- `direct + connect` 第一版可配置适配模式

当前版本**还没有完全定版**：

- Cursor 真实 Connect service/method 路径
- Cursor 是否需要 grpc-web binary framing / 特殊 metadata
- Kiro 的长期稳定推理池能力

所以它现在的定位是：

- 先让 `new-api` 能读到 `Cursor / Kiro` 的本机登录态和最小池状态
- 给后续真正接推理代理留统一协议壳

## 1. 启动

### Cursor

```bash
bash scripts/start_cursor_local_pool.sh
```

默认：

- `PORT=3401`
- `API_KEY=demo-cursor-key`
- `DASHBOARD_PASSWORD=demo-cursor-dashboard`

支持额外环境变量：

- `CURSOR_PROVIDER_MODE=direct|cli`
- `CURSOR_DIRECT_PROTOCOL=rest|connect`
- `CURSOR_DIRECT_BASE_URL=...`
- `CURSOR_CONNECT_MODELS_PATH=...`
- `CURSOR_CONNECT_RESPONSES_PATH=...`
- `CURSOR_CONNECT_CHAT_COMPLETIONS_PATH=...`
- `CURSOR_CONNECT_PAYLOAD_MODE=passthrough|prompt_model|chat_model_messages`
- `CURSOR_CONNECT_MODEL_PATHS=...`
- `CURSOR_CONNECT_TEXT_PATHS=...`
- `CURSOR_CONNECT_EXTRA_HEADERS_JSON='{\"x-foo\":\"bar\"}'`
- `INFERENCE_MODE=responses|chat_completions|dual`

### Kiro

```bash
bash scripts/start_kiro_local_pool.sh
```

默认：

- `PORT=3501`
- `API_KEY=demo-kiro-key`
- `DASHBOARD_PASSWORD=demo-kiro-dashboard`

## 2. 默认本机登录态来源

### Cursor

- `~/Library/Application Support/Cursor/User/globalStorage/state.vscdb`

读取字段：

- `cursorAuth/cachedEmail`
- `cursorAuth/accessToken`
- `cursorAuth/refreshToken`
- `cursorAuth/cachedSignUpType`
- `cursorAuth/stripeMembershipType`

### Kiro

- `~/.aws/sso/cache/kiro-auth-token.json`

读取字段：

- `email`
- `accessToken`
- `refreshToken`
- `provider`
- `authMethod`
- `region`

## 3. 可选快照输入

若你不想直接读本机实时文件，也可以先导出快照，再通过 `SNAPSHOT_PATH` 启动：

```bash
bash scripts/export_local_client_auth_snapshots.sh --output-dir local-auth-snapshots
```

例如：

```bash
SNAPSHOT_PATH="$PWD/local-auth-snapshots/cursor-local-auth.json" bash scripts/start_cursor_local_pool.sh
```

```bash
SNAPSHOT_PATH="$PWD/local-auth-snapshots/kiro-local-auth.json" bash scripts/start_kiro_local_pool.sh
```

## 4. 最小验证

### Cursor

```bash
curl -sS -H 'Authorization: Bearer demo-cursor-key' http://127.0.0.1:3401/auth/status
```

```bash
curl -sS -H 'Authorization: Bearer demo-cursor-key' http://127.0.0.1:3401/auth/accounts
```

### Kiro

```bash
curl -sS -H 'Authorization: Bearer demo-kiro-key' http://127.0.0.1:3501/auth/status
```

```bash
curl -sS -H 'Authorization: Bearer demo-kiro-key' http://127.0.0.1:3501/auth/accounts
```

## 5. Dashboard

### Cursor

- [http://127.0.0.1:3401/dashboard/login](http://127.0.0.1:3401/dashboard/login)

密码默认：

- `demo-cursor-dashboard`

### Kiro

- [http://127.0.0.1:3501/dashboard/login](http://127.0.0.1:3501/dashboard/login)

密码默认：

- `demo-kiro-dashboard`

## 6. Cursor Connect 第一版联调

若你已经拿到真实 Cursor Connect method path，可以直接这样起：

```bash
CURSOR_PROVIDER_MODE=direct \
CURSOR_DIRECT_PROTOCOL=connect \
CURSOR_DIRECT_BASE_URL='https://agentn.global.api5.cursor.sh' \
CURSOR_CONNECT_MODELS_PATH='/your.service/YourListModelsMethod' \
CURSOR_CONNECT_RESPONSES_PATH='/your.service/YourResponsesMethod' \
CURSOR_CONNECT_PAYLOAD_MODE=passthrough \
bash /Users/jj/Documents/Playground/scripts/start_cursor_local_pool.sh
```

然后用 probe 脚本测：

```bash
bash /Users/jj/Documents/Playground/scripts/probe_cursor_connect.sh
```

若你要测聊天兼容入口：

```bash
MODE=chat_completions bash /Users/jj/Documents/Playground/scripts/probe_cursor_connect.sh
```

更完整的改造说明见：

- [docs/junhuo-cursor-connect-adapter-checklist.md](/Users/jj/Documents/Playground/docs/junhuo-cursor-connect-adapter-checklist.md)

## 7. 当前阶段结论

这套 MVP 先把：

- `本机登录态 -> 最小池状态接口 -> Cursor 第一版 direct adapter`

走到可联调状态。

下一阶段再补：

- Cursor 真实 Connect method path 定位
- Cursor 返回体字段定型
- Kiro 的长期稳定推理池
