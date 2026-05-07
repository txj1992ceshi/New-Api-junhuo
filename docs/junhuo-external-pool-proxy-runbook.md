# `junhuo` 外部池代理渠道运维手册

本文面向第一阶段的三条外部池代理渠道：

- `cursor-pool-proxy`
- `windsurf-pool-proxy`
- `kiro-pool-proxy`

目标是让 `new-api` 继续做统一入口，但把账号补池、refresh、session 维护留给外部池服务。

## 1. 最小配置原则

每条渠道都建议满足：

- `type=1`（普通 OpenAI-compatible）
- 单独的 `name`
- 单独的 `models`
- 较低的 `priority`
- `other_info` 中打开对应 `*_pool_proxy=true`

默认约定：

- `base_url`：外部池服务地址
- `key`：外部池服务的只读或代理访问密钥
- `*_pool_status_path`：状态接口
- `*_pool_accounts_path`：账号列表接口
- `*_pool_dashboard_path`：Dashboard 地址
- `*_pool_authorize_url`：手动授权页地址
- `*_pool_auth_start_path`：授权启动接口，默认 `/auth/start`
- `*_pool_auth_complete_path`：授权完成接口，默认 `/auth/complete`
- `*_pool_mode`：池运行模式（如 `local_state_direct` / `external_managed`）
- `*_pool_auth_strategy`：授权策略（如 `manual_callback` / `local_state_sync`）

对应后台 API：

- `GET /api/channel/:id/cursor/auth_view`
- `POST /api/channel/:id/cursor/auth/start`
- `POST /api/channel/:id/cursor/auth/complete`
- `GET /api/channel/:id/windsurf/auth_view`
- `POST /api/channel/:id/windsurf/auth/start`
- `POST /api/channel/:id/windsurf/auth/complete`
- `GET /api/channel/:id/kiro/auth_view`
- `POST /api/channel/:id/kiro/auth/start`
- `POST /api/channel/:id/kiro/auth/complete`

## 2. 后台三类状态判断

后台“帐号/池状态”弹窗要按下面三类理解：

列表页新增的辅助视图可直接配合使用：

- `外部池异常前置`：把当前页异常渠道优先排到前面
- `当前页池快筛`：只筛当前页内存数据，不改后端分页总数
- 顶部状态概览标签：
  - 点击 `认证失败 / 空池 / 连接失败 / 路径错误 / 限流` 等标签，可直接切到对应快筛
  - 点击 `当前页外部池` 总标签，可清空快筛回到完整视图

### 2.1 渠道配置有效，但上游未连通

表现：

- 渠道能保存
- 列表里能看到代理摘要入口
- 弹窗显示 `upstream_error`
- `connection_ok=false`

优先检查：

- `base_url`
- `key`
- 池服务是否在线
- 服务端到池服务的网络

### 2.2 上游连通，但池为空

表现：

- `connection_ok=true`
- `total=0`
- 账号列表为空

说明：

- 这通常不是 `new-api` 问题
- 是上游池还没补到账号

### 2.3 池有账号，可承接请求

表现：

- `connection_ok=true`
- `active > 0`
- 账号列表能看到至少一个可用账号

这时再去做真实 `/v1/responses` 验证。

## 3. 灰度建议

第一阶段不要直接替换主链。

建议顺序：

1. 新增渠道，`priority` 低于现有主链
2. 只暴露少量模型
3. 只给内部 token 或灰度 token
4. 单独验证 `/v1/models`
5. 单独验证 `/v1/responses`
6. 观察 24-48 小时再决定是否提权

## 4. 回滚建议

若某条外部池渠道不稳，优先：

1. 降低 `priority`
2. 缩窄 `models`
3. 直接 `status=disabled`

不要在事故窗口里同时修改：

- `priority`
- `models`
- `base_url`
- `other_info`

否则很难分辨是配置问题还是上游波动。

## 5. 快速排障对照

如果后台已经能看到状态弹窗，优先按下面这张表判断：

### 5.1 `pool_state = upstream_error`

常见含义：

- 状态接口根本没打通

优先排查：

- `401/403`：多半是 `key / auth header / auth scheme` 错
- `404`：多半是 `*_pool_status_path / *_pool_accounts_path` 配错
- `connection refused / timeout / no such host`：多半是 `base_url / 服务在线状态 / 网络` 问题

### 5.2 `pool_state = empty_pool`

常见含义：

- 池服务在线
- 但还没有可用账号

优先排查：

- 是否真的已经补池
- 上游是否把账号成功写进池
- 当前模型是否被池过滤为空

### 5.3 `pool_state = degraded`

常见含义：

- 池里有库存，但错误账号偏多
- 可能还能接请求，但不适合直接提权

优先排查：

- 错误账号数
- 限流状态
- 可用模型列表
- 最近是否有大面积认证失效

## 6. 常见问答

### 6.1 为什么要区分 `*_pool_mode` 和 `*_pool_auth_strategy`？

- `*_pool_mode` 表示这条池怎么跑：本地态直连还是外部托管。
- `*_pool_auth_strategy` 表示这条池怎么拿登录态：手工回调、自动同步或其它自定义流程。

### 6.2 Cursor CLI 已退出登录，但客户端仍登录，为什么还能用？

- 当池服务运行在 `local_state_direct` 模式并能读取本机客户端登录态时，请求不依赖 `cursor-agent login`。
- 若要快速回滚旧行为，可把池服务切到 `cli` 模式，或直接下调该渠道优先级/禁用渠道。

## 7. 分阶段灰度与回滚执行

推荐顺序：`Cursor -> Windsurf -> Kiro`，每次只提一条渠道。

执行命令：

```bash
bash scripts/validate_external_pool_channels.sh
```

每轮放量前至少确认：

- `/auth/status` 为可用
- `/auth/accounts` 有 active 账号
- `/v1/models`、`/v1/responses` 冒烟通过

回滚动作（按优先级从快到慢）：

1. 降低该渠道 `priority`
2. 缩小该渠道 `models`
3. `status=disabled`
4. Cursor 场景可切回 `cli` 模式，恢复旧行为
