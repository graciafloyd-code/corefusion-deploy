# DAXI Cloud 经销商平台首期交接文档

版本：v1.0
交接日期：2026-06-15
交付对象：DAXI Cloud / 大汐科技
交付范围：英文官网、多场景产品页、客户接入页、模型 API 与 Token 接入入口、经销商后台首期运营能力

> 本文档用于交接给经销商运营团队。服务器密码、Admin Token、上游 API Key、客户 API Key 等敏感信息不写入本文档，应通过私密渠道单独发送。

## 1. 平台访问地址

| 类型 | 地址 | 说明 |
| --- | --- | --- |
| 官网首页 | `https://daxicloud.com/` | DAXI Cloud 对外官网首页 |
| 客户接入页 | `https://daxicloud.com/get-started.html` | 客户提交接入需求 |
| 模型 API 页 | `https://daxicloud.com/model-api.html` | 模型 API 与 Token 套餐入口 |
| 短剧生成页 | `https://daxicloud.com/short-drama.html` | 短剧生成场景页 |
| 电商短视频页 | `https://daxicloud.com/ecommerce-video.html` | 电商短视频生成场景页 |
| AI 算力页 | `https://daxicloud.com/ai-compute.html` | GPU / 私有部署 / 算力需求入口 |
| Agent 方案页 | `https://daxicloud.com/agent-solutions.html` | 企业 / 校园 Agent 场景页 |
| API 文档页 | `https://daxicloud.com/docs.html` | API 接入说明 |
| 经销商后台 | `https://daxicloud.com/admin-console.html` | DAXI 运营人员处理线索与运营数据 |
| 健康检查 | `https://daxicloud.com/healthz` | 服务存活检查 |

## 2. 首期交付能力

### 2.1 对外官网

首期已交付英文官网及多场景产品页，覆盖：

- DAXI Cloud 品牌首页。
- 模型 API 与 Token 接入入口。
- Short Drama Generation 场景页。
- E-commerce Short Video Generation 场景页。
- AI Compute 需求入口。
- Agent Solutions 场景页。
- Docs / API access guide。

客户可通过官网提交业务需求，提交后进入经销商后台的 Leads CRM。

### 2.2 客户接入流程

首期采用“客户注册 / 提交需求 + 人工开通”的模式：

1. 客户访问官网或 Get Started 页面。
2. 客户提交公司、联系方式、目标场景、预估用量等信息。
3. 线索自动进入经销商后台 Leads CRM。
4. DAXI 运营人员跟进客户，记录 Follow-up note。
5. 确认需求后，将客户信息交给平台运营人工创建客户、API Key、Token 套餐和充值记录。
6. 平台运营完成开通后，再将 API 接入信息通过私密渠道交付给客户。

首期不开放客户自助控制台，不支持客户自行创建 API Key 或自助充值。

### 2.3 经销商后台

后台首期重点是线索运营，而不是完整客户自助系统。

已上线能力：

- Dashboard 运营概览。
- Leads CRM 线索列表。
- 线索搜索和状态筛选。
- 线索状态流：`New -> Contacted -> Quoting -> PoC -> Won -> Closed`。
- 线索详情面板。
- Follow-up note / Next step 跟进记录。
- 跟进记录刷新后持久化。
- 当前筛选结果 CSV 导出。
- 基础客户、API Key、Token、用量、支付、模型路由等运营数据模块。

## 3. 后台使用说明

### 3.1 登录后台

后台地址：

```text
https://daxicloud.com/admin-console.html
```

登录方式：

- 使用交付时单独发送的后台账号和临时密码登录。
- 首次交付后，建议 DAXI 运营负责人立即修改临时密码。
- Admin Token 属于高级连接方式，只应由技术运维人员保管，不建议普通运营人员使用。

### 3.2 处理客户线索

进入后台后，打开 `Leads` 模块。

推荐操作流程：

