# `junhuo` 当前线上渠道快照说明

本文档用于记录当前 `junhuo` 线上常见渠道的职责解释。它不是数据库导出，也不保证每个字段永远不变，而是帮助维护者快速回答：

- 线上 1/2/3/4 大概是谁
- 这些渠道平时各自承担什么角色
- 什么情况下应该单开、双开或只做灰度

## 1. 使用方式说明

阅读本文时要记住两点：

1. 这里描述的是“职责快照”，不是永远锁死的配置
2. 渠道的 `status / priority / weight / models` 可能会随灰度而调整

所以本文适合作为：

- 接手说明
- 排障前的脑图
- 发布前的角色确认

不适合作为：

- 精确配置源
- 数据库真值替代品

## 2. 当前常见四条渠道

### channel 1: `caowo`

通常角色：

- 通用主链路
- 高优先级稳定渠道

常见理解：

- 当它健康时，很多同名模型会优先命中它
- 更适合承担普通 OpenAI 兼容主流量

排障时常见用途：

- 作为“主链是否正常”的对照组

### channel 2: `codex-e2e-temp`

通常角色：

- Codex 相关链路
- 某些阶段可作为 Win Codex 的更稳承接方
- 也可能承担回源或代理语义

常见理解：

- 它不是简单的普通 key 渠道
- 某些场景下，对 Codex `/v1/responses` 的稳定性高于 Antigravity 渠道

排障时常见用途：

- 用来对比：
  - “客户端请求 shape 本身是否有问题”
  - “是不是只有 Antigravity 协议链有问题”

### channel 3: `antigravity-openclaw`

通常角色：

- 第一条 Antigravity 存量渠道
- 保底渠道
- 协议对照基线

常见理解：

- 这条渠道常被拿来和 `openclaw2` 做对照
- 即便它在聊天链路可用，也不能直接推导出 Codex 一定可用

排障时常见用途：

- 作为老链路基线

### channel 4: `antigravity-openclaw2`

通常角色：

- 第二条并行 Antigravity 增强渠道
- 用于灰度和实验增强

已知重点：

- 支持多账号 OAuth 追加
- 更偏多账号池、project、错误分类增强
- 默认不应一上来替换旧渠道承担全部主流量

排障时常见用途：

- 单独灰度
- 与 channel 3 做隔离对照

## 3. 常见启停组合的意义

### 组合 A：只开 1+2

常见目的：

- 暂时绕开 Antigravity
- 验证主链路和 Codex 稳定性

### 组合 B：只开 3+4

常见目的：

- 只测 Antigravity
- 隔离 Win Codex 与 Antigravity 的兼容问题

### 组合 C：单开 3

常见目的：

- 看老 Antigravity 基线表现

### 组合 D：单开 4

常见目的：

- 看 `openclaw2` 灰度增强后的纯表现

## 4. 如何用这份快照

当有人说：

- “2 能用，3/4 不行”

通常意味着：

- 客户端原始请求不是唯一根因
- Antigravity 协议兼容层更值得怀疑

当有人说：

- “QQbot 正常，但 Win Codex 不正常”

通常意味着：

- `chat/completions` 正常
- `/v1/responses` 异常

## 5. 与其它文档的关系

建议配合阅读：

- [渠道说明](./junhuo-channels.md)
- [渠道配置样例](./junhuo-channel-config-examples.md)
- [Antigravity / Codex 兼容笔记](./junhuo-antigravity-codex-notes.md)
