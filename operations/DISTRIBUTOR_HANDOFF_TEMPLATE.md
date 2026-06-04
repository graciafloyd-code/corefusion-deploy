# 分销商交付模板

以下内容用于给分销商交付接入信息。发送时请将占位内容替换为实际信息，API Key 建议通过私密渠道单独发送。

## 1. 基础接入信息

```text
渠道类型：OpenAI 兼容
API Base URL：https://supchuang.com/v1
API Key：单独发送
可用模型：deepseek-v4-pro
额度上限：以双方约定为准
计费倍率：以双方约定为准
```

## 2. 分销商后台渠道填写

```text
渠道名称：CoreFusion 主站
渠道类型：OpenAI 兼容
Base URL：https://supchuang.com/v1
密钥：填写分配给你的专属 Token
模型列表：deepseek-v4-pro
状态：启用
```

## 3. 测试请求

```bash
curl https://supchuang.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer 分销商专属Token" \
  -d '{
    "model": "deepseek-v4-pro",
    "messages": [
      {
        "role": "user",
        "content": "测试连接"
      }
    ]
  }'
```

## 4. 对接注意事项

- 请勿把专属 Token 泄露给无关人员。
- 不要多个分销商共用一个 Token。
- 下游用户消耗会计入你的分销商额度。
- 余额不足时，下游用户调用会失败。
- 如需新增模型，请先联系主站确认模型权限和价格。
- 如发现异常消耗，请立即联系主站禁用 Token。

## 5. 分销商需要自行负责

- 下游用户注册与管理
- 下游用户充值和售后
- 下游价格和套餐
- 下游用户内容合规
- 分销商站点品牌与运营

主站负责提供上游 API、额度结算、模型权限和调用日志支持。