1. 查看新线索。
2. 用搜索框按公司、联系人、邮箱、场景关键词筛选。
3. 用状态筛选查看当前跟进阶段。
4. 点击 `Details` 查看线索详情。
5. 在 `Follow-up note` 中记录沟通纪要。
6. 在 `Next step` 中填写下一步动作，例如：
   - Send pricing
   - Schedule demo
   - Prepare API trial
   - Wait for platform activation
7. 根据实际进展更新状态。

状态含义建议：

| 状态 | 含义 |
| --- | --- |
| New | 新提交，尚未联系 |
| Contacted | 已联系客户，正在确认需求 |
| Quoting | 已进入报价或套餐沟通 |
| PoC | 进入测试、试用或方案验证 |
| Won | 客户已成交或已开通 |
| Closed | 无效、流失或暂停跟进 |

### 3.3 导出线索

后台支持导出当前筛选结果：

1. 在 Leads 模块中先搜索或选择状态。
2. 点击 `Export CSV`。
3. 浏览器会下载 `daxi-leads-YYYY-MM-DD.csv`。
4. 可将 CSV 提交给平台运营，用于人工创建客户、API Key、Token 套餐和充值记录。

导出字段包括：

- Lead ID
- Status
- Scenario
- Company
- Country
- Contact Name
- Email
- Phone
- Usage Profile
- Budget
- Source
- Notes
- Created At

## 4. API 与 Token 开通边界

### 4.1 首期客户开通方式

首期不让 DAXI 下游客户直接自助开通 API Key。

正确流程：

1. DAXI 后台收集客户线索。
2. DAXI 运营确认客户需求、套餐和用量。
3. DAXI 将线索导出或整理后提交给平台运营。
4. 平台运营创建客户记录。
5. 平台运营创建客户 API Key。
6. 平台运营设置 Token 套餐、额度、模型权限和充值记录。
7. 平台运营将 API Base URL、API Key、模型列表和测试命令交付给 DAXI 或终端客户。

### 4.2 API 调用地址

对下游客户交付时使用 DAXI Cloud 域名：

```text
API Base URL: https://daxicloud.com/v1
Models API:   https://daxicloud.com/v1/models
Chat API:     https://daxicloud.com/v1/chat/completions
```

示例请求：

```bash
curl https://daxicloud.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <CUSTOMER_API_KEY>" \
  -d '{
    "model": "deepseek-v4-flash",
    "messages": [
      {
        "role": "user",
        "content": "Hello, DAXI Cloud"
      }
    ]
  }'
```

模型列表请求：

```bash
curl https://daxicloud.com/v1/models \
  -H "Authorization: Bearer <CUSTOMER_API_KEY>"
```

注意：`<CUSTOMER_API_KEY>` 由平台运营人工创建并私密发送，不应出现在公开文档、网页源码、截图或群聊中。

## 5. Token 回流与主平台控制原则

DAXI Cloud 是经销商运营层，不直接持有真实上游模型供应商密钥。

调用链路：

```text
DAXI 下游客户
  -> DAXI Cloud customer API key
  -> DAXI Cloud validation / usage / balance
  -> Master platform upstream key
  -> Master platform final token metering
  -> Upstream model provider
```

设计原则：

- DAXI 可以拥有品牌、域名、客户线索、客户运营和本地套餐。
- 下游客户调用必须先经过 DAXI Cloud。
- DAXI Cloud 再通过专属上游 Key 回源到主平台。
- 最终模型消耗、上游额度、风控和停用权保留在主平台。
- 若 DAXI 上游 Key 余额不足、被停用或超出权限，下游客户调用会停止。

这样可以确保 DAXI 发展的客户越多，最终 Token 消耗越自然回流到主平台。

## 6. 安全与权限要求

### 6.1 DAXI 运营侧要求

- 后台账号只分配给必要运营人员。
- 临时密码交付后应立即修改。
- 不要在公开群、截图、工单或文档中暴露完整 API Key。
- 每个客户、每个项目建议使用独立 API Key。
- 客户离职、项目结束或出现异常消耗时，应及时联系平台运营停用 Key。
- 不要让下游客户绕过 DAXI Cloud 直接调用主平台。

