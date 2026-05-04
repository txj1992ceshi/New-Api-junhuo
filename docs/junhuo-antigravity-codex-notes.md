# Antigravity / Codex 兼容笔记

本文档记录 `junhuo` 当前在 Antigravity 承接 Win Codex 时已经确认的现象、结论和边界，避免后续重复从头试错。

## 1. 当前结论摘要

截至目前，可以确认：

- Antigravity 渠道对 `chat/completions` 通常可用
- QQbot / Telegram 可通过 Antigravity 正常得到回复
- Win Codex 的真实流量重点在 `POST /v1/responses`
- Antigravity 承接该路径时，兼容风险明显高于 `chat/completions`

这意味着：

- `聊天可用` 不等于 `Codex 可用`

## 2. 已经确认过的问题层级

以下层级已经排查过，不应再重复从零猜：

### 2.1 OAuth / 多账号池并非唯一根因

这些链路虽然曾有问题，但不是当前 Codex 兼容异常的唯一解释：

- OAuth 授权
- 多账号追加
- refresh
- 多账号池写入

### 2.2 project 识别确实影响账号是否可用

已经确认过：

- 新 OAuth 账号若探测 project 失败，可能落入受限 project
- 这会导致账号虽入池，但实际上无法承接请求

### 2.3 `User-Agent` 透传覆盖曾是问题点

已经修过：

- Antigravity 应保留自身 `User-Agent`
- 不应被客户端透传 `User-Agent` 覆盖

但它不是后续所有 `/responses` 失败的唯一根因。

## 3. 当前对 Codex `/responses` 的已知结论

### 3.1 请求确实发到了服务端

在“无回复”样本中，已确认：

- Win Codex 的 `/v1/responses` 请求真实发出
- 服务端真实收到了请求
- 服务端有时也会回 `200 OK` + SSE

### 3.2 失败并不总是网络层错误

已经确认过一类典型现象：

- 服务端返回了 SSE
- 但没有 `assistant` 可见正文
- 客户端表现为“无回复”或无渲染

### 3.3 `gpt-5.5` 在 Antigravity 上显著比 `gpt-5.4` 更不稳定

已观察到：

- `gpt-5.4` 存在成功正文样本
- `gpt-5.5` 更容易出现空流或协议兼容异常

## 4. 当前已尝试过的修复方向

以下方向已经试过，不宜再当作全新思路重复盲猜：

### 4.1 Responses 无状态 transcript 化

已经把一部分 Codex Responses 请求转成更偏无状态 transcript 的形式，但未彻底解决 `gpt-5.5` 问题。

### 4.2 空流显式失败

曾引入“空流显式失败”，将：

- 无正文 SSE

改为：

- 明确错误提示

后续为了配合当前运维决策，又回退成旧的“无回复但不显式抛错”的表现。

### 4.3 `gpt-5.5` 顶层 no-tools 兼容

已经试过对 Antigravity 的 `gpt-5.5` 路径强制去掉顶层：

- `tools`
- `tool_choice`

但并未彻底消除上游的异常返回。

## 5. 当前最重要的判断

当前最值得保留的结论是：

- 问题不只是在渠道路由
- 也不只是在 OAuth / project
- 真正难点在于：
  - Codex `/v1/responses`
  - 到 Antigravity / Gemini 请求 envelope
  - 以及其中的 transcript / tool / state 语义

换句话说：

- 这是协议兼容层问题
- 不是简单配置问题

## 6. 当前可用边界

从运维角度看，当前更现实的边界是：

- Antigravity 可继续服务 QQbot / Telegram / 常规聊天链路
- 若 Codex 主链要求稳定，应优先使用已验证更稳的非 Antigravity 渠道

## 7. 后续继续修复时应优先寻找的证据

如果未来继续推进这条线，优先寻找：

1. Antigravity / Gemini 成功承接类似请求的真实上游 request 样本
2. 成功样本与失败样本的最终上游 body/headers 对照
3. 是否仍有残余 transcript 结构触发上游误判为 function calling
4. 某些模型是否应直接降级为“仅文本、弱状态、无工具”

## 8. 文档使用建议

未来再排这条线时，先看本文档，再决定：

- 是继续做协议兼容修复
- 还是在运维上将 Codex 与 Antigravity 职责彻底拆开
