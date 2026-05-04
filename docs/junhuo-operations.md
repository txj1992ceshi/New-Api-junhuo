# `junhuo` 运维手册

本文档偏向日常操作与排障，假设维护者已经知道 `new-api` 的基本概念，但需要快速处理 `junhuo` 线上环境。

## 1. 常用检查项

### 1.1 容器状态

```bash
docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}"
```

### 1.2 服务状态

```bash
curl http://127.0.0.1:3000/api/status
```

### 1.3 容器日志

```bash
docker logs --tail 100 new-api
```

### 1.4 线上业务日志

日志常见位置：

- `/opt/new-api/data/logs/oneapi-*.log`

查看最新日志文件：

```bash
ls -1t /opt/new-api/data/logs/oneapi-*.log | head
```

## 2. 渠道排障的基本顺序

遇到“某客户端不可用”时，先按下面顺序排：

1. 请求有没有真的到服务端
2. 命中了哪个渠道
3. 命中了哪个模型映射
4. 是客户端兼容问题、上游问题，还是空流问题

重点要记录：

- 测试时间窗
- 客户端类型
- 使用模型
- 当时启用的渠道组合

## 3. 常见测试方式

### 3.1 单开渠道

适合判断问题是否来自某一条特定渠道。

常见方式：

- 关闭 `1/2`
- 只开 `3/4`
- 再用 Win Codex 或 QQbot 复测

### 3.2 对照测试

适合同模型跨渠道对比，例如：

- `channel 2` 可用
- `channel 3/4` 不可用

这种情况下通常说明：

- 客户端原始请求 shape 不是唯一根因
- 更像是渠道兼容层差异

## 4. Antigravity 常见排查点

### 4.1 账号与 project

检查重点：

- OAuth 是否成功写入
- 是否为多账号池
- `project_id` 是否真实有效
- 是否误落到受限 project

### 4.2 refresh

容器启动后通常会看到自动 refresh 日志。若没有：

- 看 channel 是否启用
- 看凭据是否已损坏
- 看 refresh 逻辑是否触发

### 4.3 `/responses` 与 `chat/completions` 分开判断

不要把两者混为一谈。

一个常见现象是：

- `chat/completions` 正常
- `/v1/responses` 异常

这在 Antigravity 链路里是合理现象，不足以单独证明整个渠道已坏。

## 5. Codex 相关排障

Codex 场景重点看：

- 请求路径是否为 `POST /v1/responses`
- SSE 是否建立
- 是否有 `response.output_text.delta`
- 最终有没有 `assistant` 可见输出

如果只看到：

- `response.completed`
- 但无可见正文

那通常要继续看 Antigravity 兼容层和上游返回。

## 6. 什么时候该看代码，什么时候该看日志

优先看日志的场景：

- 怀疑请求没到
- 怀疑命中错渠道
- 怀疑上游返回空

优先看代码的场景：

- 同类请求稳定复现某种错误
- 修改后行为与预期不一致
- 已能确定问题在适配层而非外部网络

## 7. 文档化建议

每次处理一类复杂问题后，建议顺手记录：

- 时间
- 现象
- 结论
- 临时 workaround
- 是否已彻底修复

这类信息建议优先写入：

- 本文档
- `junhuo-antigravity-codex-notes.md`
- 专项切换/发布说明

## 8. 推荐连读

建议配合以下文档一起使用：

- [当前线上渠道快照说明](./junhuo-current-channel-snapshot.md)
- [发布与回滚 Checklist](./junhuo-release-checklist.md)
- [发布记录模板](./junhuo-release-log-template.md)
