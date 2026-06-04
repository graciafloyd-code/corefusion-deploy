# 中科超创 CoreFusion — 部署与运营文档

## 目录结构

```
~/new-api/              主实例（上游渠道商后台）
~/new-api-dealer1/      分销商 OEM 实例 #1
~/new-api-web-src/      前端源码（CoreFusion 定制版）
/tmp/docker-ctx/        Docker 镜像构建上下文
```

---

## 一、架构说明

```
终端用户
    ↓
分销商 OEM 实例（localhost:3002）
    ↓  使用分销商专属 Token + default 分组（倍率 1.4x）
主实例（localhost:3000）
    ↓
上游 API 渠道（apix.newapi.pro / 其他）
    ↓
真实大模型（deepseek-v4-pro 等）
```

---

## 二、主实例

### 启动 / 停止

```bash
cd ~/new-api
docker-compose up -d       # 启动
docker-compose down        # 停止
docker-compose restart new-api   # 重启（不影响 MySQL/Redis）
docker-compose logs -f new-api   # 查看日志
```

### 配置文件

- `~/new-api/docker-compose.yml` — 容器配置
- 数据库：MySQL 8，密码 `newapi123`，库名 `newapi`
- 端口：3000

### 关键数据库操作

```bash
# 进入 MySQL
docker exec -it new-api-mysql mysql -uroot -pnewapi123 newapi

# 查看所有令牌
SELECT name, \`group\`, remain_quota, used_quota FROM tokens;

# 给分销商令牌增加额度（充值后操作）
UPDATE tokens SET remain_quota = remain_quota + 50000000 WHERE name = 'dealer_standard_01';
```

### 分销商令牌

| 名称 | 分组 | 倍率 | Key |
|---|---|---|---|
| dealer_standard_01 | default | 1.4x | 见数据库 |
| dealer_pro_01 | vip | 1.25x | 见数据库 |
| dealer_strategic_01 | svip | 1.1x | 见数据库 |

```bash
# 查看完整 Key
docker exec new-api-mysql mysql -uroot -pnewapi123 newapi \
  -e "SELECT name, \`group\`, \`key\` FROM tokens;"
```

### 分组倍率

- `default`（标准代理）：1.4x
- `vip`（专业代理）：1.25x
- `svip`（战略代理）：1.1x

---

## 三、分销商 OEM 实例

### 启动 / 停止

```bash
cd ~/new-api-dealer1
docker-compose up -d
docker-compose down
```

- 端口：3002（3001 被本地 Node 进程占用）
- 数据库：SQLite，路径 `~/new-api-dealer1/data/one-api.db`
- 上游渠道：`http://new-api:3000`（通过 Docker 内部网络指向主实例，不加 /v1）

### SQLite 操作注意事项

由于是 volume 挂载，修改数据库须直接操作宿主机文件：

```bash
python3 -c "
import sqlite3
conn = sqlite3.connect('/Users/wuquan/new-api-dealer1/data/one-api.db')
# 操作...
conn.commit()
conn.close()
"
```

**不要用 docker cp 改数据库再重启，重启会读 volume 里的文件覆盖。**

### 新增分销商的完整流程

1. **主实例** → 新建令牌，绑定对应分组，设置额度
2. **新建 OEM 目录**（复制 `new-api-dealer1` 目录结构）
3. **修改端口**（避免冲突，如 3003、3004...）
4. **启动实例**，注册 root 用户
5. **提升 root 为管理员**（修改 SQLite users.role=100）
6. **写入渠道配置**（SQLite channels 表）
7. **写入 abilities 路由缓存**（SQLite abilities 表）
8. **写入系统选项**（ModelRatio、GroupRatio、QuotaPerUnit）
9. **设置 Base URL** 为 `http://new-api:3000`（不加 /v1），并确保 OEM 容器加入主实例 Docker 网络 `new-api_default`

---

## 四、前端 CoreFusion 主题

### 品牌资产

