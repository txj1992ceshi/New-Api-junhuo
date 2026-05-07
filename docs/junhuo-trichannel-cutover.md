# `junhuo.icu` 三渠道池统一切换说明

本说明对应以下目标：

- 香港机 `new-api` 作为公网唯一入口
- `codex-e2e-temp` 账号池继续保留在本机
- `caowo` 与 `antigravity-openclaw` 收敛到香港机
- `gpt-5.4 / gpt-5.5` 按统一渠道优先级故障切换

## 1. 统一优先级约定

`new-api` 继续使用原生的渠道选择思路，不再按客户端类型做额外分流。

同名模型的渠道顺序固定为：

1. `caowo`
2. `codex-e2e-temp`
3. `antigravity-openclaw`

也就是：

- `caowo` 可用时优先使用 `caowo`
- `caowo` 不可用时回退到本机 `codex-e2e-temp`
- `codex-e2e-temp` 也不可用时再回退到 `antigravity-openclaw`

实现上依赖渠道/abilities 的 `priority` 配置，而不是客户端识别。

## 2. 本机 Codex 池回源方式

不要把本机 `codex-e2e-temp` 的 OAuth 池复制到香港机。

推荐做法：

1. 在本机真实生产 `new-api` 中创建一个 **管理员内部 token**
2. 让香港机回源本机时使用：

   - `Authorization: Bearer sk-<internal_token>-2`

其中 `-2` 利用现有“管理员 token 可指定 channel id”的机制，强制本机只走 `channel_id=2` 的 `codex-e2e-temp`。

这样香港机只把请求转发到本机 Codex 池，不会误用本机其它同名渠道。

## 3. 香港机渠道建议

### 3.1 本机 Codex 回源代理渠道

建议在香港机新增一个 OpenAI 兼容代理渠道，指向 chisel 暴露的本机入口：

- `base_url`: `http://127.0.0.1:18080`
- `key`: `sk-<internal_token>-2`
- `models`: 仅暴露需要回源本机 Codex 池的模型

该渠道本身不需要是 `ChannelTypeCodex`；它只负责把公网请求回源到本机真实 Codex 池。

### 3.2 Antigravity 渠道

- `type`: `58`
- `prefer_ipv4`: `true`

`prefer_ipv4` 对应本次代码新增的 `ChannelSettings.PreferIPv4`，也会对 Antigravity 默认生效，用于规避当前 IPv6 上游超时。

### 3.3 Caowo 渠道

- 保持原普通 OpenAI 上游 key 语义
- 对需要统一切换的模型，优先级高于 `codex-e2e-temp` 与 `antigravity-openclaw`

### 3.4 第一阶段外部池代理渠道

若本轮要把 `Cursor / Windsurf / Kiro` 先作为灰度外部池代理接入，建议遵循：

1. 每条都新建独立 OpenAI-compatible 渠道
2. `type` 先保持普通 OpenAI-compatible，不新增特殊 channel type
3. 在 `other_info` 中分别打开：
   - `cursor_pool_proxy=true`
   - `windsurf_pool_proxy=true`
   - `kiro_pool_proxy=true`
4. 每条渠道只先暴露少量模型
5. `priority` 先低于现有主链

建议命名：

- `cursor-pool-proxy`
- `windsurf-pool-proxy`
- `kiro-pool-proxy`

这样后台可直接查看帐号/池状态，但不会一上来抢走主流量。

## 4. Nginx 切换

香港机 Nginx 需要从：

- `location / -> http://127.0.0.1:18080`

切换为：

- `location / -> http://127.0.0.1:3000`

切换后：

- `junhuo.icu` 的公网主入口直接命中香港机 `new-api`
- `chisel` 仅作为香港机回源本机 Codex 池的内部链路保留

## 5. 验证顺序

1. 香港机本地验证 `caowo`
2. 香港机本地验证 `antigravity-openclaw`
3. 香港机通过回源代理验证本机 `codex-e2e-temp`
4. 验证 `gpt-5.4 / gpt-5.5`：
   - 所有客户端统一优先命中 `caowo`
   - `caowo` 不可用时回退到本机 `codex-e2e-temp`
   - `codex-e2e-temp` 不可用时回退到 `antigravity-openclaw`
5. 分别验证 `cursor-pool-proxy / windsurf-pool-proxy / kiro-pool-proxy`：
   - `/v1/models`
   - `/v1/responses`
   - 后台“帐号/池状态”是否能区分未连通 / 空池 / 有可用账号
6. 最后再切 `junhuo.icu` 的 Nginx 主入口
