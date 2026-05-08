# `junhuo` 客户端稳定模型调用约定

本文把当前已验证的稳定模型别名，收口成客户端、脚本、令牌分发侧可直接使用的一套调用约定。

配套参考：

- [junhuo-stable-model-aliases.md](/Users/jj/Documents/Playground/docs/junhuo-stable-model-aliases.md)

## 1. 当前建议对外开放的模型名

建议当前只对外公开以下稳定别名：

- `cursor-default`
- `cursor-gpt5-mini`
- `cursor-gpt4o-mini`
- `codex-default`
- `codex-gpt5`
- `codex-gpt5-mini`
- `codex-gpt54`
- `codex-o3-mini`
- `kiro-sonnet`
- `kiro-haiku`
- `kiro-deepseek`
- `kiro-auto`

当前不建议公开：

- `Windsurf` 原始模型名
- `Cursor / Kiro` 尚未纳入稳定别名的原始模型名

## 2. 给客户端的统一规则

客户端只应该知道两件事：

1. 你的 `base_url`
2. 你的稳定模型别名

客户端不应该直接依赖：

- `default`
- `gpt-5-mini`
- `gpt-4o-mini`
- `claude-sonnet-4.5`
- `deepseek-3.2`
- 其它上游原始模型名

## 3. 推荐优先级

若客户端需要默认模型，建议按以下顺序理解：

1. `cursor-default`
2. `codex-default`
3. `cursor-gpt5-mini`
4. `codex-gpt54`
5. `kiro-sonnet`
6. `kiro-haiku`
7. `kiro-deepseek`
8. `kiro-auto`

说明：

- `cursor-default` 是当前最适合做默认入口的稳定名
- `codex-default` 是当前 Codex 渠道的稳定默认入口
- `kiro-auto` 是兜底入口，不建议优先于显式模型

## 4. OpenAI 兼容调用样例

### 4.1 `/v1/responses`

```bash
curl http://127.0.0.1:3000/v1/responses \
  -H 'Authorization: Bearer YOUR_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "cursor-default",
    "input": "hello"
  }'
```

### 4.2 `/v1/chat/completions`

```bash
curl http://127.0.0.1:3000/v1/chat/completions \
  -H 'Authorization: Bearer YOUR_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "codex-gpt54",
    "messages": [
      {"role": "user", "content": "hello"}
    ]
  }'
```

## 5. 令牌侧建议

若某个 token 只希望开放稳定模型面，建议给该 token 开启模型限制，并只填稳定别名。

推荐模型限制列表示例：

```json
[
  "cursor-default",
  "cursor-gpt5-mini",
  "cursor-gpt4o-mini",
  "codex-default",
  "codex-gpt5",
  "codex-gpt5-mini",
  "codex-gpt54",
  "codex-o3-mini",
  "kiro-sonnet",
  "kiro-haiku",
  "kiro-deepseek",
  "kiro-auto"
]
```

如果你想做更保守的默认 token，可只开放：

```json
[
  "cursor-default",
  "cursor-gpt5-mini",
  "codex-default",
  "kiro-sonnet"
]
```

## 6. 渠道波动时的调用建议

当前按真实状态建议这样使用：

- `Cursor`：主入口
- `Codex`：第二主入口
- `Kiro`：补充入口
- `Windsurf`：观察位，不纳入默认客户端模型面

若某段时间 `Kiro` 上游频繁限流：

- 客户端默认模型切回 `cursor-default`
- 不要要求调用方临时改原始模型名
- 只在后台调整优先级或别名映射

## 7. 运维动作

当你新增或调整稳定别名后，建议按这个顺序同步：

1. 更新渠道 `models`
2. 更新 `test_model`
3. 更新 `model_mapping`
4. 更新 `settings.responses_model_mapping`
5. 更新本文档
6. 如有模型限制 token，再同步 token 的 `model_limits`

## 8. 当前不做的事

当前这套约定先不自动改 token 的 `model_limits`，原因是：

- 你可能有不同 token 对应不同模型面
- 你未必希望所有 token 立刻只剩稳定别名
- 更稳的做法是先有统一清单，再按 token 分批收口