### 6.2 平台运营侧保留权限

平台运营保留以下权限：

- DAXI 上游 API Key 管理。
- DAXI 总额度、模型权限和风控策略。
- 紧急停用 DAXI 上游访问。
- 审计 DAXI 总调用量、Token 消耗和异常请求。
- 协助 DAXI 创建下游客户、API Key、套餐和充值记录。

## 7. 首期不包含范围

以下能力不在首期交付范围内：

- 下游客户自助注册后直接进入 Customer Console。
- 客户自助创建 API Key。
- 客户在线支付和自动充值。
- 自动开票、自动结算。
- 完整客户用量账单下载。
- 视频生成真实模型深度集成。
- Agent 模板后台完整管理。
- 多级经销商结算系统。

这些能力可作为 Phase 2 / Phase 3 继续规划。

## 8. 运维信息

当前生产服务器：

```text
Server IP: 101.47.76.60
OS: CentOS 7.9
Region: Hong Kong
Service: daxi-cloud-api
Frontend root: /opt/daxi-cloud/daxi-cloud-en
Backend binary: /opt/daxi-cloud/daxi-cloud-api
Database: /opt/daxi-cloud/data/daxi-cloud-api.db
```

服务检查：

```bash
curl https://daxicloud.com/healthz
```

正常返回：

```json
{"ok":true,"service":"daxi-cloud-api"}
```

生产注意事项：

- CentOS 7.9 已偏旧，不建议长期继续扩展生产能力。
- 不建议原地升级系统。
- 建议后续迁移到 Ubuntu 24.04 LTS 或 Debian 12 新服务器。
- 迁移前应备份 `/opt/daxi-cloud`、数据库、Nginx 配置和 SSL 证书。

## 9. 本次上线验收结果

验收日期：2026-06-15

已验证：

- 官网健康检查正常。
- 后台静态文件已更新。
- Leads CRM 页面包含状态流、详情面板和 CSV 导出入口。
- `/admin/leads` 可读取线上线索。
- `/admin/leads/{id}/activities` 可新增和读取跟进记录。
- Follow-up note 刷新后仍存在。
- CSV 导出按钮和前端下载逻辑已上线。

CSV 下载说明：

- 真实浏览器点击 `Export CSV` 会生成 `daxi-leads-YYYY-MM-DD.csv`。
- 自动化验收环境不支持浏览器下载事件，因此下载动作需由人工在 Chrome / Safari 中补充确认。

## 10. 交接清单

交接给 DAXI 前请确认：

- [ ] 后台登录账号已单独发送。
- [ ] 临时密码已通过私密渠道发送。
- [ ] DAXI 运营负责人知道首次登录后应修改密码。
- [ ] DAXI 运营人员知道如何处理 Leads。
- [ ] DAXI 运营人员知道如何记录 Follow-up note。
- [ ] DAXI 运营人员知道如何导出 CSV。
- [ ] DAXI 明确首期不开放客户自助控制台。
- [ ] DAXI 明确 API Key 和充值由平台运营人工处理。
- [ ] 上游 API Key、Admin Token、服务器 root 密码未写入公开文档。

## 11. 建议发给 DAXI 的简短说明

```text
DAXI Cloud 首期平台已上线。

官网地址：
https://daxicloud.com/

经销商后台：
https://daxicloud.com/admin-console.html

首期采用“客户提交需求 + DAXI 运营跟进 + 平台运营人工开通 API/Token”的模式。
客户通过官网提交需求后，会进入后台 Leads CRM。运营人员可查看详情、更新状态、记录跟进内容，并导出 CSV 给平台运营进行客户开通。

首期暂不开放客户自助控制台，不支持客户自行创建 API Key 或自助充值。
后台账号和临时密码将通过私密渠道单独发送。
```
