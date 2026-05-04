# `junhuo` 发布与回滚 Checklist

本文档提供一份面向 `junhuo` 当前实际环境的操作清单，适合在发布前、发布后和回滚时逐项勾选。

## 1. 发布前 Checklist

### 代码与分支

- [ ] 本轮改动范围已经确认
- [ ] 未误包含无关代码变更
- [ ] 本地关键测试已执行
- [ ] 当前提交已落在明确分支上
- [ ] 需要的话已推送到 GitHub

### 发布目标

- [ ] 已明确本次镜像 tag
- [ ] 已明确是否只换镜像，不改 env
- [ ] 已明确是否需要保持当前渠道启停状态
- [ ] 已明确本次验收模型和客户端

### 风险控制

- [ ] 已知道上一版稳定镜像 tag
- [ ] 已确认 `/opt/new-api/data` 不会被动到
- [ ] 已确认 `/opt/new-api/deploy/new-api.env` 不会被误改

## 2. 发布执行 Checklist

### 上传与构建

- [ ] 当前 HEAD 已通过 `git archive` 上传到服务器构建目录
- [ ] 新构建目录已使用独立 tag 命名
- [ ] 服务器上 `docker build -t new-api:<tag> .` 成功

### 容器替换

- [ ] 已停止旧容器 `new-api`
- [ ] 已按原参数重新运行新容器
- [ ] 参数保持为：
  - [ ] `--network host`
  - [ ] `--env-file /opt/new-api/deploy/new-api.env`
  - [ ] `-v /opt/new-api/data:/data`
  - [ ] `--restart unless-stopped`

## 3. 发布后基础验收 Checklist

### 运行态

- [ ] `docker ps` 显示新镜像已运行
- [ ] `curl http://127.0.0.1:3000/api/status` 成功
- [ ] `docker logs --tail 80 new-api` 无明显启动错误

### 自动任务

- [ ] 关键自动任务正常启动
- [ ] 若涉及 Antigravity，能看到 refresh 任务正常跑起

### 业务日志

- [ ] 最新日志文件正常生成
- [ ] 无明显 panic / fatal / migration error

## 4. 发布后渠道验收 Checklist

### 常规验收

- [ ] 当前计划启用的渠道状态正确
- [ ] 当前计划关闭的渠道未误开启
- [ ] `priority / weight` 没被意外改动

### 按客户端验收

- [ ] 常规聊天客户端已做 smoke test
- [ ] 若涉及 QQbot / Telegram，已完成最小发言验证
- [ ] 若涉及 Win Codex，已明确测试：
  - [ ] 使用模型
  - [ ] 测试时间窗
  - [ ] 是否命中 `/v1/responses`

## 5. Antigravity 专项验收 Checklist

若本次涉及 Antigravity：

- [ ] 账号 refresh 正常
- [ ] 多账号池状态符合预期
- [ ] `channel_info.is_multi_key` 符合预期
- [ ] 未误覆盖已有账号
- [ ] 关键模型已做最小验证

若涉及 `openclaw2` 灰度：

- [ ] 已确认它是否应参与主流量
- [ ] 若只是灰度，已确认其 `status / priority` 没误伤线上

## 6. Win Codex 专项验收 Checklist

若本次目标包含 Win Codex：

- [ ] 用新会话测试
- [ ] 记录准确时间窗
- [ ] 记录模型名
- [ ] 确认客户端表现属于哪一类：
  - [ ] 正常出正文
  - [ ] 返回显式错误
  - [ ] 有 SSE 但无可见正文
  - [ ] 客户端完全无反应

- [ ] 服务端日志已能对应到该时间窗

## 7. 回滚 Checklist

当新版本异常时：

- [ ] 已确认问题确实来自新镜像而非上游瞬时波动
- [ ] 已找到上一版稳定镜像 tag
- [ ] 已用同样运行参数重新起旧镜像
- [ ] 已重新验证 `/api/status`
- [ ] 已确认关键客户端恢复

回滚后还应做：

- [ ] 记录失败镜像 tag
- [ ] 记录失败现象
- [ ] 记录是否保留构建目录和日志供后续排查

## 8. 每次发布后建议补记

建议至少补一条简短记录，写清：

- 发布时间
- 镜像 tag
- 目的
- 验收结果
- 是否回滚

这样后面再查历史，不会只剩聊天记录。
