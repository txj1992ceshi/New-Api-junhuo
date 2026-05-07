# `junhuo` Windsurf MVP 验证清单

本文用于把：

- `windsurf-manager`
- `WindsurfAPI`
- `New-Api-junhuo`

组合成一条最小可行的新渠道验证链，并回答两个问题：

1. 这套东西能不能完全放在 Vultr 上跑
2. 它能不能作为 `New-Api-junhuo` 的一条新 Codex-compatible 渠道

本文只关注最小可行验证，不追求第一天就做成生产最终形态。

## 1. 目标边界

本次 MVP 的目标是：

1. 在 Vultr 上常驻 `WindsurfAPI`
2. 在 Vultr 上按需运行 `windsurf-manager`
3. 让 `windsurf-manager` 产出的 Windsurf 会话能进入 `WindsurfAPI` 账号池
4. 在 `New-Api-junhuo` 中新增一条指向 `WindsurfAPI` 的新渠道
5. 通过 `POST /v1/responses` 做一次真实 Codex-compatible 验证

本次 MVP 不要求：

- 替换现有 `codex-e2e-temp`
- 立即承担主流量
- 证明它等价于原生 OpenAI Codex 上游
- 覆盖 `file_search` / `computer_use_preview` / `mcp` 等 OpenAI server-side tools

## 2. 关键结论先记住

### 2.1 `windsurf-manager`

- 更像注册/补池 worker
- 可以部署在服务器
- 不需要 24 小时一直跑
- 适合定时触发或低水位触发

### 2.2 `WindsurfAPI`

- 更像常驻代理层
- 需要 24 小时在线
- 应被 `New-Api-junhuo` 当成普通 OpenAI-compatible 上游渠道接入

### 2.3 和 `CursorPro3` 的本质区别

- `CursorPro3`：本机 GUI/本机 token 落盘/本机强依赖
- `windsurf-manager + WindsurfAPI`：服务端 worker + 服务端代理层

## 3. 前置条件

开始前先确认以下条件成立：

### 3.1 Vultr 主机资源

- 至少 2 核 CPU
- 至少 4 GB 内存
- 至少 20 GB 可用磁盘
- 能稳定访问 `windsurf.com`
- 能稳定访问 `server.self-serve.windsurf.com`

### 3.2 CloudMail 可用

`windsurf-manager` 依赖 CloudMail：

- `CLOUDMAIL_BASE_URL`
- `CLOUDMAIL_ADMIN_EMAIL`
- `CLOUDMAIL_ADMIN_PASSWORD`
- `CLOUDMAIL_DOMAIN`

如果 CloudMail 不可用，本次 MVP 直接失败，不要继续后续步骤。

### 3.3 安全边界

`WindsurfAPI` 默认带 Dashboard，公网部署时必须至少设置：

- `API_KEY`
- `DASHBOARD_PASSWORD`

不要把一个空密码、空 API key 的 `WindsurfAPI` 暴露到公网。

## 4. 目录规划

建议在 Vultr 上新增独立目录：

```text
/opt/windsurf-api
/opt/windsurf-api/data
/opt/windsurf-api/opt/windsurf
/opt/windsurf-manager
/opt/windsurf-manager/output
```

建议把职责分开：

- `/opt/windsurf-api`
  - 代理层代码
  - 持久化账号池
  - LS binary

- `/opt/windsurf-manager`
  - 补池脚本
  - CloudMail 配置
  - 注册产物输出

## 5. 第一阶段：单独跑通 WindsurfAPI

目标：不接 `New-Api-junhuo`，只验证代理层本身能启动和登录。

### 5.1 部署 `WindsurfAPI`

从仓库副本可见，关键环境变量是：

- `PORT`
- `API_KEY`
- `DATA_DIR`
- `CODEIUM_API_KEY` 或 `CODEIUM_AUTH_TOKEN`
- `LS_BINARY_PATH`
- `LS_PORT`
- `DASHBOARD_PASSWORD`

