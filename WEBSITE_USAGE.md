# 中科超创 CoreFusion 网站使用说明

## 1. 访问入口

- 官网入口：https://supchuang.com/
- 后台入口：https://supchuang.com/
- API 接口地址：https://supchuang.com/v1
- 管理员账号：以交付信息为准
- 管理员密码：以交付信息为准

首次访问建议使用 Chrome、Edge 或 Safari 最新版本。若刚完成域名解析，浏览器仍显示旧页面，可等待 5-30 分钟，或使用无痕窗口重新访问。

## 2. 管理员登录

1. 打开 https://supchuang.com/
2. 点击登录入口。
3. 输入管理员账号和密码。
4. 登录后进入控制台，可进行用户、令牌、渠道、分组、充值和系统设置管理。

为安全起见，首次交付后建议尽快修改管理员密码，并妥善保存。

## 3. 基础配置说明

当前站点已完成以下基础配置：

- 系统名称：中科超创 CoreFusion
- API 地址：https://supchuang.com/v1
- 额度显示：人民币 CNY
- 额度换算：1 元 = 10000 quota
- 最低充值金额：10 元
- 充值返利比例：0
- 默认主题：default
- 网站 Logo：已替换为品牌 Logo

## 4. 用户管理

路径：后台 -> 用户管理

常用操作：

- 新增用户：为客户创建登录账号。
- 修改用户余额：可手动补充或扣减额度。
- 修改用户分组：根据代理级别分配不同倍率。
- 禁用用户：用于暂停异常账号访问。
- 查看用量：检查用户请求量、消耗额度和调用情况。

建议：普通客户不要分配管理员权限，只分配正常用户权限。

## 5. 分组与倍率

路径：后台 -> 分组管理

当前设计了三档代理分组：

| 分组名 | 说明 | 倍率 |
| --- | --- | --- |
| standard | 标准代理 | 1.4 |
| pro | 专业代理 | 1.25 |
| strategic | 战略代理 | 1.1 |

倍率越低，用户调用同一模型时扣费越低。给用户分组时，应按客户等级选择对应分组。

## 6. 令牌使用

路径：控制台 -> 令牌

用户调用 API 前，需要先创建令牌：

1. 登录用户控制台。
2. 进入令牌页面。
3. 点击创建令牌。
4. 设置令牌名称和额度限制。
5. 保存后复制令牌。

令牌只显示一次，建议用户创建后立即保存。若令牌泄露，应删除旧令牌并重新创建。

## 7. API 调用方式

接口兼容 OpenAI 格式。

Base URL：

```text
https://supchuang.com/v1
```

示例请求：

```bash
curl https://supchuang.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer 用户令牌" \
  -d '{
    "model": "deepseek-v4-pro",
    "messages": [
      {
        "role": "user",
        "content": "你好"
      }
    ]
  }'
```

常用模型可先使用：

```text
deepseek-v4-pro
```

## 8. 充值与额度

路径：控制台 -> 充值

当前额度规则：

- 最低充值金额：10 元
- 货币显示：CNY
- 额度换算：1 元 = 10000 quota
- 示例：充值 10 元 = 100000 quota

如果用户无法充值，请检查：

- 充值金额是否低于 10 元。
- 支付方式是否启用。
- 用户账号是否被禁用。
- 系统支付配置是否完整。

## 9. 渠道管理

路径：后台 -> 渠道管理

渠道用于连接上游模型服务。运营时重点检查：

- 渠道是否启用。
- 渠道密钥是否有效。
- 渠道支持的模型是否包含当前售卖模型。
- 渠道分组是否包含 standard、pro、strategic。
- 测试调用是否返回成功。

如果用户调用 API 报错，优先查看渠道测试结果和系统日志。

## 9.1 分销商接入

分销商接入采用“每个分销商一个独立用户 + 一个独立 Token”的方式。

分销商 OEM 实例填写：

