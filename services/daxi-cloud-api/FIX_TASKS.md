# daxi-cloud-api 修复任务清单（状态版）

仓库：services/daxi-cloud-api
约束：保持现有分层（cmd / internal{config,db,httpapi,model,upstream}）；
不引入重型框架，沿用标准库 + go-sqlite3；改完 `go build ./...`、`go vet ./...`、
`go test ./...` 必须通过。

状态图例：✅ 已完成（Claude）  ⬜ 待办（Codex）

---

## P0 —— 上线前必须修复（涉及资金与鉴权）

### ✅ P0-1 流式请求计费
方案 A 已落地。
- proxy.go：`stream:true` 时强制注入 `stream_options.include_usage=true` 再转发；
  `streamAndCapture` 边转发边 `Flush()`（实时流），扫描最后的 usage chunk 完成扣费。
- upstream/client.go：`ProxyChat(ctx, body, identity, stream)` 不再预缓冲；
  新增 `ParseUsage`（非流式）/`ParseStreamUsage`（SSE）；流式走独立 `streamClient`
  （Timeout=0，避免长流被 120s 砍断），非流式仍受超时约束。
- 测试：TestProxyNonStreamingDeductsBalance / TestProxyStreamingDeductsBalanceAndInjectsUsage
  / TestProxyStreamingNoFreeRideWhenUpstreamOmitsUsage（均通过）。

### ✅ P0-2 后台账号禁用可被环境变量凭证绕过
- server.go handleCreateAdminSession：移除 env 凭证登录回退，登录一律走
  store.AuthenticateAdmin（内含 status=="Active" 校验）；env 仅用于 EnsureAdminUser 种子。
- admin_token 与 admin 中间件改用 crypto/subtle.ConstantTimeCompare。
- 已手测：DB 种子登录可用、错密码 401、禁用账号无法登录。

### ✅ P0-3 后付费 + 并发无锁导致超卖
- store.go：DecreaseCustomerBalance 改为 `BEGIN IMMEDIATE` 原子事务，返回超额 token；
  余额 floor 到 0，超额量记入 usage_records.overspend_tokens。
- model.go：UsageRecord / UsageSummary 增加 OverspendTokens 字段。
- store.go：ensureColumn 为存量库安全补列（SQLite ALTER TABLE ADD COLUMN）。
- README：补充「postpaid settlement / 单请求级超额风险」说明。
- 与流式计费(P0-1)已合并：流式扣费后同样记录 overspend。
- 测试：TestStreamingFloorsBalanceAndRecordsOverspend 等通过。
说明：当前为后付费 + 记账模式（非预扣）。如上线策略要求硬阻断，再加预扣(hold)流程。

---

## P1 —— 重要功能/安全缺口

### ✅ P1-1 单个 API Key 吊销
- db/store.go：新增 UpdateAPIKeyStatus(publicID, status)。
- server.go：新增 PATCH /admin/api-keys/{public_id}/status（走 admin 中间件）+ 审计日志。
- 测试：TestAPIKeyRevocationBlocksProxy（通过）。

### ✅ P1-2 model_routes.max_tokens_per_request 生效
- proxy.go：路由 MaxTokensPerRequest>0 时，校验请求 max_tokens / max_completion_tokens，
  超限返回 400（新增 payloadInt 辅助函数）。
- 测试：TestMaxTokensPerScenarioEnforced（通过）。

### ✅ P1-3 请求体大小限制
- 新增 DAXI_MAX_BODY_BYTES（默认 1MB）+ limitBody 中间件包裹所有请求。
- 代理路径超限 413；JSON 接口经 badRequest 辅助函数统一映射 413。
- 已手测：300B body 超 200B 限额返回 413，正常 body 201。

### ✅ P1-4 /v1/models 鉴权
- handleModels 校验 bearer key + 客户/key 状态，无效返回 401。
- 已手测：无 key 401、有效 key 200。

### ✅ P1-5 未知 scenario 静默绕过路由管控
现状：GetModelRouteByScenario 出错时跳过 disabled 检查，落到全局 allowlist。
要求：明确策略——选 (b) 显式回退到 "model-api" 路由并注释说明；
区分 sql.ErrNoRows（回退）与真实 DB 错误（返回 500）。
验收：传不存在的 scenario 时按既定策略处理，不再静默使用 DefaultProxyModel。
- proxy.go：未知 scenario 显式回退到 `model-api`，真实 DB 错误返回 500。
- 测试：TestUnknownScenarioFallsBackToModelAPIRoute 覆盖转发 header。

---

## P2 —— 收尾/健壮性（全部待 Codex）

- ✅ P2-1 配置超时落地：cfg.RequestTimeoutSecs 改 int 秒并用于 http.Client.Timeout。
  说明：upstream/client.go 已部分使用该值构造非流式 client.Timeout（解析 string），
  仍建议把类型彻底改为 int 并清理。
- ✅ P2-2 main.go 改用 &http.Server{ReadHeaderTimeout, ReadTimeout, WriteTimeout, IdleTimeout}，
  不要裸用 http.ListenAndServe（流式响应的 WriteTimeout 需酌情放宽）。
- ✅ P2-3 常量时间比较：store.go 密码/keyhash 比对改用 crypto/subtle.ConstantTimeCompare。
  说明：admin_token 与 admin 中间件已在 P0-2 改好；剩 AuthenticateAdmin 的 hash 比较。
- ✅ P2-4 newPublicID 抗碰撞：随机字节 5→8，并在 Create* 遇 UNIQUE 冲突时重试一次；
  统一前缀避免 DXP/DXR 复用（payment 用 DXY，recharge 保持 DXR，上游 requestID 前缀改 req-）。
- ✅ P2-5 status 收敛：对 customer/apikey/payment/route 的 status 做大小写规范化，
  proxy 比较保持一致，避免 "active" 锁死。
- ✅ P2-6 启动告警：cfg.AdminToken=="dev-admin-token" 或使用默认 AdminPassword 时打 WARN 日志。
- ✅ P2-7 adminSessions 过期清理：后台 goroutine 定期清理过期 token（可选）。

---

## 提交要求
- 每个任务单独 commit，message 英文前缀 + 中文描述，例如 `fix: 并发原子扣减余额 (P0-3)`。
- 改完跑：go build ./... && go vet ./... && go test ./...
- 同步更新 README.md 受影响章节。
- 新增 DB 字段：迁移须对空库和存量库都安全（SQLite ALTER TABLE ADD COLUMN）。

## 进度汇总
已完成（Claude）：P0-1, P0-2, P1-1, P1-2, P1-3, P1-4
已完成（Codex）：P0-3, P1-5, P2-1 ~ P2-7

全部任务完成。已统一合并（Claude 的 P0-1~P1-4 + Codex 的 P0-3/P1-5/P2），
build/vet/test 全通过（8 个测试）。