参考：

- [/Users/jj/Documents/Playground/WindsurfAPI/.env.example](/Users/jj/Documents/Playground/WindsurfAPI/.env.example)
- [/Users/jj/Documents/Playground/WindsurfAPI/README.md](/Users/jj/Documents/Playground/WindsurfAPI/README.md)

建议先按 Docker 方式跑，原因是：

- 跟当前 `new-api` 的部署形态一致
- 更容易隔离
- 后续回滚简单

### 5.2 最小启动验收

启动后至少检查：

1. 端口是否监听，例如 `3003`
2. Dashboard 是否可访问
3. 日志中没有 LS 启动失败
4. `/v1/models` 是否能返回模型列表

若以上任一失败，不进入下一阶段。

### 5.3 账号导入最小验证

先不用 `windsurf-manager` 自动注册，优先做最小人工验证：

1. 打开 `https://windsurf.com/show-auth-token`
2. 用 `WindsurfAPI` 的 `/auth/login` 手工导入一个 token
3. 确认账号出现在其池中

目标不是省事，而是先证明：

- 这台 Vultr 上的 `WindsurfAPI` 能独立登录
- 会话池可以在服务器端建立

如果连手工导入都不稳定，不要继续折腾自动补池。

## 6. 第二阶段：单独跑通 windsurf-manager

目标：验证服务器环境里能批量注册并产出会话文件。

### 6.1 配置 `windsurf-manager`

关键变量来自：

- [/Users/jj/Documents/Playground/windsurf-manager/.env.example](/Users/jj/Documents/Playground/windsurf-manager/.env.example)

必填：

- `CLOUDMAIL_BASE_URL`
- `CLOUDMAIL_ADMIN_EMAIL`
- `CLOUDMAIL_ADMIN_PASSWORD`
- `CLOUDMAIL_DOMAIN`

建议额外设置：

- `WINDSURF_POOL_API_BASE_URL=http://127.0.0.1:3003`

这里的目的是让 `windsurf-manager` 直接把新产出的 token/会话导入本机的 `WindsurfAPI`。

### 6.2 单账号注册验证

先不要批量，先只做：

```bash
python windsurf_register.py --count 1 --output-dir auth_output
```

验收标准：

1. 能成功创建邮箱
2. 能收到验证码
3. 能完成注册
4. 能写出 `windsurf_auth_session.json`
5. 能成功调用 `WindsurfPoolAPI /auth/login`

如果在服务器环境里这里失败，说明它还不能替代本机补池。

### 6.3 批量小压测

单账号成功后，再做小规模：

- `--count 3`
- `--count 5`

此时重点观察：

- 成功率
- 单账号耗时
- CloudMail 是否限流
- Windsurf 注册是否触发风控
- 最终有多少账号真正进入 `WindsurfAPI` 池

这一步的目标是回答：

**“它在服务器里不是偶然成功一次，而是能连续补池。”**

## 7. 第三阶段：把 WindsurfAPI 接进 New-Api-junhuo

目标：让 `New-Api-junhuo` 把 `WindsurfAPI` 当新渠道来用。

### 7.1 新渠道定位

建议新增一条独立渠道，不要覆盖 `channel 2 codex-e2e-temp`。

推荐定位：

- `windsurf-codex-fallback`
- 或 `windsurf-api`

不建议一开始就给高优先级。

### 7.2 渠道类型建议

建议先按普通 OpenAI-compatible 上游接入，而不是自定义奇怪 channel type。

原因：

- `WindsurfAPI` 对外暴露的是兼容接口
- 先减少 `New-Api-junhuo` 侧改动
- 失败时更容易隔离问题到底在上游还是在 `new-api`

### 7.3 初始模型暴露策略

第一轮只开放少量模型别名，不要全开。

建议只选：

- 一个 Claude 风格模型
- 一个 GPT 风格模型

并且只给灰度 token 或内部 token 测。

## 8. 第四阶段：做真实 Codex 验证

