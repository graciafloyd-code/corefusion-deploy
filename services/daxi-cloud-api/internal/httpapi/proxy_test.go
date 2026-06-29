package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"daxi-cloud-api/internal/config"
	"daxi-cloud-api/internal/db"
	"daxi-cloud-api/internal/model"
	"daxi-cloud-api/internal/upstream"
)

// newTestServer wires a Server against a temp SQLite store and a mock upstream
// that records the last request body it received.
func newTestServer(t *testing.T, transport http.RoundTripper) (*Server, *db.Store) {
	t.Helper()
	store, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	if err := store.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cfg := config.Config{
		UpstreamBaseURL:   "https://upstream.test/v1",
		UpstreamAPIKey:    "upstream-secret",
		ResellerCode:      "daxi-cloud",
		AllowedModels:     []string{"deepseek-v4-flash"},
		DefaultProxyModel: "deepseek-v4-flash",
		MaxBodyBytes:      1 << 20,
		AdminToken:        "test-admin",
	}
	wrappedTransport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/models") {
			return modelListResponse(cfg.AllowedModels...), nil
		}
		return transport.RoundTrip(r)
	})
	return NewServer(cfg, store, upstream.NewClientWithTransport(cfg, wrappedTransport)), store
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func mockResponse(contentType, body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func modelListResponse(models ...string) *http.Response {
	items := make([]string, 0, len(models))
	for _, model := range models {
		items = append(items, `{"id":"`+model+`","object":"model","owned_by":"supchuang"}`)
	}
	return mockResponse("application/json", `{"object":"list","data":[`+strings.Join(items, ",")+`]}`)
}

func jsonUsageTransport(t *testing.T) http.RoundTripper {
	t.Helper()
	return roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return mockResponse("application/json", `{"choices":[],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`), nil
	})
}

func seedCustomerKey(t *testing.T, store *db.Store, balance int64) string {
	t.Helper()
	c := &model.Customer{Company: "ACME", Balance: balance}
	if err := store.CreateCustomer(c); err != nil {
		t.Fatalf("create customer: %v", err)
	}
	_, raw, err := store.CreateAPIKey(c.PublicID, "k1", "")
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	return raw
}

func currentBalance(t *testing.T, store *db.Store, rawKey string) int64 {
	t.Helper()
	_, customer, err := store.FindAPIKey(rawKey)
	if err != nil {
		t.Fatalf("find key: %v", err)
	}
	return customer.Balance
}

func TestProxyNonStreamingDeductsBalance(t *testing.T) {
	srv, store := newTestServer(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return mockResponse("application/json", `{"choices":[{"message":{"content":"hi"}}],"usage":{"prompt_tokens":5,"completion_tokens":7,"total_tokens":12}}`), nil
	}))
	raw := seedCustomerKey(t, store, 1000)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"deepseek-v4-flash","messages":[]}`))
	req.Header.Set("Authorization", "Bearer "+raw)
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	bal := currentBalance(t, store, raw)
	if bal != 1000-12 {
		t.Fatalf("balance = %d, want %d", bal, 1000-12)
	}
}

// chat 计费对齐 quota:上游返回 usage.quota 时,DAXI 按 quota 扣(不是 total_tokens),且 usage_records 记 quota 单位。
func TestChatBillsByUpstreamQuotaWhenPresent(t *testing.T) {
	srv, store := newTestServer(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return mockResponse("application/json", `{"choices":[{"message":{"content":"hi"}}],"usage":{"prompt_tokens":8,"completion_tokens":141,"total_tokens":149,"quota":11963}}`), nil
	}))
	raw := seedCustomerKey(t, store, 20000)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"deepseek-v4-flash","messages":[]}`))
	req.Header.Set("Authorization", "Bearer "+raw)
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if bal := currentBalance(t, store, raw); bal != 20000-11963 {
		t.Fatalf("balance=%d want %d(应按 quota=11963 扣,不是 total_tokens=149)", bal, 20000-11963)
	}
	var total int64
	if err := store.DB().QueryRow(`SELECT total_tokens FROM usage_records ORDER BY id DESC LIMIT 1`).Scan(&total); err != nil {
		t.Fatalf("read usage: %v", err)
	}
	if total != 11963 {
		t.Fatalf("usage_records.total_tokens=%d want 11963(记 quota 单位)", total)
	}
}