| 项目 | 值 |
|---|---|
| 品牌名 | 中科超创 / COREFUSION |
| 品牌 Logo | `~/Downloads/logo-export/mark-color.svg`；运行后台已写入 `Logo` 选项，源码默认文件为 `web/default/public/logo.svg` |
| 主色（电光蓝） | `#1F57F8` |
| 辅色（聚变青） | `#13B5AB` |
| 背景（深空藏蓝） | `#0C1830` |
| 字体 | Space Grotesk + Noto Sans SC |

### 修改前端后重新部署

```bash
# 1. 修改源码
cd ~/new-api-web-src/web/default

# 2. 重新构建前端
export PATH="$HOME/.bun/bin:$PATH"
bun run build

# 3. 重新编译 Go 二进制
export PATH="/opt/homebrew/bin:$PATH"
export GOPROXY="https://goproxy.cn,direct"
cd ~/new-api-web-src
GOOS=linux GOARCH=amd64 go build \
  -ldflags "-s -w -X 'github.com/QuantumNous/new-api/common.Version=$(cat VERSION)'" \
  -o /tmp/new-api-corefusion-linux

# 4. 重新打包 Docker 镜像
cp /tmp/new-api-corefusion-linux /tmp/docker-ctx/
docker build -t new-api-corefusion:latest /tmp/docker-ctx/

# 5. 重启主实例
cd ~/new-api && docker-compose up -d new-api
```

### 前端源码关键文件

| 文件 | 用途 |
|---|---|
| `src/styles/theme-presets.css` | CoreFusion 色板定义 |
| `src/lib/theme-customization.ts` | 主题默认值（preset: 'corefusion'） |
| `src/features/home/components/sections/hero.tsx` | 首页 Hero 区文案 |
| `src/features/home/components/sections/features.tsx` | 功能介绍区 |
| `src/features/home/components/sections/stats.tsx` | 数据指标区 |
| `src/components/layout/components/footer.tsx` | 页脚 |

---

## 五、上游渠道管理

### 当前已接入渠道

| 渠道名 | Base URL | 支持模型 |
|---|---|---|
| apix-newapi | https://apix.newapi.pro/v1 | deepseek-v4-pro 等 40+ 模型 |

### 新增渠道

后台 → 渠道 → 添加渠道：
- 类型：OpenAI
- Base URL：填写时**不加 /v1**（如果是 new-api 兼容接口）或**加 /v1**（如果是标准 OpenAI 接口）
- 测试连通后在模型价格里配置该模型的倍率

---

## 六、充值与额度管理（手动阶段）

### 流程

```
分销商转账 → 确认到账 → 主实例增加 Token 额度
```

### 额度换算

`QuotaPerUnit = 10000`，即 **1元 = 10,000 quota**

```bash
# 给 dealer_standard_01 增加 ¥100 额度
docker exec new-api-mysql mysql -uroot -pnewapi123 newapi \
  -e "UPDATE tokens SET remain_quota = remain_quota + 1000000 WHERE name = 'dealer_standard_01';"
```

---

## 七、上线到云服务器（待完成）

**服务器规划：**
- 域名：supchuang.com
- 推荐：阿里云香港轻量 2核4G，Ubuntu 22.04
- 主实例域名：`api.supchuang.com`

**上线步骤：**
1. 购买香港服务器，开放 22/80/443 端口
2. 安装 Docker + Docker Compose
3. 上传 `new-api-corefusion:latest` 镜像（或在服务器重新编译）
4. 上传 `~/new-api/` 目录到服务器
5. 配置 nginx 反代 + Let's Encrypt HTTPS
6. 修改 DNS：`api.supchuang.com` → 服务器 IP
7. 启动服务

---

## 八、常用命令速查

```bash
# 查看所有容器状态
docker ps

# 主实例日志
docker-compose -f ~/new-api/docker-compose.yml logs -f new-api

# 分销商实例日志
docker-compose -f ~/new-api-dealer1/docker-compose.yml logs -f new-api-dealer1

# 备份主实例数据库
docker exec new-api-mysql mysqldump -uroot -pnewapi123 newapi > ~/newapi-backup-$(date +%Y%m%d).sql

# 测试 API 连通
curl http://localhost:3000/api/status
```
