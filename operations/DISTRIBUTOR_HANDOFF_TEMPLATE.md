# 分销商交付模板

以下内容用于给分销商交付接入信息。发送时请将占位内容替换为实际信息，API Key 建议通过私密渠道单独发送。

## 1. 可直接复制给分销商

```text
【CoreFusion API 接入信息】

渠道类型：OpenAI 兼容
API Base URL：https://supchuang.com/v1
API Key：我将通过私密渠道单独发送

可用模型：
{{MODEL_LIST}}

额度上限：
{{TOKEN_QUOTA}} quota

计费倍率：
{{GROUP_NAME}}，倍率 {{GROUP_RATIO}}

结算说明：
你下游用户的每一次 API 调用都会经过你的实例，再回源到 CoreFusion 主站。主站会按模型、Token、分组倍率和实际用量扣减你的额度。
```

## 2. 分销商后台渠道填写

```text
渠道名称：CoreFusion 主站
渠道类型：OpenAI 兼容
Base URL：https://supchuang.com/v1
密钥：填写分配给你的专属 Token
模型列表：{{MODEL_LIST}}
状态：启用
```

## 3. 测试请求

```bash
curl https://supchuang.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer {{API_TOKEN}}" \
  -d '{
    "model": "{{TEST_MODEL}}",
    "messages": [
      {
        "role": "user",
        "content": "测试连接"
      }
    ]
  }'
```

## 4. 验证模型列表

```bash
curl https://supchuang.com/v1/models \
  -H "Authorization: Bearer {{API_TOKEN}}"
```

正常情况下会返回你当前 Token 允许使用的模型列表。

## 5. 对接注意事项

- 请勿把专属 Token 泄露给无关人员。
- 不要多个分销商共用一个 Token。
- 下游用户消耗会计入你的分销商额度。
- 余额不足时，下游用户调用会失败。
- 如需新增模型，请先联系主站确认模型权限和价格。
- 如发现异常消耗，请立即联系主站禁用 Token。

## 6. 分销商需要自行负责

- 下游用户注册与管理
- 下游用户充值和售后
- 下游价格和套餐
- 下游用户内容合规
- 分销商站点品牌与运营

主站负责提供上游 API、额度结算、模型权限和调用日志支持。

## 7. 首批测试账号替换示例

### standard 测试客户

```text
{{MODEL_LIST}} = deepseek-v4-pro,gpt-4o-mini
{{TOKEN_QUOTA}} = 100000
{{GROUP_NAME}} = standard
{{GROUP_RATIO}} = 1.4
{{TEST_MODEL}} = deepseek-v4-pro
```

### pro 测试分销商

```text
{{MODEL_LIST}} = deepseek-v4-pro,gpt-4o-mini,qwen-plus
{{TOKEN_QUOTA}} = 300000
{{GROUP_NAME}} = pro
{{GROUP_RATIO}} = 1.25
{{TEST_MODEL}} = deepseek-v4-pro
```

### strategic 测试分销商

```text
{{MODEL_LIST}} = deepseek-v4-pro,gpt-4o-mini,qwen-plus,claude-sonnet-4
{{TOKEN_QUOTA}} = 500000
{{GROUP_NAME}} = strategic
{{GROUP_RATIO}} = 1.1
{{TEST_MODEL}} = deepseek-v4-pro
```
