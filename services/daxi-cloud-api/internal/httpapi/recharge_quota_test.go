package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"daxi-cloud-api/internal/model"
)

// P0:视频额度(quota)钱包必须能通过 admin 充值入口注资,否则生产里所有客户 balance_quota=0,
// video-agent 全部 402,功能不可交付。本测试端到端证明:充值 quota 钱包 → 解锁 video-agent。
func TestRechargeQuotaWalletUnblocksVideoAgent(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/drafts/generate") {
			return mockResponse("application/json", `{"id":"upd-q","script":"s","status":"draft_ready","usage":{"quota":100}}`), nil
		}
		return mockResponse("application/json", `{}`), nil
	})
	srv, store := newTestServer(t, transport)
	// 模拟生产门槛:draft 预估额度 80000,balance_quota 不足则 402。
	srv.cfg.VideoAgentEstDraftQuota = 80_000

	rec := doJSON(t, srv, http.MethodPost, "/admin/sessions", "", `{"admin_token":"test-admin"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("admin session status=%d body=%s", rec.Code, rec.Body.String())
	}
	var session struct {
		SessionToken string `json:"session_token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &session); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	adminAuth := "Bearer " + session.SessionToken

	cust := &model.Customer{Company: "VQ", Status: "Active"}
	if err := store.CreateCustomer(cust); err != nil {
		t.Fatalf("create customer: %v", err)
	}
	_, rawKey, err := store.CreateAPIKey(cust.PublicID, "k", "")
	if err != nil {
		t.Fatalf("create key: %v", err)
	}

	// 充值前:balance_quota=0 → draft generate 必须 402
	pre := videoReq(t, srv, rawKey, "POST", "/v1/video-agent/drafts/generate", `{"prompt":"x"}`)
	if pre.Code != http.StatusPaymentRequired {
		t.Fatalf("pre-fund draft want 402 got %d body=%s", pre.Code, pre.Body.String())
	}

	// 通过 admin 充值入口给「视频额度(quota)」钱包注资
	rec = doJSON(t, srv, http.MethodPost, "/admin/recharges", adminAuth,
		`{"customer_public_id":"`+cust.PublicID+`","tokens":1000000,"wallet":"quota","source":"manual"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("quota recharge want 201 got %d body=%s", rec.Code, rec.Body.String())
	}
	var charged model.TokenRecharge
	if err := json.Unmarshal(rec.Body.Bytes(), &charged); err != nil {
		t.Fatalf("decode recharge: %v", err)
	}
	if charged.Wallet != "quota" {
		t.Fatalf("recharge wallet=%q want quota", charged.Wallet)
	}
	if charged.BalanceAfter != 1_000_000 {
		t.Fatalf("recharge balance_after=%d want 1000000", charged.BalanceAfter)
	}

	// 充值只动 quota 钱包,不得碰 token 钱包
	rec = doJSON(t, srv, http.MethodGet, "/admin/customers/"+cust.PublicID, adminAuth, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get customer status=%d", rec.Code)
	}
	var detail struct {
		Customer model.Customer `json:"customer"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode customer: %v", err)
	}
	got := detail.Customer
	if got.BalanceQuota != 1_000_000 {
		t.Fatalf("balance_quota after quota recharge=%d want 1000000", got.BalanceQuota)
	}
	if got.Balance != 0 {
		t.Fatalf("quota recharge must NOT touch balance_tokens, got %d", got.Balance)
	}

	// 充值后:draft generate 必须成功
	post := videoReq(t, srv, rawKey, "POST", "/v1/video-agent/drafts/generate", `{"prompt":"x"}`)
	if post.Code != http.StatusOK {
		t.Fatalf("post-fund draft want 200 got %d body=%s", post.Code, post.Body.String())
	}
}
