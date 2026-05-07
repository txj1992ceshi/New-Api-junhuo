# `junhuo` 外部池本地真实联调快照（2026-05-06）

本次快照基于本机环境直接验证：

- `new-api`: `http://127.0.0.1:3001`
- `WindsurfAPI`: `http://127.0.0.1:3003`
- `Cursor pool`: `http://127.0.0.1:3401`
- `Kiro pool`: `http://127.0.0.1:3501`

验证脚本：

```bash
bash scripts/validate_external_pool_channels.sh
```

## 1. 当前结论

### 1.1 Windsurf

- 已有真实在线池服务
- 当前库中渠道配置可直连
- `/auth/status` 成功
- `/v1/models` 成功
- `/v1/responses` 已打到上游，但返回 `503 No active accounts`

这说明：

- `new-api -> Windsurf 外部池` 链路已经基本打通
- 当前卡点不是接入层，而是 `Windsurf` 池内还没有活跃账号

### 1.2 Cursor

- 当前 `3401` 未监听
- `/auth/status` 不通
- `/v1/models` 不通

这说明：

- `Cursor` 渠道目前还是“配置已落库，但池服务未启动”

### 1.3 Kiro

- 当前 `3501` 未监听
- `/auth/status` 不通
- `/v1/models` 不通

这说明：

- `Kiro` 渠道目前还是“配置已落库，但池服务未启动”

## 2. 实测摘要

### 2.1 `cursor-pool-proxy`

- `status_http=000`
- `models_http=000`
- 原因：本机 `127.0.0.1:3401` 连接失败

### 2.2 `windsurf-pool-proxy`

- `status_http=200`
- `status_body={"authenticated":false,"total":0,"active":0,"error":0}`
- `models_http=200`
- `models_count=101`
- `smoke_model=gpt-5.4-high`
- `responses_http=503`
- `responses_body={"error":{"message":"No active accounts. POST /auth/login to add accounts.","type":"auth_error"}}`

### 2.3 `kiro-pool-proxy`

- `status_http=000`
- `models_http=000`
- 原因：本机 `127.0.0.1:3501` 连接失败

## 3. 下一步最该做什么

按优先级建议：

1. 先把 `Windsurf` 池补到至少 1 个 active account
2. 补完后再次运行：

```bash
WINDSURF_SMOKE_MODEL=gpt-5.4-high bash scripts/validate_external_pool_channels.sh
```

若你已经拿到 Windsurf token / api_key / 邮箱密码，可直接导入当前本机池：

```bash
bash scripts/import_windsurf_pool_account.sh --token '你的 token'
```

或：

```bash
bash scripts/import_windsurf_pool_account.sh --api-key '你的 api_key'
```

或：

```bash
bash scripts/import_windsurf_pool_account.sh --email '你的邮箱' --password '你的密码'
```

3. 确认 `windsurf-pool-proxy` 的 `/v1/responses` 从 `503` 变成 `200`
4. 再启动 `Cursor` 的本地池服务（`3401`）
5. 再启动 `Kiro` 的本地池服务（`3501`）

## 4. 当前阶段判定

若按“真实三条渠道配置落库 + 联调检查清单”的完成度来分：

- `Windsurf`: 已进入真实联调阶段
- `Cursor`: 已落库，未进入真实联调
- `Kiro`: 已落库，未进入真实联调
