# `junhuo` 渠道配置样例

本文档不追求给出数据库里的完整真实快照，而是总结当前 `junhuo` 体系里几类关键渠道的配置语义，帮助维护者理解：

- 这些渠道大概应该长什么样
- 哪些字段最关键
- 哪些字段改错最容易出事故

## 1. 先看哪些字段

无论是什么渠道，排查时建议优先看：

- `id`
- `name`
- `type`
- `status`
- `priority`
- `weight`
- `group`
- `models`
- `base_url`
- `key`
- `settings`
- `other`
- `channel_info`

其中最容易被忽略但很关键的是：

- `settings`
  - 经常承载 `responses_model_mapping`
  - 也可能承载 `responses_compact_model_mapping`

- `other`
  - 经常承载一些额外开关
  - 例如远端 Codex 池代理、偏好 IPv4 等语义

- `channel_info`
  - 反映运行态
  - 尤其是 multi-key、多账号池、key 状态等

## 2. 常规 OpenAI 兼容主链渠道样例

这类渠道可近似理解为：

- 以 API key 驱动
- 承接常规 OpenAI 兼容请求
- 主要关注稳定性、优先级和模型暴露范围

示意：

```json
{
  "name": "caowo",
  "type": "OpenAI-compatible",
  "status": "enabled",
  "priority": 300,
  "weight": 100,
  "group": "default",
  "models": "gpt-5.4,gpt-5.5,...",
  "base_url": "https://<upstream>/v1",
  "key": "sk-***",
  "settings": {},
  "other": {}
}
```

关注点：

- `priority` 往往直接决定是否优先于其它同名模型渠道
- `models` 不要暴露不准备让它承接的模型

## 3. 远端 Codex 池代理渠道样例

这类渠道的目标不是自己直连真实模型，而是把请求继续转发到另一端的 Codex 池。

仓库里已有相关逻辑：

- [service/remote_codex_pool_proxy.go](/Users/jj/Documents/Playground/service/remote_codex_pool_proxy.go)

它能识别的关键语义包括：

- `remote_codex_pool_proxy`
- `remote_codex_pool_channel_id`
- `remote_codex_admin_base_url`
- `remote_codex_admin_access_token`
- `remote_codex_admin_user_id`

示意：

```json
{
  "name": "codex-e2e-temp",
  "type": "OpenAI-compatible",
  "status": "enabled",
  "priority": 200,
  "weight": 100,
  "group": "default",
  "base_url": "http://127.0.0.1:18080",
  "key": "sk-<internal_token>-2",
  "other": {
    "remote_codex_pool_proxy": true,
    "remote_codex_pool_channel_id": 2,
    "remote_codex_admin_base_url": "http://127.0.0.1:18080",
    "remote_codex_admin_user_id": 1
  }
}
```

关注点：

- `key` 里可能带有特定 channel id 语义
- `other` 才是这类渠道真正能工作的关键
- 不要把它误当成普通上游 key 渠道

## 4. Antigravity 单账号渠道样例

Antigravity 渠道最初可以是单账号形态。

示意：

```json
{
  "name": "antigravity-openclaw",
  "type": 58,
  "status": "enabled",
  "priority": 100,
  "weight": 100,
  "group": "default",
  "models": "gpt-5.4,gpt-5.5,antigravity-gemini-3-flash,...",
  "base_url": "",
  "key": "{\"access_token\":\"...\",\"refresh_token\":\"...\",\"project_id\":\"...\",\"managed_project_id\":\"...\",\"email\":\"...\"}",
  "other": {
    "prefer_ipv4": true
  },
  "channel_info": {
    "is_multi_key": false
  }
}
```

关注点：

- `type=58` 是 Antigravity
- `key` 不是普通 token，而是 OAuth JSON
- `project_id` / `managed_project_id` 是否真实可用非常关键

## 4.1 Cursor 外部池代理渠道样例

第一阶段建议把 Cursor 作为独立外部池代理渠道接入，而不是直接并入 `ChannelTypeCodex` 内部池。

示意：

```json
{
  "name": "cursor-pool-proxy",
  "type": "OpenAI-compatible",
  "status": "enabled",
  "priority": 80,
  "weight": 100,
  "group": "default",
  "models": "gpt-5.4,gpt-5.5",
  "base_url": "http://127.0.0.1:3401",
  "key": "sk-cursor-proxy",
  "other": {
    "cursor_pool_proxy": true,
    "cursor_pool_status_path": "/auth/status",
    "cursor_pool_accounts_path": "/auth/accounts",
    "cursor_pool_dashboard_path": "/dashboard",
    "cursor_pool_authorize_url": "http://127.0.0.1:3401/dashboard/login",
    "cursor_pool_authorize_hint": "完成 Cursor 手动授权后，回到池状态页确认账号数和可用数",
    "cursor_pool_auth_start_path": "/auth/start",
    "cursor_pool_auth_complete_path": "/auth/complete"
  }
}
```

关注点：

- 默认仍走普通 OpenAI-compatible relay
- `other` 里的 `cursor_pool_proxy=true` 才会触发后台池状态面板
- 若外部 Cursor 服务接口路径不同，可直接改 `*_path`
- 若后面要走手动授权登录，可直接配置 `cursor_pool_authorize_url`

