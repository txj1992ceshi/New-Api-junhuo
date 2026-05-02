# `junhuo.icu` 三渠道池切换说明

本说明对应以下目标：

- 香港机 `new-api` 作为公网唯一入口
- `codex-e2e-temp` 账号池继续保留在本机
- `caowo` 与 `antigravity-openclaw` 收敛到香港机
- `gpt-5.4 / gpt-5.5` 由客户端类型决定优先路由

## 1. 渠道标签约定

为避免同名模型继续靠默认 abilities 竞争，香港机侧渠道请固定打以下标签：

- 本机 Codex 回源代理渠道：`codex_pool`
- 香港机 Antigravity 渠道：`antigravity_pool`
- 香港机 Caowo 渠道：`caowo_pool`

代码中的客户端优先级依赖这些标签；未打标签时会回退到历史名称/类型识别。

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
- `tag`: `codex_pool`
- `models`: 仅暴露需要回源本机 Codex 池的模型

该渠道本身不需要是 `ChannelTypeCodex`；它只负责把公网请求回源到本机真实 Codex 池。

### 3.2 Antigravity 渠道

- `tag`: `antigravity_pool`
- `type`: `58`
- `prefer_ipv4`: `true`

`prefer_ipv4` 对应本次代码新增的 `ChannelSettings.PreferIPv4`，也会对 Antigravity 默认生效，用于规避当前 IPv6 上游超时。

### 3.3 Caowo 渠道

- `tag`: `caowo_pool`
- 保持原普通 OpenAI 上游 key 语义

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
   - Codex 客户端优先命中 `codex_pool`
   - OpenClaw / 普通 OpenAI 客户端优先命中 `antigravity_pool` 或 `caowo_pool`
5. 最后再切 `junhuo.icu` 的 Nginx 主入口
