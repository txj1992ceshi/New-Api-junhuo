# Windsurf 本机补池 + Vultr 承接 Runbook

## 1. 服务器部署 `WindsurfAPI`

在本机仓库根目录执行：

```bash
cd /Users/jj/Documents/Playground
export REMOTE_HOST=198.13.35.85
export SSH_KEY=~/.ssh/tencent_hk_mac
./scripts/deploy-windsurfapi-to-vultr.sh
```

默认部署结果：

- 服务器目录：`/opt/windsurf-api`
- 应用目录：`/opt/windsurf-api/app`
- 数据目录：`/opt/windsurf-api/data`
- 宿主机发布地址：`127.0.0.1:3003`
- 容器内监听地址：`0.0.0.0:3003`
- LS 端口：`127.0.0.1:42100`

## 2. 本机建立 SSH 隧道

```bash
cd /Users/jj/Documents/Playground
export REMOTE_HOST=198.13.35.85
export SSH_KEY=~/.ssh/tencent_hk_mac
./scripts/start-windsurf-tunnel.sh
```

默认会把：

- 本机 `127.0.0.1:3003`

转发到：

- 服务器 `127.0.0.1:3003`

另开一个终端验证：

```bash
curl -sS http://127.0.0.1:3003/health
curl -sS http://127.0.0.1:3003/v1/models
```

## 3. 本机配置 `windsurf-manager`

编辑：

- `/Users/jj/Documents/Playground/windsurf-manager/.env`

至少填真实值：

```dotenv
CLOUDMAIL_BASE_URL=
CLOUDMAIL_ADMIN_EMAIL=
CLOUDMAIL_ADMIN_PASSWORD=
CLOUDMAIL_DOMAIN=
WINDSURF_POOL_API_BASE_URL=http://127.0.0.1:3003
```

## 4. 手动补池验证

先单账号：

```bash
cd /Users/jj/Documents/Playground/windsurf-manager
source .venv/bin/activate
python windsurf_register.py --count 1 --output-dir auth_output
```

再小批量：

```bash
python windsurf_register.py --count 3 --output-dir auth_output
```

验收点：

- `auth_output/<email>/windsurf_auth_session.json` 已生成
- 注册脚本成功调用 `POST /auth/login`
- 服务器 `WindsurfAPI` 账号池出现新账号

## 5. `New-API` 灰度接入

新增一条低优先级 OpenAI 渠道：

- `name`: `windsurf-api`
- `type`: `1` (`OpenAI`)
- `base_url`: `http://127.0.0.1:3003/v1`
- `group`: `default`
- `priority`: 低于现有主链

建议第一阶段只放少量模型用于验证，例如：

- `gpt-5.5`
- `gpt-5.5-openai-compact`
- `gpt-5.4`

## 6. Codex 回归验证

优先验证：

1. `POST /v1/responses` 纯文本流
2. 带 history/transcript 的 `POST /v1/responses`
3. 带一个简单 function tool 的 `POST /v1/responses`

期望 SSE 至少出现：

- `response.created`
- `response.output_text.delta`
- `response.completed`

当前已知不作为第一阶段成功标准的能力：

- `file_search`
- `computer_use_preview`
- `mcp`
