# CoreFusion 客户 API 接入指南

## 1. 接入信息

Base URL：

```text
https://supchuang.com/v1
```

接口格式：

```text
OpenAI 兼容
```

常用模型：

```text
deepseek-v4-pro
```

## 2. 创建 API Token

1. 登录 https://supchuang.com/
2. 进入控制台。
3. 打开令牌页面。
4. 点击创建令牌。
5. 设置令牌名称和额度限制。
6. 保存后复制 Token。

Token 只显示一次，请妥善保存。Token 泄露后应立即删除并重新创建。

## 3. curl 示例

```bash
curl https://supchuang.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer 你的API Token" \
  -d '{
    "model": "deepseek-v4-pro",
    "messages": [
      {
        "role": "user",
        "content": "你好，请介绍一下你自己"
      }
    ]
  }'
```

## 4. Python 示例

```python
from openai import OpenAI

client = OpenAI(
    base_url="https://supchuang.com/v1",
    api_key="你的API Token",
)

response = client.chat.completions.create(
    model="deepseek-v4-pro",
    messages=[
        {"role": "user", "content": "你好，请介绍一下你自己"}
    ],
)

print(response.choices[0].message.content)
```

## 5. Node.js 示例

```javascript
import OpenAI from "openai";

const client = new OpenAI({
  baseURL: "https://supchuang.com/v1",
  apiKey: "你的API Token",
});

const response = await client.chat.completions.create({
  model: "deepseek-v4-pro",
  messages: [
    { role: "user", content: "你好，请介绍一下你自己" },
  ],
});

console.log(response.choices[0].message.content);
```

## 6. Cherry Studio 配置

```text
供应商类型：OpenAI 兼容
API 地址：https://supchuang.com/v1
API Key：你的 API Token
模型：deepseek-v4-pro
```

## 7. 常见错误

### 401 Unauthorized

可能原因：

- Token 填写错误
- Token 已删除或禁用
- 请求头没有带 `Authorization: Bearer`

### 余额不足

可能原因：

- 账号额度已用完
- Token 设置了额度上限
- 分销商额度不足

### 模型不存在或无权限

可能原因：

- 模型名称填写错误
- 当前账号或分组没有该模型权限
- 平台暂未开放该模型

### 请求超时

可能原因：

- 上游模型响应较慢
- 网络链路异常
- 请求内容过长

可稍后重试，或联系管理员排查。

## 8. 安全建议

- 不要把 Token 写在前端网页或公开仓库中。
- 不要在群聊、截图、文档中暴露完整 Token。
- 每个项目使用独立 Token。
- 发现泄露后立即删除旧 Token。
- 为 Token 设置额度上限，降低异常消耗风险。

