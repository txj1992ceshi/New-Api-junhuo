# `junhuo` 架构说明

本文档描述 `New-Api-junhuo` 当前这套定制运行体系的高层结构，重点是帮助后续维护者快速理解：

- 公网入口在哪里
- 线上 `new-api` 承担什么职责
- 各类客户端如何进入系统
- 现有渠道池大致如何分工

本文以当前 `junhuo` 实际运维形态为准，而不是上游通用开源默认形态。

## 1. 目标形态

当前体系的核心目标是：

1. 以 `junhuo.icu` 作为统一公网 OpenAI 兼容入口
2. 用一套 `new-api` 后端承接多种客户端
3. 同时维持多条可切换渠道，便于灰度、回退和定向排障

常见客户端包括：

- Win Codex
- QQbot / OpenClaw
- Telegram Bot
- 其他 OpenAI 兼容客户端

## 2. 高层拓扑

```text
客户端
  ├─ Win Codex
  ├─ QQbot / OpenClaw
  ├─ Telegram Bot
  └─ 其他 OpenAI 兼容客户端
          │
          ▼
     https://junhuo.icu
          │
          ▼
    Vultr 上的 new-api 单容器
          │
          ├─ OpenAI 兼容上游渠道
          ├─ 远端 Codex 池代理渠道
          └─ Antigravity OAuth 渠道
```

## 3. 线上后端形态

当前 `junhuo` 线上采用的是单容器部署：

- 单容器名称：`new-api`
- 运行方式：Docker
- 网络方式：`--network host`
- 数据目录挂载：宿主机 `/opt/new-api/data` 到容器 `/data`
- 环境文件：`/opt/new-api/deploy/new-api.env`

这意味着：

- 数据库、日志、运行时状态都落在宿主机数据目录
- 容器可频繁替换镜像，而不影响 `/data` 中的持久数据

## 4. 渠道层职责分工

这套系统不是单一上游，而是多个渠道并存。

当前常见角色分工如下：

- `caowo`
  - 常规优先渠道
  - 偏向稳定承接通用 OpenAI 兼容请求

- `codex-e2e-temp`
  - Codex 池相关链路
  - 在部分阶段也作为回源代理或兜底使用

- `antigravity-openclaw`
  - 第一条 Antigravity 渠道
  - 更偏保底和存量链路

- `antigravity-openclaw2`
  - 第二条并行 Antigravity 渠道
  - 用于多账号池、OAuth 追加、灰度增强和隔离验证

注意：

- 渠道名称和 `channel_id` 经常会在排障中被直接引用
- 渠道 `status / priority / weight / group / models` 可能随灰度阶段变化
- 文档里记录的是职责，不保证某一时刻的启停状态固定不变

## 5. 客户端行为差异

不同客户端对后端的命中方式不同。

### 5.1 QQbot / Telegram / 普通聊天客户端

这类客户端大多更接近：

- `/v1/chat/completions`

它们在 Antigravity 渠道上通常比 Codex 更容易稳定。

### 5.2 Win Codex

Win Codex 的真实请求重点在：

- `POST /v1/responses`

这意味着：

- 它对 Responses 兼容层更敏感
- Antigravity 渠道即便 `chat/completions` 正常，也不代表对 Codex 一定正常

## 6. 为什么保留多条渠道

多渠道并存不是冗余，而是运维策略的一部分：

- 用于灰度
- 用于不同上游能力隔离
- 用于单渠道异常时快速切换
- 用于对比不同协议兼容效果

尤其是 Antigravity 相关链路，通常采取：

- 旧渠道保底
- 新渠道灰度
- 验证通过后再决定替换

## 7. 文档导航

与当前体系直接相关的仓库内文档：

- [部署说明](./junhuo-deployment.md)
- [渠道说明](./junhuo-channels.md)
- [运维手册](./junhuo-operations.md)
- [Antigravity / Codex 兼容笔记](./junhuo-antigravity-codex-notes.md)
- [三渠道池统一切换说明](./junhuo-trichannel-cutover.md)