// quota:0 显式存在 → 按 0 扣、不回退(关键:区分 quota:0 与无 quota 字段)。
func TestChatQuotaZeroExplicitChargesZeroNoFallback(t *testing.T) {
	srv, store := newTestServer(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return mockResponse("application/json", `{"choices":[{"message":{"content":"hi"}}],"usage":{"prompt_tokens":5,"completion_tokens":10,"total_tokens":15,"quota":0}}`), nil
	}))
	raw := seedCustomerKey(t, store, 1000)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"deepseek-v4-flash","messages":[]}`))
	req.Header.Set("Authorization", "Bearer "+raw)
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	// quota=0 存在 → 扣 0(余额不变),不得回退按 total_tokens=15 扣
	if bal := currentBalance(t, store, raw); bal != 1000 {
		t.Fatalf("balance=%d want 1000(quota:0 应扣 0,不回退扣 15)", bal)
	}
	var total int64
	if err := store.DB().QueryRow(`SELECT total_tokens FROM usage_records ORDER BY id DESC LIMIT 1`).Scan(&total); err != nil {
		t.Fatalf("read usage: %v", err)
	}
	if total != 0 {
		t.Fatalf("usage_records.total_tokens=%d want 0(记 quota 单位 0)", total)
	}
}

// 无 quota 字段 → 回退按 token 扣(防御式,上游未覆盖时)。
func TestChatFallsBackWhenQuotaAbsent(t *testing.T) {
	srv, store := newTestServer(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return mockResponse("application/json", `{"choices":[{"message":{"content":"hi"}}],"usage":{"prompt_tokens":5,"completion_tokens":10,"total_tokens":15}}`), nil
	}))
	raw := seedCustomerKey(t, store, 1000)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"deepseek-v4-flash","messages":[]}`))
	req.Header.Set("Authorization", "Bearer "+raw)
	srv.Router().ServeHTTP(rec, req)
	if bal := currentBalance(t, store, raw); bal != 1000-15 {
		t.Fatalf("balance=%d want %d(无 quota 字段应回退扣 total_tokens=15)", bal, 1000-15)
	}
}

