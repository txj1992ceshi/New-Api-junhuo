# 当前四渠道可用状态表

更新时间：2026-05-10

## 当前架构

当前已收口为：

`客户端登录态 -> 本机池服务 -> 服务器 new-api -> 外部用户请求`

其中服务器侧四条渠道已指向本机 Tailscale 地址 `100.116.117.89`：

- `cursor-pool-proxy -> http://100.116.117.89:3401`
- `windsurf-pool-proxy -> http://100.116.117.89:3003`
- `kiro-pool-proxy -> http://100.116.117.89:3501`
- `codex-pool-proxy -> http://100.116.117.89:3601`

## 当前状态

### Cursor

- 本机登录态：已入池
- `/auth/status`：通过
- `/v1/models`：通过
- `/v1/responses`：通过
- 当前稳定测试模型：
  - `default`
- 备注：
  - 当前账号真实可见模型主要是 `default / gpt-4.1-mini / gpt-4o-mini / gpt-5-mini`

### Windsurf

- 本机登录态：已入池
- `/auth/status`：通过
- `/v1/models`：通过
- `/v1/responses`：通过
- 当前稳定测试模型：
  - `gemini-2.5-flash`
- 当前推荐映射：
  - `gpt-5.5 -> gemini-2.5-flash`
  - `gpt-5.4 -> gpt-4.1-mini`
- 备注：
  - `gpt-5-mini` 已被当前上游废弃，不适合继续做默认测试模型

### Kiro

- 本机登录态：已入池
- `/auth/status`：通过
- `/v1/models`：通过
- `/v1/responses`：通过
- 当前稳定测试模型：
  - `deepseek-3.2`
- 当前推荐映射：
  - `gpt-5.5 -> deepseek-3.2`
  - `gpt-5.4 -> claude-sonnet-4.5`
  - `kiro-auto -> deepseek-3.2`
- 备注：
  - `auto` 不是当前这条 API 通道里的稳定直测模型
  - `claude-haiku-4.5` 可能仍会撞到上游限流

### Codex

- 本机 provider bridge：已入池
- `/auth/status`：通过
- `/v1/models`：通过
- `/v1/responses`：通过
- 当前稳定测试模型：
  - `gpt-5.4`
- 备注：
  - 当前 provider bridge 直连 `http://127.0.0.1:8327`

## 当前结论

截至这次收口，四条渠道都已经找到至少一条真实可推理的稳定模型路径：

- `cursor-pool-proxy`
- `windsurf-pool-proxy`
- `kiro-pool-proxy`
- `codex-pool-proxy`

因此“本机登录态四池供给服务器 new-api”主线已经完成。

## 后续建议

1. 后台测试优先使用当前稳定测试模型，不要回退到旧的废弃模型
2. 如果再次出现 `Too many requests`，优先判断为上游限流，不要先怀疑断连
3. 如果再次出现 `model_deprecated`，优先调整 `test_model` 或 `responses_model_mapping`
4. 如需继续提升稳定性，再做：
   - 渠道侧自动降级
   - 限流后的自动降权
   - 更准确的池状态提示文案
