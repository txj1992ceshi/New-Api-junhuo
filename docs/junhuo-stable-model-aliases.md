# `junhuo` 稳定模型别名约定

本文定义本机 `Cursor / Windsurf / Kiro / Codex` 四条外部池渠道对外暴露的稳定模型名。

目标只有一个：让下游调用方依赖你自己的稳定别名，而不是直接依赖上游客户端池里随时可能变化的原始模型名。

## 1. 设计原则

- 对外优先暴露稳定别名，不直接暴露全部原始模型名
- 每条渠道只保留少量已经验过、可长期维护的入口名
- 映射关系由渠道配置维护，调用方不直接感知上游原始名
- 若未来上游原始模型名变化，只改渠道映射，不要求所有调用方改名

## 2. 当前正式别名

截至当前本机联调状态，建议正式使用以下别名。

### 2.1 Cursor

渠道名：`cursor-pool-proxy`

对外模型：

- `cursor-default`
- `cursor-gpt5-mini`
- `cursor-gpt4o-mini`

当前映射：

- `cursor-default` -> `default`
- `cursor-gpt5-mini` -> `gpt-5-mini`
- `cursor-gpt4o-mini` -> `gpt-4o-mini`

说明：

- `cursor-default` 不是官方固定模型名，而是 `Cursor` 这条渠道的稳定默认入口
- `default` 背后真实落到哪一类模型能力，由当前 `Cursor` 登录态和上游策略决定
- 当前这条渠道已经通过真实 `/v1/responses` 验证，可作为第一优先级本地登录态可调用池

### 2.2 Kiro

渠道名：`kiro-pool-proxy`

对外模型：

- `kiro-sonnet`
- `kiro-haiku`
- `kiro-deepseek`
- `kiro-auto`

当前映射：

- `kiro-sonnet` -> `claude-sonnet-4.5`
- `kiro-haiku` -> `claude-haiku-4.5`
- `kiro-deepseek` -> `deepseek-3.2`
- `kiro-auto` -> `auto`

说明：

- `kiro-auto` 是这条渠道的稳定兜底入口
- 当前 `Kiro` 渠道链路已接通，但仍可能受到上游限流影响
- 别名稳定不代表上游不会 `429/503`，限流仍由渠道降级策略处理

### 2.3 Windsurf

渠道名：`windsurf-pool-proxy`

当前状态：

- 暂不定义正式稳定别名
- 先保留为灰度观察渠道
- 当前账号池已入池，但已验证过的若干原始模型名存在废弃或 entitlement 不稳定问题

建议：

- 在未确认一组长期稳定可调用模型之前，不对外承诺 Windsurf 稳定别名
- 若后续验证出稳定入口，再补独立别名组

### 2.4 Codex

渠道名：`codex-pool-proxy`

对外模型：

- `codex-default`
- `codex-gpt5`
- `codex-gpt5-mini`
- `codex-gpt54`
- `codex-o3-mini`

当前映射：

- `codex-default` -> `gpt-5.4`
- `codex-gpt5` -> `gpt-5`
- `codex-gpt5-mini` -> `gpt-5-mini`
- `codex-gpt54` -> `gpt-5.4`
- `codex-o3-mini` -> `o3-mini`

说明：

- `codex-default` 是这条渠道的稳定默认入口，当前先收口到 `gpt-5.4`
- `codex-gpt54` 与 `codex-default` 当前都指向 `gpt-5.4`，前者偏显式型号，后者偏默认入口
- 这条渠道底层走 `provider_bridge`，真实可用性仍取决于本机 Codex 当前 provider 所指向的上游额度和限流
- 当前这条渠道已经通过本机 `/auth/complete`、`/v1/models` 和最小推理验池

## 3. 当前推荐调用面

如果你现在要给下游客户端、脚本、二次分发服务一组明确入口，推荐先只开放：

- `cursor-default`
- `cursor-gpt5-mini`
- `cursor-gpt4o-mini`
- `kiro-sonnet`
- `kiro-haiku`
- `kiro-deepseek`
- `kiro-auto`
- `codex-default`
- `codex-gpt5`
- `codex-gpt5-mini`
- `codex-gpt54`
- `codex-o3-mini`

暂不建议对外主推：

- `windsurf-pool-proxy` 的原始模型名
- `Cursor` 当前未纳入稳定别名的其它探测模型
- `Kiro` 当前未纳入稳定别名的其它探测模型

## 4. 配置落点

当前这套稳定别名通过渠道配置落地：

- `channels.models`
- `channels.test_model`
- `channels.model_mapping`
- `channels.settings.responses_model_mapping`

这意味着：

- 渠道列表和后台测试优先显示稳定别名
- `/v1/responses` 路径会把稳定别名映射为真实上游模型名
- 调用方可以长期绑定稳定别名，不必感知上游原始模型漂移

## 5. 变更规则

后续若要新增或调整稳定别名，建议遵循：

1. 先在池状态确认账号仍然 `active`
2. 先用真实 `/v1/models`、`/v1/responses` 验证原始模型名
3. 验证通过后再把原始模型名纳入稳定别名
4. 优先增量调整 `model_mapping`，避免直接把全部原始模型放出去
5. 若某个原始模型废弃，只改映射目标，不改对外稳定别名

## 6. 当前优先级建议

- `Cursor = priority 80`
- `Windsurf = priority 70`
- `Kiro = priority 60`

但从实际可用性角度，当前更推荐按以下理解使用：

- `Cursor`：正式可用
- `Codex`：正式可用
- `Kiro`：可用，但受上游限流影响
- `Windsurf`：灰度观察，不承诺稳定模型面
