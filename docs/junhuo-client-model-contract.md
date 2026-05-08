# `junhuo` 客户端统一模型契约

当前四条 external-pool 渠道对外统一成两层语义：

- 对外客户端、令牌、脚本：只推荐 `gpt-5.5`、`gpt-5.4`
- 管理后台、池状态、账号详情：继续显示真实上游模型名

配套参考：

- [junhuo-stable-model-aliases.md](/Users/jj/Documents/Playground/docs/junhuo-stable-model-aliases.md)

## 1. 当前正式对外模型名

建议对外只使用：

- `gpt-5.5`
- `gpt-5.4`

兼容期内，旧 alias 仍可能可调用，但不再推荐给新客户端。

## 2. 统一规则

调用方只需要知道：

1. `base_url`
2. `gpt-5.5 / gpt-5.4`

调用方不应该直接依赖各池里的真实模型名，也不应该把后台里看到的真实 `Claude / Gemini / DeepSeek / mini` 当成稳定外部合同。

## 3. 推荐理解

- `gpt-5.5`：主入口，代表“当前渠道最优能力”
- `gpt-5.4`：辅入口，代表“当前渠道更稳或次优能力”

这里的名字是统一契约名，不要求真实上游也刚好叫这个名字。

## 4. 调用样例

### 4.1 `/v1/responses`

```bash
curl http://127.0.0.1:3000/v1/responses \
  -H 'Authorization: Bearer YOUR_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gpt-5.5",
    "input": "hello"
  }'
```

### 4.2 `/v1/chat/completions`

```bash
curl http://127.0.0.1:3000/v1/chat/completions \
  -H 'Authorization: Bearer YOUR_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gpt-5.4",
    "messages": [
      {"role": "user", "content": "hello"}
    ]
  }'
```

## 5. 令牌建议

新 token 建议优先只开放：

```json
[
  "gpt-5.5",
  "gpt-5.4"
]
```

兼容期内，如需兼容旧调用，可在后台继续保留旧 alias，但默认分发面应收口到这两个名字。

## 6. 运维同步顺序

每次调整统一合同后，按这个顺序同步：

1. 更新渠道 `models`
2. 更新渠道 `test_model`
3. 更新 `settings.public_models`
4. 更新 `settings.responses_model_mapping`
5. 更新本文档
6. 按需同步 token `model_limits`
