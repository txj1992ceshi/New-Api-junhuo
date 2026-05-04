# `junhuo` 发布记录

本文档用于按时间顺序记录 `junhuo` 的实际发布历史。

---

## 2026-05-04 15:43 CST

- 发布人：Codex / jj
- 分支：`codex/cursorpro3-control-fixes`
- 提交：`81208e85`
- 镜像 tag：`new-api:codex-responses-capability-81208e85`
- 构建目录：`/opt/new-api/deploy/build-codex-responses-capability-81208e85`
- 发布目标：让真实 Win Codex `/v1/responses` 在选路前按渠道类型能力矩阵分流，默认跳过 Antigravity
- 是否改 env：否
- 是否改数据：否

### 本轮变更

- 将原本分散的 Antigravity 规避判断升级为统一的“Codex `/responses` 渠道类型能力矩阵”
- 对真实 `Codex CLI + /v1/responses(/compact)`：
  - `Antigravity(type=58)` 默认不参与候选
  - `Codex(type=57)` 与已支持真实 Responses 的 OpenAI-compatible 类型继续参与
- 常规 `chat/completions` 与非 Codex 客户端不受该规则误伤

### 发布动作

- 已上传构建目录：是
- 已完成 docker build：是
- 已替换线上容器：是
- 运行参数是否保持不变：是

### 验收

- `/api/status`：正常
- 容器启动日志：正常
- 当前线上容器镜像：`new-api:codex-responses-capability-81208e85`
- 自动刷新任务：正常启动
- Win Codex：本记录只完成部署验活，尚未在此记录中补做客户端实测
- 验收时间窗：约 `2026-05-04 15:39` 至 `15:44 CST`

### 结果

- 发布结果：成功
- 是否回滚：否
- 若回滚，回滚到：不适用

### 补充说明

- 上一版线上镜像是 `new-api:codex-pool-display-095eeab2`
- 这次的核心目标不是修复 Antigravity 对 Codex `/responses` 的协议兼容，而是把它从真实 Codex 主链候选中前移排除
- 因此后续新增同类型渠道实例会自动继承这条规则，不需要再按渠道名补特判

## 2026-05-04 08:52 CST

- 发布人：Codex / jj
- 分支：`codex/cursorpro3-control-fixes`
- 提交：`8611c8be`
- 镜像 tag：`new-api:antigravity-silent-empty-8611c8be`
- 构建目录：`/opt/new-api/deploy/build-antigravity-silent-empty-8611c8be`
- 发布目标：将 Antigravity `/v1/responses` 从“空流显式报错”恢复为“无回复但不显式抛错”的旧表现
- 是否改 env：否
- 是否改数据：否

### 本轮变更

- 回退 Antigravity `/responses` 空流的显式 `response.failed` 处理
- 保留空流检测日志，但不再向 Win Codex 直接抛出 `Antigravity Responses compatibility empty output`

### 发布动作

- 已上传构建目录：是
- 已完成 docker build：是
- 已替换线上容器：是
- 运行参数是否保持不变：是

### 验收

- `/api/status`：正常
- 容器启动日志：正常
- 渠道状态检查：正常
- QQbot / Telegram：未测
- Win Codex：未在本记录中复测
- 验收时间窗：约 `2026-05-04 08:52` 至 `08:53 CST`

### 结果

- 发布结果：成功
- 是否回滚：否
- 若回滚，回滚到：不适用

### 补充说明

- 发布时容器镜像从 `new-api:antigravity-no-tools-4a1c0d0e` 切到 `new-api:antigravity-silent-empty-8611c8be`
- 该版本属于运维行为回退，不代表 Antigravity 对 Win Codex `/v1/responses` 兼容问题已根治
