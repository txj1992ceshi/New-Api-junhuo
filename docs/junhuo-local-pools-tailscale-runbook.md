# 本机四池供给服务器运行手册（Tailscale）

## 目标链路

固定为：

`客户端登录态 -> 本机池服务 -> 服务器 new-api -> 外部用户请求`

这版不做 token/账号复制到服务器数据库。服务器实时消费本机池状态。

## 1. 前提

- 本机四池已能本地启动：
  - Cursor `3401`
  - Windsurf `3003`
  - Kiro `3501`
  - Codex `3601`
- 本机和服务器加入同一 Tailscale 网络
- 服务器代码目录：`/opt/new-api/src`
- 服务器数据库：`/opt/new-api/data/one-api.db`

## 2. 本机准备 Tailscale 监听

先为四池写入监听配置：

```bash
cd /Users/jj/Documents/Playground
TAILSCALE_BIND_HOST=0.0.0.0 bash scripts/prepare_local_pool_tailscale_env.sh apply
```

如果你想只绑到某个 Tailscale IP，也可以把 `0.0.0.0` 换成那个 IP。

然后重启四池：

```bash
cd /Users/jj/Documents/Playground
bash scripts/manage_local_pools_terminal.sh restart
```

检查状态：

```bash
bash scripts/manage_local_pools_terminal.sh status
bash scripts/manage_local_pools_terminal.sh logs
```

## 3. 找出本机 Tailscale IP

本机执行：

```bash
tailscale ip -4
```

假设输出是 `100.x.y.z`，下面都用这个地址。

## 4. 验证服务器能看到本机四池

先在服务器上逐条打状态接口：

```bash
curl -H 'Authorization: Bearer demo-cursor-key' http://100.x.y.z:3401/auth/status
curl -H 'Authorization: Bearer demo-windsurf-key' http://100.x.y.z:3003/auth/status
curl -H 'Authorization: Bearer demo-kiro-key' http://100.x.y.z:3501/auth/status
curl -H 'Authorization: Bearer demo-codex-key' http://100.x.y.z:3601/auth/status
```

任一失败，先排：

- 本机池是否还在运行
- Tailscale 是否在线
- 本机防火墙是否拦截 Tailscale 入站

## 5. 把服务器四渠道切到本机池

在本机执行：

```bash
cd /Users/jj/Documents/Playground
MAC_TAILSCALE_IP=100.x.y.z bash scripts/apply_tailscale_remote_pool_channels.sh
```

这个脚本会通过 SSH 登录服务器，并调用 `scripts/external_pool_channels.sh apply`，把四条渠道改成：

- `cursor-pool-proxy -> http://100.x.y.z:3401`
- `windsurf-pool-proxy -> http://100.x.y.z:3003`
- `kiro-pool-proxy -> http://100.x.y.z:3501`
- `codex-pool-proxy -> http://100.x.y.z:3601`

同时把 `*_pool_tunnel_hint` 写成 `tailscale://...`。

## 6. 授权语义

第一阶段统一约定：

- 只在本机做“读取登录态 / 授权登录”
- 服务器不再直接读取客户端登录态
- 本机入池一次即可
- 服务器消费的是本机池，不需要再重复授权一次

## 7. 验证顺序

服务器后台按下面顺序验：

1. `codex-pool-proxy`
2. `cursor-pool-proxy`
3. `windsurf-pool-proxy`
4. `kiro-pool-proxy`

每条都看：

- 池状态弹窗
- `/v1/models`
- `/v1/responses`

## 8. 故障分流

### 池掉了

现象：

- `auth/status` 连接失败
- 后台应显示连接失败，而不是缺配置

处理：

```bash
cd /Users/jj/Documents/Playground
bash scripts/manage_local_pools_terminal.sh restart
```

### Tailscale 不通

现象：

- 本机 `127.0.0.1` 正常
- 服务器访问 `100.x.y.z:port` 不通

处理：

- 确认两端都在线
- 确认 `tailscale ip -4` 返回值未变
- 确认本机监听地址已不是只绑 `127.0.0.1`

### 已认证但不可推理

现象：

- `/auth/status` 正常
- `/auth/accounts` 有账号
- `new-api` 测试失败

这通常不是隧道问题，而是上游账号/模型问题。

### 配置误报或鉴权失败

优先核对：

- `*_pool_base_url`
- `*_pool_api_key`
- `*_pool_tunnel_hint`

服务器配置改错时，重新执行一次：

```bash
MAC_TAILSCALE_IP=100.x.y.z bash scripts/apply_tailscale_remote_pool_channels.sh
```
