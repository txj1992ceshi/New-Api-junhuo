# `junhuo` 外部池真实联调清单

本文用于 `Cursor / Windsurf / Kiro / Codex` 四类外部池代理渠道进入真实联调前的准备检查。

可重复执行的本地验证脚本：

```bash
bash scripts/validate_external_pool_channels.sh
```

默认超时：

- `STATUS_TIMEOUT=10`
- `MODELS_TIMEOUT=15`
- `RESPONSES_TIMEOUT=60`

若你要单独放宽真实推理验证窗口，可临时带：

```bash
RESPONSES_TIMEOUT=90 bash scripts/validate_external_pool_channels.sh
```

若要强制指定某条渠道的 smoke model，可临时带：

```bash
CURSOR_SMOKE_MODEL=cursor-default KIRO_SMOKE_MODEL=kiro-sonnet bash scripts/validate_external_pool_channels.sh
```

脚本也会顺手读取 `/auth/accounts`，把账号真实 `availableModels` 打出来，优先从账号真实可用模型里挑 smoke model，而不是只看 `/v1/models` 全量列表。

当前已整理稳定别名的渠道，优先使用稳定别名做 smoke：

- `Cursor`：`cursor-default`
- `Kiro`：`kiro-sonnet`
- `Codex`：`codex-default` / `codex-gpt54`
- `Windsurf`：暂不固定正式别名

## 1. 渠道配置前检查

每条渠道先确认：

- 已有可访问的外部池服务地址
- 已有可用的访问密钥
- 已知状态接口路径
- 已知账号列表接口路径
- 已知 Dashboard 地址（若有）
- 已确定先灰度的模型列表

## 2. 后台配置最小项

每条渠道至少填：

- `type=1`
- `name`
- `base_url`
- `key`
- `models`
- 对应 `*_pool_proxy=true`

若上游不是默认路径，再补：

- `*_pool_status_path`
- `*_pool_accounts_path`
- `*_pool_dashboard_path`
- `*_pool_authorize_url`
- `*_pool_auth_start_path`
- `*_pool_auth_complete_path`
- `*_pool_inference_mode`（`responses` / `chat_completions` / `dual`）
- `*_pool_probe_inference`（可选：开启后会做轻量推理探测，用于区分“已认证但不可推理”）
- `*_pool_auth_header`
- `*_pool_auth_scheme`

## 3. 后台状态验收

保存后先不要急着打流量，先看后台“帐号/池状态”：

- 列表页先看状态列：
  - `池可用` => 可以继续做真实请求验收
  - `空池` => 先回上游补池
  - `降级` => 先看错误账号数和模型支持情况
  - `断连` => 先 hover 看上游错误，再排 `base_url / key / 网络`
  - `已配授权 / 待配授权` => 判断这条渠道是否已经留好手动授权入口

### 3.1 未连通

- `pool_state = upstream_error`
- 可看到 `upstream_error`

### 3.2 已连通但空池

- `pool_state = empty_pool`
- `connection_ok = true`
- `total = 0`

### 3.3 可承接请求

- `pool_state = ready`
- `active > 0`
- 能看到账号或至少看到有效模型列表

## 4. 真实请求验收顺序

每条渠道按下面顺序单独验：

1. 后台 `拉取上游模型`
2. `GET /v1/models`（应仅返回可实际调用的模型；若无可用账号可能为空）
3. `POST /v1/responses` 或 `POST /v1/chat/completions`（取决于 `*_pool_inference_mode`）
4. 后台再次查看池状态
5. 查看消费日志里的 `admin_info.external_pool_*`

若这条渠道要顺手验授权入口，再补一轮：

6. 点击 `授权登录`
7. 确认 `打开授权页面` 能正常拉起
8. 提交回调 URL / code / 上游返回结果，确认 `/auth/complete` 有响应

Cursor 本地态直连专项验收：

9. 保持 Cursor 客户端已登录，但使 `cursor-agent status` 处于未登录
10. 再次验证 `/v1/responses`，确认仍可成功（`*_pool_mode=local_state_direct`）
11. 切换回 `cli` 模式后复测，确认可按预期回滚

Codex provider bridge 专项验收：

9. 保持 `~/.codex/config.toml` 与 `~/.codex/auth.json` 可读
10. 点击 `读取 Provider 配置` 或调用 `/auth/complete`，确认 `/auth/accounts` 出现 `source=local_codex_provider_config`
11. 优先验证 `/v1/responses`，若 `/v1/chat/completions` 命中上游限流或空内容兜底，则仍应看到标准化后的 assistant 文本
12. 若返回 `Rate limit exceeded`，先视为“上游已连通但当前额度受限”，不要误判为接入失败

## 5. 日志关注点

消费日志里重点看：

- `external_pool_proxy`
- `external_pool_display_name`
- `external_pool_channel_name`
- `external_pool_base_url`
- `external_pool_origin_model`
- `external_pool_upstream_model`

## 6. 灰度建议

第一轮联调建议：

- 每次只开一条外部池渠道
- 每次只验 1-2 个模型
- `priority` 先低于主链
- 有问题直接 `status=disabled`

## 7. 失败时先看什么

如果这轮没有通过，建议按这个顺序回看：

1. 列表状态列是 `空池 / 降级 / 断连` 里的哪一种
2. 弹窗里的 `上游错误`
3. 弹窗里的 `优先排查`
4. 原始 JSON 里 `connection_ok / total / active / models`
5. 请求日志里的 `admin_info.external_pool_*`
