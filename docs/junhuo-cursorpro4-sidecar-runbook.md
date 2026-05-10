# `junhuo` CursorPro4 sidecar 落库与联调手册

这份手册对应第一阶段的 4 条 CursorPro4 渠道：

- `cursorpro4-cursor-pool-proxy`
- `cursorpro4-windsurf-pool-proxy`
- `cursorpro4-kiro-pool-proxy`
- `cursorpro4-codex-pool-proxy`

目标是把现有本地池服务按 CursorPro4 sidecar 协议跑起来，再直接落库到 `new-api`。

## 1. 启动 4 个 sidecar

### Cursor

```bash
CURSORPRO4_LICENSE_STATUS=activated \
bash scripts/start_cursor_local_pool.sh
```

### Windsurf

```bash
CURSORPRO4_LICENSE_STATUS=activated \
bash scripts/start_windsurf_local_pool.sh
```

### Kiro

```bash
CURSORPRO4_LICENSE_STATUS=activated \
bash scripts/start_kiro_local_pool.sh
```

### Codex

```bash
CURSORPRO4_LICENSE_STATUS=activated \
CURSORPRO4_BRIDGE_BASE_URL=http://127.0.0.1:8327 \
bash scripts/start_codex_local_pool.sh
```

可选状态：

- `CURSORPRO4_LICENSE_STATUS=activated`
- `CURSORPRO4_LICENSE_STATUS=expired`
- `CURSORPRO4_LICENSE_STATUS=invalid`
- `CURSORPRO4_LICENSE_STATUS=unavailable`

## 2. 落库 4 条 CursorPro4 渠道

先准备环境变量：

```bash
export CURSOR_POOL_BASE_URL=http://127.0.0.1:3401
export CURSOR_POOL_API_KEY=demo-cursor-key

export WINDSURF_POOL_BASE_URL=http://127.0.0.1:3003
export WINDSURF_POOL_API_KEY=demo-windsurf-key

export KIRO_POOL_BASE_URL=http://127.0.0.1:3501
export KIRO_POOL_API_KEY=demo-kiro-key

export CODEX_POOL_BASE_URL=http://127.0.0.1:3601
export CODEX_POOL_API_KEY=demo-codex-key
```

然后执行：

```bash
bash scripts/apply_cursorpro4_channels.sh
```

这会默认生成：

- `cursorpro4-cursor-pool-proxy`
- `cursorpro4-windsurf-pool-proxy`
- `cursorpro4-kiro-pool-proxy`
- `cursorpro4-codex-pool-proxy`

并自动写入：

- `cursorpro4_sidecar=true`
- `cursorpro4_provider=<provider>`
- `cursorpro4_probe_inference=true`

## 3. 检查渠道配置

```bash
CURSORPRO4_CHANNEL_PREFIX=cursorpro4 \
bash scripts/external_pool_channels.sh check
```

重点确认：

- `base_url / api_key` 已设置
- 3401 / 3003 / 3501 / 3601 端口在监听
- `CODEX_POOL_AUTH_STRATEGY=provider_bridge`

## 4. 做 sidecar 验证

```bash
bash scripts/validate_external_pool_channels.sh
```

输出里重点看：

- `health_http`
- `health_license`
- `status_http`
- `license_status`
- `bridge_status`
- `bridge_error`
- `models_http`
- `accounts_http`

预期规则：

- `license_status=activated` 才能进入可推理判断
- `bridge_status=ready` 才能通过 bridge 相关探测
- `active > 0` 才会被判定为真正可用

## 5. 后台联调验收

后台至少确认这些点：

- 4 条渠道显示对应池状态面板
- `Sidecar 未激活 / Sidecar 已过期 / Bridge 不可达 / 待授权 / 无可用账号` 能正确显示
- Codex 走 `provider_bridge`
- `pool_status`、`accounts`、`auth_view`、`auth/start`、`auth/complete` 都能返回
- `/v1/responses` 为主，`/v1/chat/completions` 兼容

## 6. 常见排查

### health 正常但 status 不可用

优先检查：

- `Authorization` header
- API key 是否与 sidecar 启动值一致
- `*_pool_status_path` 是否仍是 `/auth/status`

### status 有账号，但 bridge 不可推理

优先检查：

- `CURSORPRO4_LICENSE_STATUS`
- `CURSORPRO4_BRIDGE_BASE_URL`
- 本地 bridge 进程是否真在监听
- `INFERENCE_MODE` 与上游开放路径是否一致

### 渠道已落库但不是 `cursorpro4-*`

执行前确认：

```bash
export CURSORPRO4_CHANNEL_PREFIX=cursorpro4
```
