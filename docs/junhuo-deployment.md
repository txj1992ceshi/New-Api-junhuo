# `junhuo` 部署说明

本文档记录 `junhuo` 当前采用的实际部署方式，重点是帮助维护者完成：

- 发布新版本
- 替换线上容器
- 保留持久数据
- 进行最小运行态验证

## 1. 当前部署约定

当前线上使用 Docker 单容器模式：

- 容器名：`new-api`
- 运行网络：`host`
- 数据挂载：`/opt/new-api/data:/data`
- 环境文件：`/opt/new-api/deploy/new-api.env`

不建议在没有明确原因的情况下改变以上结构。

## 2. 线上目录约定

宿主机上常见目录：

- `/opt/new-api/data`
  - 运行数据
  - SQLite 数据库
  - 日志

- `/opt/new-api/deploy`
  - 部署入口目录
  - 环境文件
  - 构建源码目录
  - 保留的稳定部署样本

常见构建目录命名方式：

- `/opt/new-api/deploy/build-<tag>`

常见镜像命名方式：

- `new-api:<tag>`

## 3. 推荐发布流程

推荐流程是：

1. 在本地仓库完成修改
2. 提交并推送对应分支
3. 用 `git archive` 将当前 HEAD 打包推送到服务器构建目录
4. 在服务器上构建新镜像
5. 替换 `new-api` 容器
6. 做最小运行态验证

## 4. 参考发布命令

下面是当前常用模式，`<tag>` 建议使用提交号或有语义的构建标签。

### 4.1 上传当前代码到服务器构建目录

```bash
cd /path/to/New-Api-junhuo
git archive --format=tar HEAD | ssh <server> \
  "rm -rf /opt/new-api/deploy/build-<tag> && \
   mkdir -p /opt/new-api/deploy/build-<tag> && \
   tar -xf - -C /opt/new-api/deploy/build-<tag>"
```

### 4.2 在服务器构建镜像

```bash
ssh <server> \
  "cd /opt/new-api/deploy/build-<tag> && \
   docker build -t new-api:<tag> ."
```

### 4.3 替换线上容器

```bash
ssh <server> \
  "docker rm -f new-api && \
   docker run -d \
     --name new-api \
     --restart unless-stopped \
     --network host \
     --env-file /opt/new-api/deploy/new-api.env \
     -v /opt/new-api/data:/data \
     new-api:<tag>"
```

## 5. 最小验收

替换容器后，至少检查：

### 5.1 容器是否已切到新镜像

```bash
docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}"
```

### 5.2 服务状态是否正常

```bash
curl http://127.0.0.1:3000/api/status
```

### 5.3 启动日志是否正常

```bash
docker logs --tail 80 new-api
```

重点看：

- 数据库初始化是否正常
- 自动刷新任务是否启动
- 没有明显 panic 或启动失败

## 6. 发布时不要动的东西

除非任务明确要求，否则不要在发布时顺手修改：

- `/opt/new-api/deploy/new-api.env`
- `/opt/new-api/data`
- 数据库 schema
- 当前公网域名入口
- Nginx / 反代结构

部署的默认目标应该是：

- 换镜像
- 不换环境
- 不清数据

## 7. 回滚思路

若新镜像行为异常：

1. 找到上一版稳定镜像 tag
2. 用相同运行参数重新起旧镜像
3. 验证 `/api/status`
4. 再决定是否保留失败构建目录供排查

回滚时的核心原则是：

- 不动 `/data`
- 只回滚镜像

## 8. 现有已知注意事项

- 服务器上 `docker build` 的 Go 编译阶段有时较慢，这是常态，不等于卡死
- Antigravity 链路与 Codex `/v1/responses` 的兼容问题，和部署流程本身是两件事
- 发布前后要分清是“镜像问题”还是“渠道协议问题”

## 9. 进一步参考

建议和本文一起看：

- [发布与回滚 Checklist](./junhuo-release-checklist.md)
- [渠道配置样例](./junhuo-channel-config-examples.md)