func TestModelsInheritFromUpstreamModelList(t *testing.T) {
	cfg := config.Config{
		UpstreamBaseURL:   "https://upstream.test/v1",
		UpstreamAPIKey:    "upstream-secret",
		ResellerCode:      "daxi-cloud",
		AllowedModels:     []string{"legacy-static-model"},
		DefaultProxyModel: "supchuang-live-model",
		MaxBodyBytes:      1 << 20,
		AdminToken:        "test-admin",
	}
	store, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	if err := store.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := store.UpsertModelRoute(&model.ModelRoute{Scenario: "model-api", PrimaryModel: "supchuang-live-model", Status: "Active"}); err != nil {
		t.Fatalf("upsert model route: %v", err)
	}
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/models") {
			return modelListResponse("supchuang-live-model", "another-upstream-model"), nil
		}
		return mockResponse("application/json", `{"choices":[],"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}}`), nil
	})
	srv := NewServer(cfg, store, upstream.NewClientWithTransport(cfg, transport))
	raw := seedCustomerKey(t, store, 1000)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("models status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "supchuang-live-model") || strings.Contains(rec.Body.String(), "legacy-static-model") {
		t.Fatalf("models response did not inherit upstream list: %s", rec.Body.String())
	}

	for _, modelName := range []string{"supchuang-live-model", "another-upstream-model"} {
		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"`+modelName+`","messages":[]}`))
		req.Header.Set("Authorization", "Bearer "+raw)
		srv.Router().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("chat with upstream model %q status = %d body=%s", modelName, rec.Code, rec.Body.String())
		}
	}
}

func TestProxyStreamingDeductsBalanceAndInjectsUsage(t *testing.T) {
	var gotBody string
	srv, store := newTestServer(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		return mockResponse("text/event-stream", strings.Join([]string{
			"data: {\"choices\":[{\"delta\":{\"content\":\"he\"}}]}",
			"",
			"data: {\"choices\":[{\"delta\":{\"content\":\"llo\"}}]}",
			"",
			"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":20,\"total_tokens\":30}}",
			"",
			"data: [DONE]",
			"",
		}, "\n")), nil
	}))
	raw := seedCustomerKey(t, store, 1000)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"deepseek-v4-flash","stream":true,"messages":[]}`))
	req.Header.Set("Authorization", "Bearer "+raw)
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	// 1. stream_options.include_usage must have been injected before forwarding.
	var sent map[string]any
	if err := json.Unmarshal([]byte(gotBody), &sent); err != nil {
		t.Fatalf("upstream body not json: %v (%s)", err, gotBody)
	}
	opts, _ := sent["stream_options"].(map[string]any)
	if opts == nil || opts["include_usage"] != true {
		t.Fatalf("include_usage not injected: %s", gotBody)
	}
	// 2. client received the streamed chunks verbatim.
	if !strings.Contains(rec.Body.String(), "hello"[:2]) || !strings.Contains(rec.Body.String(), "[DONE]") {
		t.Fatalf("stream not forwarded: %s", rec.Body.String())
	}
	// 3. balance deducted by the usage reported in the final SSE chunk.
	bal := currentBalance(t, store, raw)
	if bal != 1000-30 {
		t.Fatalf("balance = %d, want %d", bal, 1000-30)
	}
}

func TestAPIKeyRevocationBlocksProxy(t *testing.T) {
	srv, store := newTestServer(t, jsonUsageTransport(t))

	c := &model.Customer{Company: "ACME", Balance: 1000}
	if err := store.CreateCustomer(c); err != nil {
		t.Fatalf("create customer: %v", err)
	}
	key, raw, err := store.CreateAPIKey(c.PublicID, "k1", "")
	if err != nil {
		t.Fatalf("create key: %v", err)
	}

	call := func() int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"deepseek-v4-flash"}`))
		req.Header.Set("Authorization", "Bearer "+raw)
		srv.Router().ServeHTTP(rec, req)
		return rec.Code
	}

	if code := call(); code != http.StatusOK {
		t.Fatalf("pre-revoke status = %d", code)
	}

	rec := httptest.NewRecorder()
	preq := httptest.NewRequest(http.MethodPatch, "/admin/api-keys/"+key.PublicID+"/status", strings.NewReader(`{"status":"Disabled"}`))
	preq.Header.Set("Authorization", "Bearer test-admin")
	srv.Router().ServeHTTP(rec, preq)
	if rec.Code != http.StatusOK {
		t.Fatalf("revoke status = %d body=%s", rec.Code, rec.Body.String())
	}

	if code := call(); code != http.StatusUnauthorized {
		t.Fatalf("post-revoke status = %d, want 401", code)
	}
}

func TestMaxTokensPerScenarioEnforced(t *testing.T) {
	srv, store := newTestServer(t, jsonUsageTransport(t))
	raw := seedCustomerKey(t, store, 1000)

	// Seeded model-api route caps max_tokens at 32000.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"deepseek-v4-flash","max_tokens":40000}`))
	req.Header.Set("Authorization", "Bearer "+raw)
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("over-limit status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"deepseek-v4-flash","max_tokens":1000}`))
	req.Header.Set("Authorization", "Bearer "+raw)
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("within-limit status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
}

func TestUnknownScenarioFallsBackToModelAPIRoute(t *testing.T) {
	var gotScenario string
	srv, store := newTestServer(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotScenario = r.Header.Get("X-Reseller-Scenario")
		return mockResponse("application/json", `{"choices":[],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`), nil
	}))
	raw := seedCustomerKey(t, store, 1000)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"messages":[]}`))
	req.Header.Set("Authorization", "Bearer "+raw)
	req.Header.Set("X-DAXI-Scenario", "unknown-scenario")
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if gotScenario != "model-api" {
		t.Fatalf("forwarded scenario = %q, want model-api", gotScenario)
	}
}

func TestProxyStreamingNoFreeRideWhenUpstreamOmitsUsage(t *testing.T) {
	// Sanity: if upstream never reports usage, nothing is deducted (documents the
	// dependency on include_usage support upstream).
	srv, store := newTestServer(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return mockResponse("text/event-stream", "data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n\ndata: [DONE]\n\n"), nil
	}))
	raw := seedCustomerKey(t, store, 1000)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"deepseek-v4-flash","stream":true,"messages":[]}`))
	req.Header.Set("Authorization", "Bearer "+raw)
	srv.Router().ServeHTTP(rec, req)

	if got := currentBalance(t, store, raw); got != 1000 {
		t.Fatalf("balance = %d, want 1000 (no usage reported)", got)
	}
}

