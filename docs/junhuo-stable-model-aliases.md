# `junhuo` 稳定模型别名约定

当前这套 external-pool 进入双层语义：

- 对外稳定合同：`gpt-5.5`、`gpt-5.4`
- 兼容期旧 alias：继续可路由，但不再作为主推荐模型面

## 1. 设计原则

- 对外稳定合同优先
- 后台继续保留真实模型名
- 旧 alias 只做兼容，不再扩大使用面
- 真实上游变化时，优先改 `responses_model_mapping`

## 2. 当前统一合同

四条渠道都应对外支持：

- `gpt-5.5`
- `gpt-5.4`

这两个名字是能力契约名，不要求真实上游也同名。

## 3. 各渠道当前建议映射

### 3.1 Cursor

- `gpt-5.5` -> `default`
- `gpt-5.4` -> `gpt-5-mini`

兼容旧 alias：

- `cursor-default` -> `default`
- `cursor-gpt5-mini` -> `gpt-5-mini`
- `cursor-gpt4o-mini` -> `gpt-4o-mini`

### 3.2 Windsurf

- `gpt-5.5` -> `gpt-5-mini`
- `gpt-5.4` -> `gemini-2.5-flash`

`Windsurf` 后台仍会看到账号真实可用模型，外部不再直接承诺这些原名。

### 3.3 Kiro

- `gpt-5.5` -> `claude-sonnet-4.5`
- `gpt-5.4` -> `claude-haiku-4.5`

兼容旧 alias：

- `kiro-sonnet` -> `claude-sonnet-4.5`
- `kiro-haiku` -> `claude-haiku-4.5`
- `kiro-deepseek` -> `deepseek-3.2`
- `kiro-auto` -> `auto`

### 3.4 Codex

- `gpt-5.5` -> `gpt-5.5`
- `gpt-5.4` -> `gpt-5.4`

兼容旧 alias：

- `codex-default` -> `gpt-5.4`
- `codex-gpt5` -> `gpt-5`
- `codex-gpt5-mini` -> `gpt-5-mini`
- `codex-gpt54` -> `gpt-5.4`
- `codex-o3-mini` -> `o3-mini`

## 4. 配置落点

当前稳定合同通过这些字段落地：

- `channels.models`
- `channels.test_model`
- `channels.settings.public_models`
- `channels.settings.responses_model_mapping`

其中：

- `public_models` 决定 `/v1/models` 暴露面
- `responses_model_mapping` 决定统一合同如何落到真实上游模型

## 5. 兼容期策略

兼容期内：

- `/v1/models` 应优先只显示 `gpt-5.5 / gpt-5.4`
- 旧 alias 保留在渠道能力里，保证老调用不立刻中断
- 新 token 优先只放行 `gpt-5.5 / gpt-5.4`
