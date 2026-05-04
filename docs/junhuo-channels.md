# `junhuo` 渠道说明

本文档记录 `junhuo` 当前常见渠道的职责分工、灰度思路和排障关注点。

## 1. 渠道设计原则

当前体系不是“一个模型只靠一个上游”，而是：

- 多渠道并行
- 能力分层
- 灰度与保底分离

因此维护时不要只看 `name`，还要同时看：

- `type`
- `status`
- `priority`
- `weight`
- `models`
- `group`
- `other_settings`
- `channel_info`

## 2. 当前常见渠道角色

### 2.1 `caowo`

用途：

- 通用主链路
- 常作为高优先级稳定渠道

适合承接：

- 常规 OpenAI 兼容请求
- 需要优先稳定性的主流量

### 2.2 `codex-e2e-temp`

用途：

- Codex 相关链路
- 在某些部署阶段也可作为远端 Codex 池代理或兜底

典型关注点：

- 是否只暴露需要的模型
- 是否误与其它同名模型渠道形成竞争

### 2.3 `antigravity-openclaw`

用途：

- 第一条 Antigravity 渠道
- 存量可用链路
- 保底使用

特点：

- 常作为 Antigravity 基线对照
- 适合对照 `openclaw2` 的行为变化

### 2.4 `antigravity-openclaw2`

用途：

- 第二条并行 Antigravity 渠道
- 用于多账号池和增强逻辑灰度

当前重点能力方向：

- 渠道级 OAuth 追加账号
- 多账号池化
- project 健康度隔离
- 更细粒度错误分类

## 3. Antigravity 渠道的特别说明

Antigravity 渠道与普通 OpenAI 兼容 key 渠道不同。

它们通常依赖：

- OAuth 凭据
- `project_id`
- `managed_project_id`
- 自动 refresh
- 多账号池状态

所以排障时不能只看：

- key 有没有填

还要看：

- 账号是否过期
- project 是否真实可用
- 当前模型是否可承接
- `channel_info` 是否处于 multi-key 状态

## 4. 渠道灰度原则

涉及 Antigravity 或 Codex 兼容修复时，默认遵循：

1. 旧渠道保底
2. 新渠道灰度
3. 验证稳定后再决定是否替换

这意味着：

- 不要一上来覆盖 `antigravity-openclaw`
- 更适合在 `antigravity-openclaw2` 上先做验证

## 5. 当前已知兼容性结论

截至目前，Antigravity 的表现可粗分为：

- `chat/completions`
  - 相对更容易稳定
  - QQbot / Telegram 常可正常使用

- `/v1/responses`
  - 尤其是 Win Codex 的真实 `gpt-5.5` 场景
  - 协议兼容风险更高

因此不能简单认为：

- `chat/completions` 可用
- 就等于 Codex 一定可用

## 6. 渠道运维建议

日常操作时建议保留以下习惯：

- 用 `status` 控制灰度启停
- 用 `priority` 控制优先级
- 用单开/双开方式做隔离测试
- 记录关键请求时间窗，便于日志定位

## 7. 不建议做的事情

以下行为容易把排障复杂度拉高：

- 同时改多个渠道的大量配置
- 在没有记录前提下反复改 `priority` 和 `status`
- 把新增灰度渠道直接投入主流量
- 混淆“渠道可聊天”和“Codex 可 Responses”的判断标准

## 8. 进一步参考

若要看更具体的字段语义和样例，请继续看：

- [渠道配置样例](./junhuo-channel-config-examples.md)
- [运维手册](./junhuo-operations.md)
- [Antigravity / Codex 兼容笔记](./junhuo-antigravity-codex-notes.md)
