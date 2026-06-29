package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"daxi-cloud-api/internal/model"
)

// P0:视频额度(quota)钱包必须能通过 admin 充值入口注资,否则生产里客户 balance_quota=0,
// video-agent 建任务全部 402,功能不可交付。本测试端到端证明:充值 quota 钱包 → 解锁建任务。
// 注:draft 不计费(真实契约),计费闸在【建任务】处。
func TestRechargeQuotaWalletUnblocksVideoAgent(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/drafts/generate"):
			return mockResponse("application/json", envelope(`{"draft":{"id":15,"status":"draft"},"script":{}}`)), nil
		case strings.HasSuffix(r.URL.Path, "/tasks") && r.Method == http.MethodPost:
			return mockResponse("application/json", envelope(`{"id":12,"task_id":"task_q","status":"QUEUED","data":{"cost_snapshot":{"estimated_quota":570000}}}`)), nil
		}
		return mockResponse("application/json", envelope(`{}`)), nil
	})
	srv, store := newTestServer(t, transport)
	// 建任务闸:balance_quota 不足则 402。
	srv.cfg.VideoAgentEstTaskQuota = 600_000

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

	// draft 免费,先拿到 draft_id
	dr := videoReq(t, srv, rawKey, "POST", "/v1/video-agent/drafts/generate", `{"product_name":"x","selling_points":"y"}`)
	if dr.Code != http.StatusOK {
		t.Fatalf("draft want 200 got %d body=%s", dr.Code, dr.Body.String())
	}
	var draft struct {
		DraftID string `json:"draft_id"`
	}
	json.Unmarshal(dr.Body.Bytes(), &draft)

	// 充值前:balance_quota=0 → 建任务必须 402
	pre := videoReq(t, srv, rawKey, "POST", "/v1/video-agent/drafts/"+draft.DraftID+"/tasks", `{}`)
	if pre.Code != http.StatusPaymentRequired {
		t.Fatalf("pre-fund task want 402 got %d body=%s", pre.Code, pre.Body.String())
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

	// 充值后:建任务必须成功
	post := videoReq(t, srv, rawKey, "POST", "/v1/video-agent/drafts/"+draft.DraftID+"/tasks", `{}`)
	if post.Code != http.StatusCreated {
		t.Fatalf("post-fund task want 201 got %d body=%s", post.Code, post.Body.String())
	}
}
