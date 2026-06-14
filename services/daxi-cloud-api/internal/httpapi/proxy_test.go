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
		AllowedModels:     []string{"daxi-smart-router"},
		DefaultProxyModel: "daxi-smart-router",
		MaxBodyBytes:      1 << 20,
		AdminToken:        "test-admin",
	}
	return NewServer(cfg, store, upstream.NewClientWithTransport(cfg, transport)), store
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
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"daxi-smart-router","messages":[]}`))
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
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"daxi-smart-router","stream":true,"messages":[]}`))
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
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"daxi-smart-router"}`))
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
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"daxi-smart-router","max_tokens":40000}`))
	req.Header.Set("Authorization", "Bearer "+raw)
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("over-limit status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"daxi-smart-router","max_tokens":1000}`))
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
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"daxi-smart-router","stream":true,"messages":[]}`))
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
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"daxi-smart-router","stream":true,"messages":[]}`))
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
