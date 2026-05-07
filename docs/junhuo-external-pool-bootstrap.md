# `junhuo` 外部池渠道一键落库说明

当你已经具备真实的外部池参数后，可以直接用下面这支脚本把三条渠道落进本地 `one-api.db`：

- [scripts/external_pool_channels.sh](/Users/jj/Documents/Playground/scripts/external_pool_channels.sh)

## 1. 先做只读检查

```bash
cd /Users/jj/Documents/Playground
bash scripts/external_pool_channels.sh check
```

它会输出：

- 当前数据库里是否已有渠道
- `users / tokens / options` 是否为空
- `3003 / 3401 / 3501` 是否在监听
- 三条渠道的核心环境变量是否已提供

## 2. 提供真实参数

最少需要这 6 个环境变量：

```bash
export CURSOR_POOL_BASE_URL="http://127.0.0.1:3401"
export CURSOR_POOL_API_KEY="your-cursor-pool-key"
export WINDSURF_POOL_BASE_URL="http://127.0.0.1:3003"
export WINDSURF_POOL_API_KEY="your-windsurf-pool-key"
export KIRO_POOL_BASE_URL="http://127.0.0.1:3501"
export KIRO_POOL_API_KEY="your-kiro-pool-key"
```

可选项：

- `CURSOR_MODELS`
- `WINDSURF_MODELS`
- `KIRO_MODELS`
- `*_PRIORITY`
- `*_WEIGHT`
- `*_GROUP`
- `*_POOL_STATUS_PATH`
- `*_POOL_ACCOUNTS_PATH`
- `*_POOL_DASHBOARD_PATH`
- `*_POOL_AUTHORIZE_URL`
- `*_POOL_AUTHORIZE_HINT`
- `*_POOL_AUTH_START_PATH`
- `*_POOL_AUTH_COMPLETE_PATH`
- `*_POOL_AUTH_HEADER`
- `*_POOL_AUTH_SCHEME`
- `*_POOL_TUNNEL_HINT`
- `*_POOL_MODE`（示例：`local_state_direct` / `external_managed`）
- `*_POOL_AUTH_STRATEGY`（示例：`manual_callback` / `local_state_sync`）

如果你已经知道后面手动授权要打开哪一个页面，建议现在就一并提供：

```bash
export CURSOR_POOL_AUTHORIZE_URL="http://127.0.0.1:3401/dashboard/login"
export CURSOR_POOL_AUTHORIZE_HINT="完成 Cursor 手动授权后，回到池状态页确认账号数和可用数"
export WINDSURF_POOL_AUTHORIZE_URL="http://127.0.0.1:3003/dashboard/login"
export WINDSURF_POOL_AUTHORIZE_HINT="完成 Windsurf 手动授权后，回到池状态页确认账号数和可用数"
export KIRO_POOL_AUTHORIZE_URL="http://127.0.0.1:3501/dashboard/login"
export KIRO_POOL_AUTHORIZE_HINT="完成 Kiro 手动授权后，回到池状态页确认账号数和可用数"
```

## 3. 正式落库

```bash
cd /Users/jj/Documents/Playground
bash scripts/external_pool_channels.sh apply
```

行为说明：

- 若渠道名不存在，则创建
- 若渠道名已存在，则更新
- 默认渠道名：
  - `cursor-pool-proxy`
  - `windsurf-pool-proxy`
  - `kiro-pool-proxy`

## 4. 落库后立刻做什么

按这个顺序：

1. 看渠道列表状态列
2. 点开 `池状态`
3. 如已配置授权入口，可直接点 `授权登录`
4. 拉取上游模型
5. 验 `/v1/models`
6. 验 `/v1/responses`
7. 回看 `admin_info.external_pool_*`

## 5. 当前已知现场限制

如果脚本 `check` 输出下面这些情况，说明现在还不适合直接落真实渠道：

- `channels count = 0` 且 `users / options / tokens = 0`
  - 说明本地库还是未初始化状态
- `WINDSURF_POOL_API_KEY / CURSOR_POOL_API_KEY / KIRO_POOL_API_KEY = missing`
  - 说明真实池访问密钥还没补齐
- 对应端口没有监听
  - 说明上游池服务还没起来
