# `junhuo` 当前线上状态记录

本文档记录一份“此刻线上是什么样”的状态快照，用于减少接手时对聊天记录的依赖。

> 记录时间：`2026-05-04 15:43 CST`

## 1. 容器运行状态

当前线上容器：

- 容器名：`new-api`
- 镜像：`new-api:codex-responses-capability-81208e85`
- 状态：`Up`

当前部署仍是：

- 单容器
- `--network host`
- `--env-file /opt/new-api/deploy/new-api.env`
- `/opt/new-api/data:/data`

## 2. 当前渠道状态

根据线上数据库当前读取结果：

| id | name | type | status | priority | weight | group |
|---|---|---:|---:|---:|---:|---|
| 1 | `caowo` | 1 | 2 | 300 | 100 | `default` |
| 2 | `codex-e2e-temp` | 1 | 2 | 200 | 100 | `default` |
| 3 | `antigravity-openclaw` | 58 | 1 | 100 | 100 | `default` |
| 4 | `antigravity-openclaw2` | 58 | 1 | 90 | 100 | `default` |

按当前状态理解：

- `status=1`：启用
- `status=2`：停用

所以当前线上处于：

- `1/2` 关闭
- `3/4` 开启

这是一种明显偏向 Antigravity 隔离排障/灰度的组合，而不是常规主链流量组合。

## 3. 当前状态的运维含义

当前这份状态更适合用于：

- 单独观察 Antigravity 渠道行为
- 隔离 Win Codex 与 Antigravity 的 `/v1/responses` 兼容问题
- 对比 `antigravity-openclaw` 与 `antigravity-openclaw2`

它不等价于：

- 通用生产最佳组合
- 长期稳定主链配置

## 4. 当前已知背景

与此快照直接相关的近期背景：

1. `openclaw2` 已具备多账号 OAuth 追加能力
2. Win Codex 与 Antigravity 的 `/v1/responses` 协议兼容问题仍未视为彻底解决
3. 当前线上已将真实 `Codex CLI /v1/responses(/compact)` 改为先走“按渠道类型生效”的能力矩阵
4. 因此 `Antigravity(type=58)` 默认不再参与真实 Codex `/responses` 候选链
5. Antigravity 仍保留给 QQbot / Telegram / 常规 `chat/completions`

## 5. 参考文档

建议配合阅读：

- [当前线上渠道快照说明](./junhuo-current-channel-snapshot.md)
- [发布记录](./junhuo-release-log.md)
- [Antigravity / Codex 兼容笔记](./junhuo-antigravity-codex-notes.md)