目标：验证它不是只能聊天，而是真能承接一部分 Codex 工作流。

### 8.1 必测接口

优先测：

- `POST /v1/responses`

不要只测：

- `/v1/chat/completions`

因为聊天通不代表 Codex 真能用。

### 8.2 最小 Codex 验证样例

至少覆盖：

1. 文本 Responses 流式返回
2. 带历史 transcript 的多轮请求
3. 一个本地 function tool
4. 工具结果回传后的第二轮完成

### 8.3 明确不纳入 MVP 通过标准的项

以下项在 MVP 阶段失败，不判整个方案死刑，但必须单独记录：

- `file_search`
- `computer_use_preview`
- `mcp`

原因是本地代码已经明确说明这些属于未桥接 server-side tools。

## 9. 第五阶段：自动补给验证

目标：回答它能不能“像服务端补池系统一样自动工作”。

### 9.1 建议的第一版自动补给形态

不要一上来写复杂编排，先做最小版本：

1. `WindsurfAPI` 常驻
2. `windsurf-manager` 用 cron 定时跑
3. 每次跑 `--count 1` 或 `--count 2`
4. 导入成功后由 `WindsurfAPI` 池承接

例如可以先按：

- 每 30 分钟补 1 个
- 或每 1 小时补 2 个

先跑一天。

### 9.2 自动补给验收标准

连续 24 小时内至少确认：

1. `WindsurfAPI` 服务没有挂
2. `windsurf-manager` 定时任务有执行记录
3. 新账号能持续进入池
4. 新渠道没有明显持续 5xx
5. `/v1/responses` 能至少重复成功多次

如果 24 小时后：

- 补池持续成功
- 代理层持续可用
- `New-Api-junhuo` 新渠道可反复调用

才可以进入下一步灰度。

## 10. 灰度建议

MVP 通过后，也不要立刻让它替换现有主路径。

建议顺序：

1. 只给内部 token 使用
2. 只给指定模型使用
3. 只做 fallback，不做 primary
4. 连续观察 48 小时
5. 再决定是否提高优先级

## 11. 失败时怎么判断是哪层坏了

### 11.1 `windsurf-manager` 坏

现象通常是：

- 注册失败
- 验证码收不到
- 会话文件没生成
- `/auth/login` 导入不上去

### 11.2 `WindsurfAPI` 坏

现象通常是：

- `/v1/models` 异常
- 账号池存在但请求失败
- LS 启动失败
- `/v1/responses` 抽风

### 11.3 `New-Api-junhuo` 接入层坏

现象通常是：

- `curl` 直打 `WindsurfAPI` 成功
- 但经 `New-Api-junhuo` 新渠道失败
- 渠道配置、模型映射、鉴权头或流式转发有问题

## 12. 最终判断标准

只有同时满足以下 4 条，才能算这条路成立：

1. `windsurf-manager` 能在 Vultr 服务器环境里稳定产出会话
2. `WindsurfAPI` 能在 Vultr 上稳定维持会话池并常驻提供服务
3. `New-Api-junhuo` 新渠道能稳定转发到 `WindsurfAPI`
4. `POST /v1/responses` 在真实 Codex-compatible 请求下可重复成功

如果只满足前 3 条，不满足第 4 条，那它只是“能聊天的代理池”，不是你要的 Codex 渠道。

## 13. 建议的执行顺序

按这个顺序执行最省时间：

1. 单独启动 `WindsurfAPI`
2. 手工导入一个 Windsurf token
3. 直打 `WindsurfAPI /v1/responses`
4. 单独运行 `windsurf-manager --count 1`
5. 确认它能把新账号导入 `WindsurfAPI`
6. 在 `New-Api-junhuo` 新增灰度渠道
7. 经 `New-Api-junhuo` 做真实 `/v1/responses` 回归
8. 再做 cron 自动补给

这个顺序的核心是：

- 先证明代理可用
- 再证明补池可用
- 最后证明接入 `new-api` 后仍可用
