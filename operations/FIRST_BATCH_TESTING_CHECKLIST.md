# 首批客户测试清单

## 1. 测试账号建议

建议先创建：

```text
test_user_001      普通测试用户
dealer_test_001    分销商测试用户
internal_admin     内部管理测试
```

不要使用真实管理员账号直接做客户测试。

## 2. 普通用户测试

- 登录成功
- 能查看余额
- 能创建 Token
- 能调用 `deepseek-v4-pro`
- 调用后余额减少
- 日志能查到请求
- Token 删除后无法继续调用

## 3. 分销商测试

- 分销商用户分组正确
- 分销商 Token 独立
- 分销商额度独立
- 分销商 OEM 渠道能连接主站
- 下游请求能到达主站
- 主站日志归属到分销商用户
- 余额不足时调用失败

## 4. API 测试命令

```bash
curl https://supchuang.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer 测试Token" \
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

## 5. 验收标准

```text
HTTP 状态正常
模型正常返回
余额正确扣减
日志可追踪
错误信息可解释
Token 可禁用
```

