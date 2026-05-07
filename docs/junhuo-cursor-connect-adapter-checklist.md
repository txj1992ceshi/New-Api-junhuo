# Cursor Direct Adapter 改造清单

## 已确认的调用形状

- Cursor 本机登录态仍然落在 `state.vscdb`
  - `cursorAuth/cachedEmail`
  - `cursorAuth/accessToken`
  - `cursorAuth/refreshToken`
- Cursor 推理上游宿主来自 `cursorai/serverConfig.agentUrlConfig.agentnUrl`
  - 当前本机样本为 `https://agentn.global.api5.cursor.sh`
- 该宿主不是 OpenAI REST
  - `GET /v1/models` 返回 `404`
  - `POST /v1/responses` 返回 `404`
- 客户端包内存在 `@connectrpc/connect`、`protocol-connect`、`protocol-grpc-web` 依赖
- 本机日志里持续出现 `ConnectError`、`http/2 stream closed`、`ECONNRESET`

结论：

- 现有 Cursor `local_state_direct` 的“读本机态入池”链路成立
- 真正缺的是“池服务 -> Cursor Connect 宿主”的 direct adapter

## 第一版适配目标

- 不改坏现有：
  - `local_state_direct`
  - `/auth/start`
  - `/auth/complete`
  - Cursor 本机态入池
- 仅在 `local-pool-service` 给 Cursor `direct` 增加一条新协议分支：
  - `CURSOR_DIRECT_PROTOCOL=connect`
- 让本地池服务可以按配置向 Connect 宿主发请求
- 先支持 unary JSON 适配
- 先不承诺 grpc-web binary framing / server-streaming 完整复刻

## 第一版已落的配置面

- `CURSOR_DIRECT_PROTOCOL=rest|connect`
- `CURSOR_CONNECT_MODELS_PATH`
- `CURSOR_CONNECT_RESPONSES_PATH`
- `CURSOR_CONNECT_CHAT_COMPLETIONS_PATH`
- `CURSOR_CONNECT_PROTOCOL_VERSION`
- `CURSOR_CONNECT_ACCEPT`
- `CURSOR_CONNECT_CONTENT_TYPE`
- `CURSOR_CONNECT_TIMEOUT_MS`
- `CURSOR_CONNECT_PAYLOAD_MODE=passthrough|prompt_model|chat_model_messages`
- `CURSOR_CONNECT_MODEL_PATHS`
- `CURSOR_CONNECT_TEXT_PATHS`
- `CURSOR_CONNECT_EXTRA_HEADERS_JSON`

## 第一版行为

### `/v1/models`

- 当 `CURSOR_DIRECT_PROTOCOL=connect` 时：
  - 走 `POST {CURSOR_DIRECT_BASE_URL}{CURSOR_CONNECT_MODELS_PATH}`
  - 从返回体里按 `CURSOR_CONNECT_MODEL_PATHS` 抽模型名

### `/v1/responses`

- 当 `CURSOR_DIRECT_PROTOCOL=connect` 时：
  - 走 `POST {CURSOR_DIRECT_BASE_URL}{CURSOR_CONNECT_RESPONSES_PATH}`
  - Header 默认带：
    - `Connect-Protocol-Version`
    - `Content-Type`
    - `Accept`
    - `Authorization`
  - 请求体按 `CURSOR_CONNECT_PAYLOAD_MODE` 生成
  - 响应按 `CURSOR_CONNECT_TEXT_PATHS` 抽文本

### `/v1/chat/completions`

- 当 `CURSOR_DIRECT_PROTOCOL=connect` 时：
  - 优先走 `CURSOR_CONNECT_CHAT_COMPLETIONS_PATH`
  - 若未配置则回退到 `CURSOR_CONNECT_RESPONSES_PATH`
  - 仍回包成 OpenAI chat completion 兼容格式

## 推荐联调顺序

1. 先确定一个真实 Connect method path
2. 配 `CURSOR_CONNECT_MODELS_PATH`
3. 配 `CURSOR_CONNECT_RESPONSES_PATH`
4. 先用 `CURSOR_CONNECT_PAYLOAD_MODE=passthrough`
5. 若 400/422，再切到：
   - `prompt_model`
   - 或 `chat_model_messages`
6. 根据真实返回体调：
   - `CURSOR_CONNECT_MODEL_PATHS`
   - `CURSOR_CONNECT_TEXT_PATHS`
7. 最后再看是否需要：
   - grpc-web content type
   - 特殊 header
   - server-streaming

## 还没收掉的风险

- 真实 service/method 名目前还未在客户端包里完全定位出来
- 真实宿主可能要求：
  - grpc-web binary framing
  - 特定 metadata header
  - Connect GET/POST 特殊约定
  - 非通用 JSON body 结构
- 第一版更像“可配置 Connect 探针适配器”
  - 不是最终定版的 Cursor 原生协议复刻

## 下一步最值钱的两件事

1. 从 Cursor 客户端或日志里继续定位真实 method path
2. 用真实 path + header 组合，做一次 `models` 和最小 `responses` 联调