```text
渠道类型：OpenAI 兼容
API Base URL：https://supchuang.com/v1
API Key：主实例分配的分销商专属 Token
模型列表：主实例允许该分销商转售的模型
```

完整流程见：

```text
DISTRIBUTOR_ONBOARDING.md
```

## 10. 价格与模型维护

路径：后台 -> 分组管理 / 渠道管理 / 模型价格配置

新增模型时建议流程：

1. 在渠道中确认上游支持该模型。
2. 在模型价格或倍率配置中添加模型。
3. 给 standard、pro、strategic 三个分组配置对应倍率。
4. 使用测试令牌发起一次 API 调用。
5. 确认返回成功并正确扣费。

## 11. 日常运维

服务器目录：

```text
/opt/corefusion
```

查看服务状态：

```bash
cd /opt/corefusion
docker compose ps
```

查看应用日志：

```bash
docker logs --tail=200 new-api
```

查看 HTTPS / 反代日志：

```bash
docker logs --tail=200 corefusion-caddy
```

重启服务：

```bash
cd /opt/corefusion
docker compose restart
```

## 12. 数据备份

生产环境已提供数据库备份脚本：

```bash
cd /opt/corefusion
./backup-db.sh
```

建议：

- 重大配置修改前先备份。
- 每周至少备份一次数据库。
- 备份文件不要公开上传到 GitHub。
- 备份文件应保存到安全位置。

## 13. 常见问题

### 访问网站仍显示旧页面

原因通常是 DNS 或浏览器缓存。

处理方式：

1. 等待 5-30 分钟。
2. 使用无痕窗口访问。
3. 清理浏览器缓存。
4. 确认 DNS 只保留：

```text
A      @      72.61.114.187
CNAME  www    supchuang.com
```

### HTTPS 证书异常

检查 DNS 是否已正确指向服务器，然后查看 Caddy 日志：

```bash
docker logs --tail=200 corefusion-caddy
```

正常情况下，Caddy 会自动签发和续期 HTTPS 证书。

### API 返回无权限或余额不足

检查：

- 用户令牌是否正确。
- 用户余额是否充足。
- 用户分组是否可用。
- 模型是否对该分组开放。
- 渠道是否启用。

### 模型调用失败

检查：

- 上游渠道密钥是否有效。
- 渠道模型名称是否填写正确。
- 用户调用的模型名是否与后台一致。
- 渠道测试是否成功。

## 14. 安全建议

- 管理员密码定期更换。
- 不要把 `.env`、数据库备份、镜像包上传到公开仓库。
- 用户令牌泄露后应立即删除并重建。
- 只给可信人员管理员权限。
- VPS 只开放必要端口：22、80、443。
- 定期检查系统日志和异常调用。

## 15. 交付状态

当前项目已完成：

- 品牌 Logo 替换
- 首页 UI 重设计
- 后台基础配置
- 额度与充值规则配置
- 分组倍率配置
- API 状态验证
- 生产部署
- HTTPS 证书签发
- GitHub 备份

当前线上访问地址：

```text
https://supchuang.com/
```

## 16. 试运营材料

P1 试运营材料已补齐，存放在：

```text
operations/
```

| 文件 | 用途 |
| --- | --- |
| operations/USER_AGREEMENT.md | 用户服务协议 |
| operations/PRIVACY_POLICY.md | 隐私政策 |
| operations/SUPPORT_AND_REFUND_POLICY.md | 充值、退款与售后规则 |
| operations/CUSTOMER_API_GUIDE.md | 普通客户 API 接入指南 |
| operations/DISTRIBUTOR_HANDOFF_TEMPLATE.md | 分销商交付模板 |
| operations/MANUAL_RECHARGE_SOP.md | 手动充值与额度调整 SOP |
| operations/MODEL_PRICING_TEMPLATE.md | 模型价格表模板 |
| operations/FIRST_BATCH_TESTING_CHECKLIST.md | 首批客户测试清单 |
