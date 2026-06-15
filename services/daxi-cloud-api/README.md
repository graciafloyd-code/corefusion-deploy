# DAXI Cloud API

Standalone first-phase backend for the DAXI Cloud distributor platform.

## Scope

- Public requirement intake.
- Compute inquiry intake.
- Agent, video, and token package requests.
- Admin lead/customer/API key management.
- Customer token balance control and API key listing.
- Persistent admin users, role labels, and audit log.
- Scenario model routes for controlled upstream model access.
- Video task queue for short drama and e-commerce video jobs.
- Payment records for offline transfer and invoice confirmation.
- OpenAI-compatible `/v1/models` and `/v1/chat/completions` proxy.
- Master-platform token backflow headers.

## Local Run

```bash
cd services/daxi-cloud-api
go run ./cmd/daxi-cloud-api
```

Default server:

```text
http://127.0.0.1:8088
```

Default database:

```text
services/daxi-cloud-api/data/daxi-cloud-api.db
```

## Environment

```text
DAXI_API_ADDR=:8088
DAXI_DB_PATH=data/daxi-cloud-api.db
DAXI_ADMIN_TOKEN=dev-admin-token
DAXI_ADMIN_USERNAME=admin
DAXI_ADMIN_PASSWORD=daxi-admin-dev
DAXI_UPSTREAM_BASE_URL=https://supchuang.com/v1
DAXI_UPSTREAM_API_KEY=
DAXI_RESELLER_CODE=daxi-cloud
DAXI_EMERGENCY_DISABLED=false
DAXI_ALLOWED_MODELS=deepseek-v4-flash,deepseek-v4-pro,qwen3.6-plus,gpt-5.4-nano,gpt-5.4-pro,gpt-5.5,gpt-5.5-pro,glm-5.1,gemini-3.5-flash,gemini-3.1-pro-preview,claude-opus-4-7,claude-opus-4-8,minimax-m2.7
DAXI_DEFAULT_PROXY_MODEL=deepseek-v4-flash
DAXI_REQUEST_TIMEOUT_SECONDS=120
DAXI_MAX_BODY_BYTES=1048576
```

`DAXI_REQUEST_TIMEOUT_SECONDS` bounds non-streaming upstream calls. Streaming
calls are not bounded by this timeout (they rely on the request context) so long
responses are not cut off. `DAXI_MAX_BODY_BYTES` caps every request body; the
chat proxy returns `413` when exceeded.

## Public APIs

```text
POST /public/leads
POST /public/compute-inquiries
POST /public/agent-requests
POST /public/token-package-requests
POST /public/video-requests
POST /public/video-tasks
```

## Admin APIs

Admin requests require:

```text
Authorization: Bearer dev-admin-token
```

For prototype console login, create a short-lived admin session with:

```text
POST /admin/sessions
```

Login authenticates against the `admin_users` table only. `DAXI_ADMIN_USERNAME`
/ `DAXI_ADMIN_PASSWORD` are used solely to seed the first owner account when the
table is empty — they are not a login fallback, so a disabled account cannot be
reached through them. A disabled (`status != "Active"`) account cannot sign in.

Endpoints:

```text
GET  /admin/dashboard
GET  /admin/users
POST /admin/users
PATCH /admin/users/{public_id}/status
GET  /admin/audit-logs
GET  /admin/leads
PATCH /admin/leads/{public_id}/status
GET  /admin/compute-inquiries
GET  /admin/customers
POST /admin/customers
GET  /admin/customers/{public_id}
PATCH /admin/customers/{public_id}/balance
GET  /admin/api-keys
POST /admin/api-keys
PATCH /admin/api-keys/{public_id}/status
GET  /admin/usage
GET  /admin/usage-records
GET  /admin/token-plans
GET  /admin/token-orders
POST /admin/token-orders
GET  /admin/recharges
POST /admin/recharges
GET  /admin/model-routes
POST /admin/model-routes
PATCH /admin/model-routes/{public_id}
GET  /admin/video-tasks
POST /admin/video-tasks
PATCH /admin/video-tasks/{public_id}
GET  /admin/payments
POST /admin/payments
PATCH /admin/payments/{public_id}/status
GET  /admin/upstream
```

Customers carry `balance_tokens`. A downstream DAXI customer key can call the model proxy only when the customer is active and has positive token balance. After an upstream response returns usage, the backend records usage and deducts the reported total tokens from the customer balance.

The second-phase console flow supports:

- Token plan selection.
- Pending token order creation.
- Manual recharge ledger with balance update.
- Customer detail summary with keys, usage, and recharge history.
- Recent model proxy usage records for settlement review.
- Scenario model-route table. If a customer request omits `model`, the backend uses that scenario's primary model. Generic `model-api` traffic can call any model returned by the Supchuang upstream `/models` list. Dedicated scenarios such as short drama, e-commerce video, and agent workflows remain constrained by their scenario route. When a route sets `max_tokens_per_request > 0`, requests whose `max_tokens` / `max_completion_tokens` exceed it are rejected with `400` before reaching the upstream.
- Unknown scenarios explicitly fall back to the `model-api` route. Real route database errors return `500`; unknown scenario names no longer bypass route controls.
- Individual API key revocation. `PATCH /admin/api-keys/{public_id}/status` with `{"status":"Disabled"}` blocks that single key (proxy returns `401`) without affecting the customer's other keys.
- Video task queue with status, progress, result URL, and error message fields.
- Payment record ledger for manual payment confirmation. When a linked payment is marked `Paid`, the backend posts the linked token order once through a `payment` recharge record, updates the token order to `Paid`, and returns the payment/recharge pair for audit.
- Database-backed admin users. The first owner account is seeded from `DAXI_ADMIN_USERNAME` and `DAXI_ADMIN_PASSWORD` when the admin table is empty.

## Dedicated Upstream Key

`DAXI_UPSTREAM_API_KEY` is intentionally kept as a deployment secret and is not configured by these pages or APIs. When it is empty, downstream customer API calls return `503 upstream API key is not configured`; public intake, customer management, orders, recharges, video tasks, payments, and admin operations continue to work.

## Model API Proxy

DAXI downstream customers call:

```text
GET  /v1/models
POST /v1/chat/completions
```

Both endpoints require a valid, active DAXI customer API key (`Authorization: Bearer <key>`).

`GET /v1/models` inherits the available model list from the configured Supchuang
upstream (`DAXI_UPSTREAM_BASE_URL`, normally `https://supchuang.com/v1`) using
the dedicated `DAXI_UPSTREAM_API_KEY`. DAXI should not maintain a separate public
model catalogue; when Supchuang opens or removes models for the DAXI upstream
key, DAXI customers see the same list.

For streaming requests (`"stream": true`), the proxy injects
`stream_options.include_usage=true` before forwarding so the upstream emits a
final usage chunk. The proxy relays the SSE stream in real time (flushing each
chunk) and deducts the reported total tokens from the customer balance after the
stream completes. If the upstream does not report usage, no tokens are deducted.

The proxy validates the DAXI customer API key, then forwards to the configured master platform with:

```text
Authorization: Bearer <DAXI_UPSTREAM_API_KEY>
X-Reseller-Code: daxi-cloud
X-Reseller-Customer-ID: <daxi_customer_public_id>
X-Reseller-Key-ID: <daxi_key_public_id>
X-Reseller-Scenario: model-api | short-drama | ecommerce-video | agent
X-Reseller-Request-ID: <request_id>
```

This keeps final model supply, upstream quota, emergency stop, and token settlement under the master platform.

### Token settlement behavior

The first-phase proxy has two settlement layers:

1. DAXI customer balance: DAXI deducts the downstream customer's local
   `balance_tokens` from the usage returned by Supchuang.
2. Supchuang reseller settlement: Supchuang bills the dedicated DAXI upstream
   key according to the reseller group / wholesale multiplier configured on
   Supchuang. DAXI sends `X-Reseller-Code`, customer ID, key ID, scenario, and
   request ID on every upstream call so Supchuang can attribute consumption and
   generate reseller settlement reports.

The DAXI local proxy uses postpaid settlement for customer model calls:

- Requests are allowed only when the DAXI customer balance is greater than zero.
- After the upstream response reports actual usage, the backend deducts the reported `total_tokens`.
- Balance updates are serialized and floored at zero, so concurrent requests cannot make `balance_tokens` negative.
- If one request or concurrent requests consume more than the remaining balance, the excess is recorded in `usage_records.overspend_tokens` for audit and settlement review.

This means the current implementation intentionally accepts single-request overspend risk. A stricter prepaid hold/refund flow can be added later if launch policy requires hard blocking before upstream execution.

For streaming requests the same settlement applies once the final usage chunk is received (see Model API Proxy above); if the upstream reports no usage, no tokens are deducted and no overspend is recorded.

## Frontend Prototype Integration

The static OEM pages call the backend at:

```text
http://127.0.0.1:8088
```

Override it before `app.js` if the backend is deployed elsewhere:

```html
<script>window.DAXI_API_BASE = "https://api.daxicloud.com";</script>
<script src="./app.js"></script>
```

Admin Console uses `dev-admin-token` by default for local prototype testing. Change `DAXI_ADMIN_TOKEN` and `DAXI_ADMIN_PASSWORD` before production delivery; startup logs a warning when development defaults are still present.