func TestStreamingFloorsBalanceAndRecordsOverspend(t *testing.T) {
	// Streaming usage (30) exceeds the remaining balance (10): balance must floor
	// at 0 and the 20-token excess must be recorded as overspend (P0-1 × P0-3).
	srv, store := newTestServer(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := bytes.NewBufferString("")
		_, _ = body.WriteString("data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n")
		_, _ = body.WriteString("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":20,\"total_tokens\":30}}\n\n")
		_, _ = body.WriteString("data: [DONE]\n\n")
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       io.NopCloser(body),
		}, nil
	}))
	raw := seedCustomerKey(t, store, 10)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"deepseek-v4-flash","stream":true,"messages":[]}`))
	req.Header.Set("Authorization", "Bearer "+raw)
	srv.Router().ServeHTTP(rec, req)

	if got := currentBalance(t, store, raw); got != 0 {
		t.Fatalf("balance = %d, want 0 (floored)", got)
	}
	records, err := store.ListUsageRecords(10)
	if err != nil {
		t.Fatalf("list usage: %v", err)
	}
	if len(records) == 0 {
		t.Fatalf("no usage record written")
	}
	if records[0].OverspendTokens != 20 {
		t.Fatalf("overspend = %d, want 20", records[0].OverspendTokens)
	}
}

func TestFirstPhaseEndToEndFlow(t *testing.T) {
	var upstreamCustomerID string
	var upstreamKeyID string
	var upstreamScenario string
	var upstreamAuth string
	srv, store := newTestServer(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		upstreamCustomerID = r.Header.Get("X-Reseller-Customer-ID")
		upstreamKeyID = r.Header.Get("X-Reseller-Key-ID")
		upstreamScenario = r.Header.Get("X-Reseller-Scenario")
		upstreamAuth = r.Header.Get("Authorization")
		return mockResponse("application/json", `{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":50,"completion_tokens":73,"total_tokens":123}}`), nil
	}))

	rec := doJSON(t, srv, http.MethodPost, "/public/leads", "", `{"company":"DAXI Client","email":"ops@example.com","notes":"OEM launch"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create lead status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, srv, http.MethodPost, "/admin/sessions", "", `{"admin_token":"test-admin"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("admin session status = %d body=%s", rec.Code, rec.Body.String())
	}
	var session struct {
		SessionToken string `json:"session_token"`
	}
	decodeBody(t, rec, &session)
	adminAuth := "Bearer " + session.SessionToken

	rec = doJSON(t, srv, http.MethodPost, "/admin/customers", adminAuth, `{"company":"DAXI Client","email":"buyer@example.com"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create customer status = %d body=%s", rec.Code, rec.Body.String())
	}
	var customer model.Customer
	decodeBody(t, rec, &customer)

	rec = doJSON(t, srv, http.MethodPost, "/admin/token-orders", adminAuth, `{"customer_public_id":"`+customer.PublicID+`","plan_public_id":"DXP-TRIAL","notes":"first phase pack"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create token order status = %d body=%s", rec.Code, rec.Body.String())
	}
	var order model.TokenOrder
	decodeBody(t, rec, &order)

	rec = doJSON(t, srv, http.MethodPost, "/admin/payments", adminAuth, `{"order_public_id":"`+order.PublicID+`","customer_public_id":"`+customer.PublicID+`","provider":"manual","amount_cents":9900,"currency":"USD","status":"Pending"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create payment status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payment model.PaymentRecord
	decodeBody(t, rec, &payment)

	rec = doJSON(t, srv, http.MethodPatch, "/admin/payments/"+payment.PublicID+"/status", adminAuth, `{"status":"Paid"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("settle payment status = %d body=%s", rec.Code, rec.Body.String())
	}
	var settlement struct {
		Payment  model.PaymentRecord  `json:"payment"`
		Recharge *model.TokenRecharge `json:"recharge"`
	}
	decodeBody(t, rec, &settlement)
	if settlement.Recharge == nil || settlement.Recharge.Tokens != order.Tokens {
		t.Fatalf("recharge = %+v, want %d tokens", settlement.Recharge, order.Tokens)
	}

	rec = doJSON(t, srv, http.MethodPost, "/admin/api-keys", adminAuth, `{"customer_public_id":"`+customer.PublicID+`","name":"Production key"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create api key status = %d body=%s", rec.Code, rec.Body.String())
	}
	var keyResp struct {
		APIKey model.APIKey `json:"api_key"`
		Secret string       `json:"secret"`
	}
	decodeBody(t, rec, &keyResp)
	customerAuth := "Bearer " + keyResp.Secret

	rec = doJSON(t, srv, http.MethodGet, "/v1/models", customerAuth, ``)
	if rec.Code != http.StatusOK {
		t.Fatalf("models status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, srv, http.MethodPost, "/v1/chat/completions", customerAuth, `{"messages":[{"role":"user","content":"hello"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("chat status = %d body=%s", rec.Code, rec.Body.String())
	}

	if upstreamAuth != "Bearer upstream-secret" {
		t.Fatalf("upstream auth = %q", upstreamAuth)
	}
	if upstreamCustomerID != customer.PublicID {
		t.Fatalf("upstream customer id = %q, want %q", upstreamCustomerID, customer.PublicID)
	}
	if upstreamKeyID != keyResp.APIKey.PublicID {
		t.Fatalf("upstream key id = %q, want %q", upstreamKeyID, keyResp.APIKey.PublicID)
	}
	if upstreamScenario != "model-api" {
		t.Fatalf("upstream scenario = %q, want model-api", upstreamScenario)
	}

	storedCustomer, err := store.GetCustomer(customer.PublicID)
	if err != nil {
		t.Fatalf("get customer: %v", err)
	}
	if storedCustomer.Balance != order.Tokens-123 {
		t.Fatalf("balance = %d, want %d", storedCustomer.Balance, order.Tokens-123)
	}

	rec = doJSON(t, srv, http.MethodGet, "/admin/usage", adminAuth, ``)
	if rec.Code != http.StatusOK {
		t.Fatalf("usage status = %d body=%s", rec.Code, rec.Body.String())
	}
	var usage model.UsageSummary
	decodeBody(t, rec, &usage)
	if usage.TotalTokens != 123 || usage.RequestCount != 1 {
		t.Fatalf("usage = %+v, want one 123-token request", usage)
	}
}

func doJSON(t *testing.T, srv *Server, method, path, auth, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	if path == "/v1/chat/completions" {
		req.Header.Set("X-DAXI-Scenario", "unknown-scenario")
	}
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
}
