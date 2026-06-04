# CoreFusion / new-api 项目合并总览

更新时间：2026-06-05 02:00（Asia/Shanghai）

## 1. 当前结论

这是一个基于 `QuantumNous/new-api` 的 CoreFusion 定制部署项目，当前已经完成：

- 主实例部署目录：`/Users/wuquan/new-api`
- 分销商 OEM 实例目录：`/Users/wuquan/new-api-dealer1`
- CoreFusion 定制源码仓库：`/Users/wuquan/new-api-web-src`
- Docker 镜像构建上下文：`/tmp/docker-ctx`
- 本机主实例容器已运行：`new-api`，端口 `3000`
- 本机分销商实例容器已运行：`new-api-dealer1`，端口 `3002`
- MySQL 与 Redis 已运行，MySQL 当前 healthy
- 后台站点名已统一为：`中科超创 CoreFusion`
- 品牌 Logo 已替换为 `/Users/wuquan/Downloads/logo-export/mark-color.svg`，运行后台使用 SVG data URL，源码默认文件为 `/Users/wuquan/new-api-web-src/web/default/public/logo.svg`
- 控制台 API 信息、系统公告、FAQ 已配置；未接入 Uptime Kuma，已关闭该面板提示
- 公开首页已按当前 API/token 中转站风格重设计，并已切换 `theme.frontend=default` 部署到主实例

当前项目更像是“源码 + 本地部署 + 数据目录 + 运维文档”的组合，不是单一目录里的完整仓库。`/Users/wuquan/new-api` 本身没有 `.git`，真正的源码 Git 仓库在 `/Users/wuquan/new-api-web-src`。

## 2. 开发/部署架构

```text
终端用户
  -> 分销商 OEM 实例 http://localhost:3002
  -> 主实例 http://localhost:3000
  -> 上游 API 渠道 apix.newapi.pro / 其他
  -> 真实大模型
```

主实例：

- 容器：`new-api`
- 镜像：`new-api-corefusion:latest`
- 端口：`3000:3000`
- 数据库：MySQL 8，库名 `newapi`
- 缓存：Redis 7
- 数据目录：`/Users/wuquan/new-api/data`
- MySQL 数据目录：`/Users/wuquan/new-api/mysql`

分销商实例：

- 容器：`new-api-dealer1`
- 镜像：`calciumion/new-api:latest`
- 端口：`3002:3000`
- 数据目录：`/Users/wuquan/new-api-dealer1/data`
- 上游地址：`http://new-api:3000`，通过 Docker 内部网络 `new-api_default` 访问主实例

## 3. 相关文件合并索引

### 3.1 主实例运行目录

```text
/Users/wuquan/new-api/
  DOCS.md                  部署与运营文档，当前最完整的人写说明
  docker-compose.yml       主实例、MySQL、Redis 容器编排
  data/logs/*.log          new-api 运行日志
  mysql/                   主实例 MySQL 数据目录，不建议手动合并或复制内容
```

### 3.2 分销商实例目录

```text
/Users/wuquan/new-api-dealer1/
  docker-compose.yml       分销商 OEM 实例容器编排
  data/                    分销商实例数据目录
  mysql/                   分销商目录下的 MySQL 数据目录
```

### 3.3 CoreFusion 源码仓库

```text
/Users/wuquan/new-api-web-src/
  .git/                    真正的 Git 仓库
  main.go                  Go 服务入口
  go.mod / go.sum          Go 依赖
  Dockerfile.corefusion    CoreFusion 定制构建 Dockerfile，当前未跟踪
  web/default/             当前定制前端
  controller/ model/ service/ relay/ setting/ router/  后端核心模块
```

### 3.4 Docker 构建上下文

```text
/tmp/docker-ctx/
  Dockerfile
  new-api-corefusion-linux  已构建 Linux amd64 二进制，约 88 MB
```

## 4. 源码当前改动情况

源码仓库路径：`/Users/wuquan/new-api-web-src`

最近提交：

```text
87cc22d fix(distributor): resolve model for GET /v1/video/generations/:task_id (#5133)
```

当前有未提交改动：

```text
 M web/default/index.html
 M web/default/src/assets/logo.tsx
 M web/default/src/components/layout/components/footer.tsx
 M web/default/src/components/layout/components/system-brand.tsx
 M web/default/src/features/about/index.tsx
 M web/default/src/features/home/components/sections/features.tsx
 M web/default/src/features/home/components/sections/hero.tsx
 M web/default/src/features/home/components/sections/stats.tsx
 M web/default/src/features/system-settings/site/index.tsx
 M web/default/src/lib/constants.ts
 M web/default/src/lib/theme-customization.ts
 M web/default/src/styles/theme-presets.css
 M web/default/src/styles/theme.css
?? Dockerfile.corefusion
```

改动统计：

```text
13 个已跟踪文件改动：158 行新增，144 行删除
1 个新增未跟踪文件：Dockerfile.corefusion
```

定制方向：