## 4.2 Windsurf 外部池代理渠道样例

```json
{
  "name": "windsurf-pool-proxy",
  "type": "OpenAI-compatible",
  "status": "enabled",
  "priority": 70,
  "weight": 100,
  "group": "default",
  "models": "gpt-5.4,gpt-5.5,claude-sonnet",
  "base_url": "http://127.0.0.1:3003",
  "key": "windsurf-api-key",
  "other": {
    "windsurf_pool_proxy": true,
    "windsurf_pool_status_path": "/auth/status",
    "windsurf_pool_accounts_path": "/auth/accounts",
    "windsurf_pool_dashboard_path": "/dashboard",
    "windsurf_pool_authorize_url": "http://127.0.0.1:3003/dashboard/login",
    "windsurf_pool_authorize_hint": "完成 Windsurf 手动授权后，回到池状态页确认账号数和可用数",
    "windsurf_pool_auth_start_path": "/auth/start",
    "windsurf_pool_auth_complete_path": "/auth/complete"
  }
}
```

关注点：

- Windsurf 推荐让外部池服务自己维护补池和会话
- `new-api` 只消费池摘要和账号列表
- `windsurf_pool_authorize_url` 适合预留给后续手动授权入口

## 4.3 Kiro 外部池代理渠道样例

```json
{
  "name": "kiro-pool-proxy",
  "type": "OpenAI-compatible",
  "status": "enabled",
  "priority": 60,
  "weight": 100,
  "group": "default",
  "models": "gpt-5.4,gpt-5.5,claude-sonnet",
  "base_url": "http://127.0.0.1:3501",
  "key": "sk-kiro-proxy",
  "other": {
    "kiro_pool_proxy": true,
    "kiro_pool_status_path": "/auth/status",
    "kiro_pool_accounts_path": "/auth/accounts",
    "kiro_pool_dashboard_path": "/dashboard",
    "kiro_pool_authorize_url": "http://127.0.0.1:3501/dashboard/login",
    "kiro_pool_authorize_hint": "完成 Kiro 手动授权后，回到池状态页确认账号数和可用数",
    "kiro_pool_auth_start_path": "/auth/start",
    "kiro_pool_auth_complete_path": "/auth/complete"
  }
}
```

关注点：

- Kiro 上游若不是 `/auth/status` / `/auth/accounts`，直接在 `other` 中覆盖路径
- 第一阶段不要求把上游账号明细回灌进 `channel_info.multi_key_meta`
- `kiro_pool_authorize_url` 适合预留给后续手动授权入口

## 5. Antigravity 多账号池渠道样例

`antigravity-openclaw2` 这类渠道的重点，不是某一个 key，而是多账号池。

运行态上更应关注：

- `channel_info.is_multi_key`
- `channel_info.multi_key_size`
- `channel_info.multi_key_mode`
- `channel_info.multi_key_meta`
- `channel_info.multi_key_disabled_reason`

示意：

```json
{
  "name": "antigravity-openclaw2",
  "type": 58,
  "status": "enabled",
  "priority": 90,
  "weight": 100,
  "group": "default",
  "models": "gpt-5.4,gpt-5.5,antigravity-gemini-3-flash,...",
  "other": {
    "prefer_ipv4": true
  },
  "channel_info": {
    "is_multi_key": true,
    "multi_key_size": 2,
    "multi_key_mode": "random"
  }
}
```

注意：

- 前端“查看密钥”里看到的可能是聚合后的多 key 内容
- 单看 `key_count` 还不够，要结合 `channel_info` 看真实运行态

## 6. Responses 模型映射字段

对于 Codex 相关兼容，常需要关注：

- `responses_model_mapping`
- `responses_compact_model_mapping`

这些字段通常位于 `settings` 内。

示意：

```json
{
  "responses_model_mapping": {
    "gpt-5.4": "gemini-3-flash",
    "gpt-5.5": "gemini-3-flash"
  },
  "responses_compact_model_mapping": {
    "gpt-5.4-mini": "gemini-3-flash"
  }
}
```

关注点：

- 模型名写错会直接导致路由异常
- “映射存在”不等于“协议一定兼容”

## 7. `other` 字段常见语义

在 `junhuo` 当前体系里，`other` 常见承担的语义包括：

- `remote_codex_pool_proxy`
- `remote_codex_pool_channel_id`
- `remote_codex_admin_base_url`
- `remote_codex_admin_user_id`
- `prefer_ipv4`

此外，渠道“额外设置”与 `other` 并不完全等价，另可参考：

- [docs/channel/other_setting.md](/Users/jj/Documents/Playground/docs/channel/other_setting.md)

## 8. 改配置时最容易出事故的点

1. 把 `models` 暴露得过宽
2. 无记录地改 `priority`
3. 把 Antigravity 的单账号 key 当普通文本替换
4. 改了 `responses_model_mapping` 却没同步验证真实模型链路
5. 误把运行态 `channel_info` 当静态配置理解

## 9. 实际操作建议

每次要动配置时，建议先记录：

- 变更前 `status / priority / weight`
- 变更前渠道截图或导出内容
- 本次要验证的模型和客户端
- 验证时间窗

这样出问题后，才能快速还原是“配置变更”还是“上游波动”。
