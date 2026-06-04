# Token 中转站功能对标与完善建议

## 1. 调研说明

用户提供的站点：

```text
https://itokengo.com/
```

本地直接访问该域名时 TLS 连接失败，无法完整抓取页面内容。因此本次对标参考了公开搜索索引中可见的同类 Token 中转站页面能力，包括 TokenGO、Gotoken、Token API 等公开描述。

参考站点普遍强调：

- 一个 API Key 接入多模型
- OpenAI SDK 兼容
- 模型广场和公开价格
- 快速接入示例
- 实时用量和费用统计
- Key 级预算控制
- 分销商 / 企业定制
- 高可用、智能路由、速率限制
- 安全能力，例如密钥隔离、IP 白名单、审计日志
- 支付充值和余额永不过期

## 2. 我们主站已有能力

当前 CoreFusion 主站已经具备：

| 功能 | 状态 |
| --- | --- |
| OpenAI 兼容接口 | 已具备 |
| 统一 Base URL | 已具备，`https://supchuang.com/v1` |
| 用户管理 | 已具备 |
| Token / 令牌管理 | 已具备 |
| 额度计费 | 已具备 |
| CNY 额度显示 | 已配置 |
| 最低充值金额 | 已配置，10 元 |
| 分组倍率 | 已配置，standard / pro / strategic |
| 分销商接入 | 已设计并形成手册 |
| 渠道管理 | 已具备 |
| 模型管理 | 已具备 |
| 请求日志 | 已具备 |
| HTTPS | 已部署并签发证书 |
| GitHub 备份 | 已完成 |

## 3. 与同类站点相比的主要差距

| 对标能力 | 当前情况 | 建议 |
| --- | --- | --- |
| 首页模型价格展示 | 原首页表达较少 | 已补模型展示区 |
| 快速接入代码示例 | 原首页不够突出 | 已补 Python SDK Quickstart |
| 企业 / 分销商卖点 | 原首页表达较少 | 已补企业和 OEM 能力区 |
| 支付自动充值 | 当前建议先手动充值 | 后续接支付宝 / 微信 / USDT |
| Key 级速率限制 | 需确认后台能力 | 建议作为下一阶段重点 |
| IP 白名单 | 需确认后台能力 | 企业客户需要时再做 |
| 用量告警 | 需新增或配置 | 建议支持余额阈值提醒 |
| SLA / 状态页 | 暂无独立状态页 | 后续可接 uptime 监控页 |
| 文档中心 | 目前有 Markdown 手册 | 后续可做公开 Docs 页面 |
| 自动对账单 | 当前可查日志 | 后续做分销商月账单导出 |

## 4. 本次已完善内容

本次已在前端首页补充：

- 模型展示区：展示模型、厂商、输入输出能力、上下文窗口、定位标签。
- 开发者快速接入区：展示 Python SDK 示例，突出只需替换 Base URL 和 Key。
- 企业与分销商能力区：展示额度上限、倍率分层、密钥隔离、审计日志、OEM 托管。
- 修正首页示例 Base URL 为 `https://supchuang.com/v1`。

涉及源码：

```text
/Users/wuquan/new-api-web-src/web/default/src/features/home/components/sections/model-showcase.tsx
/Users/wuquan/new-api-web-src/web/default/src/features/home/components/sections/developer-quickstart.tsx
/Users/wuquan/new-api-web-src/web/default/src/features/home/components/sections/enterprise.tsx
/Users/wuquan/new-api-web-src/web/default/src/features/home/index.tsx
/Users/wuquan/new-api-web-src/web/default/src/features/home/components/index.ts
/Users/wuquan/new-api-web-src/web/default/src/features/home/components/sections/hero.tsx
```

## 5. 下一阶段建议

### 第一优先级

1. 开放或完善公开模型价格页。
2. 给每个 Token 增加额度上限和到量提醒。
3. 建立分销商月度对账表。
4. 主站保留手动充值，先不急于全自动支付。

### 第二优先级

1. 接入支付宝 / 微信 / USDT 充值。
2. 增加 IP 白名单。
3. 增加 Key 级速率限制。
4. 增加余额不足提醒。
5. 增加公开 API 文档页面。

### 第三优先级

1. 状态页和 SLA 展示。
2. 模型延迟 / 可用率公开展示。
3. 企业合同价 / 阶梯折扣。
4. 分销商独立账单下载。
5. 私有化部署套餐。