- 品牌名：中科超创 / COREFUSION
- 品牌 Logo：六边形节点网络标志，来自 `/Users/wuquan/Downloads/logo-export/mark-color.svg`
- 主题色：电光蓝 `#1F57F8`、聚变青 `#13B5AB`、深空藏蓝 `#0C1830`
- 定制重点：Logo、系统品牌、API 中转站首页 Hero、功能区、统计区、接入流程、关于页、页脚、站点设置默认值、主题 preset

## 5. 关键配置摘要

主实例 `docker-compose.yml`：

```text
services:
  new-api:
    image: new-api-corefusion:latest
    ports: 3000:3000
    volumes: ./data:/data
    depends_on: mysql healthy, redis started

  mysql:
    image: mysql:8
    volumes: ./mysql:/var/lib/mysql

  redis:
    image: redis:7-alpine
```

分销商 `docker-compose.yml`：

```text
services:
  new-api-dealer1:
    image: calciumion/new-api:latest
    ports: 3002:3000
    volumes: ./data:/data
```

构建上下文 `/tmp/docker-ctx/Dockerfile`：

```text
FROM debian:bookworm-slim
COPY new-api-corefusion-linux /new-api
WORKDIR /data
ENTRYPOINT ["/new-api"]
```

## 6. 日志与风险点

日志目录：`/Users/wuquan/new-api/data/logs`

总日志行数约 6480 行，最近日志显示主实例在周期性同步 options/channels、轮询任务和刷新数据看板。

发现过的异常：

- 2026-06-04 21:32：上游渠道测试返回 401，提示 API key invalid。
- 2026-06-04 21:34：访问 `https://api.openai.com/v1/chat/completions` 连接被拒绝。
- 2026-06-04 22:17：`deepseek-v4-pro` 价格曾未配置，当前主实例与分销商实例均已配置 `ModelRatio`。
- 2026-06-05 00:54 到 00:59：MySQL 曾短暂 connection refused，应是数据库重启或容器未就绪期间。
- 2026-06-05 01:28：出现过一次 nil pointer panic recovered，需要后续定位触发接口。
- 2026-06-05 02:01：分销商曾通过 `host.docker.internal:3000` 调主实例返回 EOF；已改为 Docker 内部地址 `http://new-api:3000` 并接入 `new-api_default` 网络。

当前主实例和分销商 API 均已使用 `deepseek-v4-pro` 完成 200 OK 测试。

后台页面验证：

- `http://localhost:3000/`：主实例首页可打开，标题/Logo/页脚显示 `中科超创 CoreFusion`
- `http://localhost:3000/console`：管理员可登录，控制台 API 信息、公告、FAQ 已显示
- `http://localhost:3002/`：分销商首页可打开，标题/Logo/页脚显示 `中科超创 CoreFusion`
- `http://localhost:3002/v1/chat/completions`：分销商 token 调用 `deepseek-v4-pro` 返回 200 OK

## 7. 已完成事项

- 主实例可本地运行，端口 `3000`
- 分销商 OEM 实例可本地运行，端口 `3002`
- 主实例使用定制镜像 `new-api-corefusion:latest`
- 前端 CoreFusion 品牌定制已进入源码
- Docker 构建上下文已有可运行二进制
- 部署/运营文档已在 `DOCS.md` 中成型
- 主实例和分销商实例的本地数据目录已经存在

## 8. 未完成/建议下一步

1. 提交或备份 `/Users/wuquan/new-api-web-src` 里的 CoreFusion 定制改动。
2. 定位 2026-06-05 01:28 的 nil pointer panic，至少保留对应请求路径和堆栈。
3. 宿主机端口 `3000/3002` 当前由一个 `ssh` 进程监听，容器内部链路已正常，但本机浏览器直连可能仍受影响。
4. 把分销商实例统一切换到 CoreFusion 定制镜像，避免主实例和 OEM 前端品牌不一致。
5. 云服务器上线：域名 `api.supchuang.com`、Docker、nginx、HTTPS、DNS 仍待完成。
6. 生产前不要直接迁移本机 MySQL 裸数据目录，建议导出 SQL dump 或做正式备份恢复。

## 9. 不建议直接合并的内容

以下内容属于运行态数据或敏感数据，不适合直接拼接到一个文本文件里：

- `mysql/` 下的 `.ibd`、binlog、证书、密钥、redo/undo 文件
- `data/` 下可能包含的运行态数据库或上传数据
- `docker-compose.yml` 里的密钥原文
- 数据库 tokens 表里的完整 Key
- `/tmp/docker-ctx/new-api-corefusion-linux` 二进制

这些内容应该做备份、导出或制品归档，而不是文本合并。

## 10. 常用入口

```bash
# 主实例
cd /Users/wuquan/new-api
docker-compose up -d
docker-compose logs -f new-api

# 分销商实例
cd /Users/wuquan/new-api-dealer1
docker-compose up -d

# 源码仓库
cd /Users/wuquan/new-api-web-src
git status --short

# 镜像上下文
ls -lah /tmp/docker-ctx
```
