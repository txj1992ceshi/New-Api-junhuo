# `junhuo` 三渠道最终落库模板

这份文档给的是第一阶段可直接填进后台的模板，目标很简单：

- `Cursor / Windsurf / Kiro` 各自单独建渠道
- 都先按 `OpenAI-compatible` 外部池代理形态接入
- 先跑通、可观测、可灰度，再谈跨渠道统一调度

## 1. 通用落库约定

建议第一阶段统一按下面思路创建：

- `type = 1`
- `status = 1`
- `base_url = 外部池代理服务地址`
- `key = 外部池代理服务自己的 API key`
- `models = 先只放准备灰度的模型`
- `priority / weight = 先给中低优先级`
- `group = default` 或你的内部灰度组
- `other_info = 只放该池的代理开关和状态接口信息`

推荐第一阶段默认值：

- `*_pool_status_path = /auth/status`
- `*_pool_accounts_path = /auth/accounts`
- `*_pool_dashboard_path = /dashboard`
- `*_pool_auth_header = Authorization`
- `*_pool_auth_scheme = Bearer`

## 2. Cursor 渠道模板

后台建议填写为：

```json
{
  "name": "cursor-pool-proxy",
  "type": 1,
  "status": 1,
  "group": "default",
  "models": "gpt-5.4,gpt-5.5",
  "base_url": "http://127.0.0.1:3401",
  "key": "sk-cursor-proxy",
  "priority": 80,
  "weight": 100,
  "other_info": {
    "cursor_pool_proxy": true,
    "cursor_pool_status_path": "/auth/status",
    "cursor_pool_accounts_path": "/auth/accounts",
    "cursor_pool_dashboard_path": "/dashboard",
    "cursor_pool_auth_header": "Authorization",
    "cursor_pool_auth_scheme": "Bearer"
  }
}
```

适用场景：

- Cursor 外部池服务已经能自己维护登录态、session 或补池
- `new-api` 只负责接流量、查状态、做灰度

## 3. Windsurf 渠道模板

```json
{
  "name": "windsurf-pool-proxy",
  "type": 1,
  "status": 1,
  "group": "default",
  "models": "gpt-5.4,gpt-5.5,claude-sonnet",
  "base_url": "http://127.0.0.1:3003",
  "key": "sk-windsurf-proxy",
  "priority": 70,
  "weight": 100,
  "other_info": {
    "windsurf_pool_proxy": true,
    "windsurf_pool_status_path": "/auth/status",
    "windsurf_pool_accounts_path": "/auth/accounts",
    "windsurf_pool_dashboard_path": "/dashboard",
    "windsurf_pool_auth_header": "Authorization",
    "windsurf_pool_auth_scheme": "Bearer"
  }
}
```

适用场景：

- 你已经有独立 WindsurfPoolAPI 或同类代理服务
- 想让后台直接看到池状态、账号数和错误态

## 4. Kiro 渠道模板

```json
{
  "name": "kiro-pool-proxy",
  "type": 1,
  "status": 1,
  "group": "default",
  "models": "gpt-5.4,gpt-5.5,claude-sonnet",
  "base_url": "http://127.0.0.1:3501",
  "key": "sk-kiro-proxy",
  "priority": 60,
  "weight": 100,
  "other_info": {
    "kiro_pool_proxy": true,
    "kiro_pool_status_path": "/auth/status",
    "kiro_pool_accounts_path": "/auth/accounts",
    "kiro_pool_dashboard_path": "/dashboard",
    "kiro_pool_auth_header": "Authorization",
    "kiro_pool_auth_scheme": "Bearer"
  }
}
```

适用场景：

- 你用的是 `Kiro-Go`、`kiro-gateway` 或自建同类代理
- 想先把 Kiro 池作为一条独立外部代理链路接入

## 5. 什么时候改这些值

下面几项只在上游接口不一致时再改：

- `*_pool_status_path`
- `*_pool_accounts_path`
- `*_pool_dashboard_path`
- `*_pool_auth_header`
- `*_pool_auth_scheme`

例如上游不是 Bearer，而是：

```json
{
  "cursor_pool_auth_header": "X-API-Key",
  "cursor_pool_auth_scheme": ""
}
```

## 6. 推荐灰度顺序

第一阶段建议按这个顺序进：

1. 新建三条渠道，但先只开一条做验证
2. 先在后台看 `帐号/池状态`
3. 再执行一次拉上游模型
4. 再做 `/v1/responses` 测试
5. 稳定后再把模型暴露给小范围 token 或 group

## 7. 建议的初始优先级

如果你暂时不想影响现有主链，可以先用：

- `Cursor = priority 80`
- `Windsurf = priority 70`
- `Kiro = priority 60`

这样它们会低于现有主链，只在你明确给模型或 group 放流量时承接。

## 8. 验收时看什么

后台至少确认这几件事：

- 渠道详情里出现对应 `PoolSummary`
- `帐号/池状态` 能看到 `ready / empty_pool / upstream_error`
- 拉上游模型能带回状态接口里的模型列表
- 渠道测试默认走 `/v1/responses`
- 关闭任一渠道不会影响其它存量渠道
