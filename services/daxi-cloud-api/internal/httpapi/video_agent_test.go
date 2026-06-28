package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"daxi-cloud-api/internal/db"
	"daxi-cloud-api/internal/model"
)

func TestMapUpstreamStatus_OutputAlwaysInDAXIEnumSet(t *testing.T) {
	cases := map[string]string{
		"NOT_START": "Queued", "QUEUED": "Queued", "SUBMITTED": "Queued", "pending": "Queued",
		"IN_PROGRESS": "Running", "processing": "Running",
		"SUCCESS": "Completed", "done": "Completed",
		"FAILURE": "Failed", "failed": "Failed",
		"cancelled": "Canceled", "canceled": "Canceled",
		"some-future-raw-enum": "Running",
	}
	allowed := map[string]bool{"Queued": true, "Running": true, "Completed": true, "Failed": true, "Canceled": true}
	for raw, want := range cases {
		got := mapUpstreamStatus(raw)
		if got != want {
			t.Errorf("mapUpstreamStatus(%q) = %q, want %q", raw, got, want)
		}
		if !allowed[got] {
			t.Errorf("mapUpstreamStatus(%q) -> %q 不在 DAXI 五枚举内", raw, got)
		}
	}
}

// test④:归因头全带 + 状态映射不漏 raw + 终态结算恰好一次。
func TestVideoAgentFlow_AttributionStatusMappingSettleOnce(t *testing.T) {
	var mu sync.Mutex
	seen := []http.Header{}
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		mu.Lock()
		seen = append(seen, r.Header.Clone())
		mu.Unlock()
		switch {
		case strings.HasSuffix(r.URL.Path, "/drafts/generate"):
			return mockResponse("application/json", `{"id":"upd1","script":"hook line","status":"draft_ready","usage":{"quota":100}}`), nil
		case strings.HasSuffix(r.URL.Path, "/tasks") && r.Method == http.MethodPost:
			return mockResponse("application/json", `{"id":"upt1","status":"SUBMITTED"}`), nil
		case strings.Contains(r.URL.Path, "/tasks/upt1"):
			return mockResponse("application/json", `{"status":"SUCCESS","progress":100,"result_url":"https://supchuang.com/v/x.mp4","usage":{"quota":5000}}`), nil
		}
		return mockResponse("application/json", `{}`), nil
	})
	srv, store := newTestServer(t, transport)

	cust := &model.Customer{Company: "VC", Status: "Active"}
	if err := store.CreateCustomer(cust); err != nil {
		t.Fatalf("create customer: %v", err)
	}
	if err := store.UpdateCustomerQuotaBalance(cust.PublicID, 1_000_000); err != nil {
		t.Fatalf("fund quota: %v", err)
	}
	_, rawKey, err := store.CreateAPIKey(cust.PublicID, "k", "")
	if err != nil {
		t.Fatalf("create key: %v", err)
	}

	do := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+rawKey)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		srv.Router().ServeHTTP(rec, req)
		return rec
	}

	// 1) draft 生成(同步结算)
	rec := do("POST", "/v1/video-agent/drafts/generate", `{"scenario":"ecommerce-video","prompt":"x"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("generate code=%d body=%s", rec.Code, rec.Body.String())
	}
	var gen struct {
		DraftID string `json:"draft_id"`
		Usage   struct {
			Quota int64 `json:"quota"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &gen); err != nil {
		t.Fatalf("decode generate: %v", err)
	}
	if gen.DraftID == "" {
		t.Fatal("missing draft_id")
	}
	if gen.Usage.Quota != 100 {
		t.Fatalf("draft usage.quota=%d want 100", gen.Usage.Quota)
	}
	if strings.Contains(rec.Body.String(), "draft_ready") {
		t.Fatalf("raw status leaked in draft response: %s", rec.Body.String())
	}

	// 2) 建任务(SUBMITTED → Queued)
	rec = do("POST", "/v1/video-agent/drafts/"+gen.DraftID+"/tasks", `{}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create task code=%d body=%s", rec.Code, rec.Body.String())
	}
	var tk struct {
		TaskID string `json:"task_id"`
		Status string `json:"status"`
	}
	json.Unmarshal(rec.Body.Bytes(), &tk)
	if tk.TaskID == "" {
		t.Fatal("missing task_id")
	}
	if tk.Status != "Queued" {
		t.Fatalf("task status=%q want Queued(从 SUBMITTED 映射)", tk.Status)
	}

	// 3) 查询任务(终态 SUCCESS)→ 结算 + 状态映射
	rec = do("GET", "/v1/video-agent/tasks/"+tk.TaskID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get task code=%d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "SUCCESS") {
		t.Fatalf("raw 枚举泄露给客户: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Completed") {
		t.Fatalf("缺映射后状态: %s", rec.Body.String())
	}

	// 透传重试两次 → 不重复结算
	do("GET", "/v1/video-agent/tasks/"+tk.TaskID, "")
	do("GET", "/v1/video-agent/tasks/"+tk.TaskID, "")

	c, err := store.GetCustomer(cust.PublicID)
	if err != nil {
		t.Fatalf("get customer: %v", err)
	}
	if want := int64(1_000_000 - 100 - 5000); c.BalanceQuota != want {
		t.Fatalf("balance_quota=%d want %d(重复结算或漏扣?)", c.BalanceQuota, want)
	}

	// usage_record_id 已回链到 task。
	gotTask, err := store.GetVideoAgentTaskForCustomer(tk.TaskID, cust.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if gotTask.UsageRecordID <= 0 {
		t.Fatalf("task.usage_record_id 未回链, got %d", gotTask.UsageRecordID)
	}

	// 归因头:每次上游调用都带全套 X-Reseller-*
	mu.Lock()
	defer mu.Unlock()
	if len(seen) == 0 {
		t.Fatal("未捕获任何上游调用")
	}
	for i, h := range seen {
		if h.Get("X-Reseller-Code") != "daxi-cloud" {
			t.Errorf("call %d 缺 X-Reseller-Code", i)
		}
		if h.Get("X-Reseller-Customer-ID") != cust.PublicID {
			t.Errorf("call %d X-Reseller-Customer-ID=%q want %q", i, h.Get("X-Reseller-Customer-ID"), cust.PublicID)
		}
		if h.Get("X-Reseller-Scenario") != "video-agent" {
			t.Errorf("call %d X-Reseller-Scenario=%q", i, h.Get("X-Reseller-Scenario"))
		}
		if h.Get("X-Reseller-Request-ID") == "" {
			t.Errorf("call %d 缺 X-Reseller-Request-ID", i)
		}
		if h.Get("X-Reseller-Key-ID") == "" {
			t.Errorf("call %d 缺 X-Reseller-Key-ID", i)
		}
		if h.Get("Authorization") != "Bearer upstream-secret" {
			t.Errorf("call %d 上游 Authorization=%q", i, h.Get("Authorization"))
		}
	}
}

func TestVideoAgentRejectsMissingUpstreamStableIDs(t *testing.T) {
	var calls atomic.Int64
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		switch {
		case strings.HasSuffix(r.URL.Path, "/drafts/generate"):
			return mockResponse("application/json", `{"status":"draft_ready","usage":{"quota":100}}`), nil
		case strings.HasSuffix(r.URL.Path, "/tasks") && r.Method == http.MethodPost:
			return mockResponse("application/json", `{"status":"SUBMITTED"}`), nil
		}
		return mockResponse("application/json", `{}`), nil
	})
	srv, store := newTestServer(t, transport)
	cust := &model.Customer{Company: "VC", Status: "Active"}
	if err := store.CreateCustomer(cust); err != nil {
		t.Fatalf("create customer: %v", err)
	}
	if err := store.UpdateCustomerQuotaBalance(cust.PublicID, 1_000_000); err != nil {
		t.Fatalf("fund quota: %v", err)
	}
	_, rawKey, err := store.CreateAPIKey(cust.PublicID, "k", "")
	if err != nil {
		t.Fatalf("create key: %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/video-agent/drafts/generate", strings.NewReader(`{"prompt":"x"}`))
	req.Header.Set("Authorization", "Bearer "+rawKey)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatalf("draft without upstream id should fail, body=%s", rec.Body.String())
	}
	if bal := videoQuotaBalanceHTTP(t, store, cust.PublicID); bal != 1_000_000 {
		t.Fatalf("balance changed after missing draft id: %d", bal)
	}
	var usageRows int
	if err := store.DB().QueryRow(`SELECT COUNT(*) FROM usage_records WHERE customer_id = ?`, cust.ID).Scan(&usageRows); err != nil {
		t.Fatalf("count usage: %v", err)
	}
	if usageRows != 0 {
		t.Fatalf("usage rows after missing draft id = %d, want 0", usageRows)
	}

	if calls.Load() != 1 {
		t.Fatalf("calls=%d want 1", calls.Load())
	}
}

func TestVideoAgentRejectsMissingUpstreamTaskID(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/drafts/generate"):
			return mockResponse("application/json", `{"id":"upd-task-missing","status":"draft_ready","usage":{"quota":100}}`), nil
		case strings.HasSuffix(r.URL.Path, "/tasks") && r.Method == http.MethodPost:
			return mockResponse("application/json", `{"status":"SUBMITTED"}`), nil
		}
		return mockResponse("application/json", `{}`), nil
	})
	srv, store := newVideoAgentTestCustomer(t, transport, 1_000_000)
	rawKey := seedVideoAPIKey(t, store)

	draftRec := videoReq(t, srv, rawKey, "POST", "/v1/video-agent/drafts/generate", `{"prompt":"x"}`)
	if draftRec.Code != http.StatusOK {
		t.Fatalf("draft code=%d body=%s", draftRec.Code, draftRec.Body.String())
	}
	var draft struct {
		DraftID string `json:"draft_id"`
	}
	if err := json.Unmarshal(draftRec.Body.Bytes(), &draft); err != nil {
		t.Fatalf("decode draft: %v", err)
	}

	taskRec := videoReq(t, srv, rawKey, "POST", "/v1/video-agent/drafts/"+draft.DraftID+"/tasks", `{}`)
	if taskRec.Code == http.StatusCreated {
		t.Fatalf("task without upstream id should fail, body=%s", taskRec.Body.String())
	}
	var taskRows int
	if err := store.DB().QueryRow(`SELECT COUNT(*) FROM video_tasks WHERE task_type = 'video-agent'`).Scan(&taskRows); err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if taskRows != 0 {
		t.Fatalf("video task rows after missing upstream id = %d, want 0", taskRows)
	}
	if bal := videoQuotaBalanceHTTP(t, store, firstCustomerPublicID(t, store)); bal != 999900 {
		t.Fatalf("balance=%d want only draft charged", bal)
	}
}

func TestVideoAgentDraftIdempotencyPrecheckDoesNotCallUpstreamTwice(t *testing.T) {
	var draftCalls atomic.Int64
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/drafts/generate") {
			n := draftCalls.Add(1)
			return mockResponse("application/json", `{"id":"upd`+string(rune('0'+n))+`","status":"draft_ready","usage":{"quota":100}}`), nil
		}
		return mockResponse("application/json", `{}`), nil
	})
	srv, store := newVideoAgentTestCustomer(t, transport, 1_000_000)
	rawKey := seedVideoAPIKey(t, store)

	do := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/v1/video-agent/drafts/generate", strings.NewReader(`{"prompt":"x"}`))
		req.Header.Set("Authorization", "Bearer "+rawKey)
		req.Header.Set("Idempotency-Key", "idem-1")
		rec := httptest.NewRecorder()
		srv.Router().ServeHTTP(rec, req)
		return rec
	}

	first := do()
	if first.Code != http.StatusOK {
		t.Fatalf("first code=%d body=%s", first.Code, first.Body.String())
	}
	second := do()
	if second.Code != http.StatusOK {
		t.Fatalf("second code=%d body=%s", second.Code, second.Body.String())
	}
	if draftCalls.Load() != 1 {
		t.Fatalf("draft upstream calls=%d want 1", draftCalls.Load())
	}
	if bal := videoQuotaBalanceHTTP(t, store, firstCustomerPublicID(t, store)); bal != 999900 {
		t.Fatalf("balance=%d want one draft charge only", bal)
	}
}

func TestVideoAgentCompensationSettlesUnpolledCompletedTask(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/drafts/generate"):
			return mockResponse("application/json", `{"id":"upd-comp","status":"draft_ready","usage":{"quota":100}}`), nil
		case strings.HasSuffix(r.URL.Path, "/tasks") && r.Method == http.MethodPost:
			return mockResponse("application/json", `{"id":"upt-comp","status":"SUBMITTED"}`), nil
		case strings.Contains(r.URL.Path, "/tasks/upt-comp"):
			return mockResponse("application/json", `{"status":"SUCCESS","progress":100,"result_url":"https://supchuang.com/v/x.mp4","usage":{"quota":5000}}`), nil
		}
		return mockResponse("application/json", `{}`), nil
	})
	srv, store := newVideoAgentTestCustomer(t, transport, 1_000_000)
	rawKey := seedVideoAPIKey(t, store)

	draftRec := videoReq(t, srv, rawKey, "POST", "/v1/video-agent/drafts/generate", `{"prompt":"x"}`)
	if draftRec.Code != http.StatusOK {
		t.Fatalf("draft code=%d body=%s", draftRec.Code, draftRec.Body.String())
	}
	var draft struct {
		DraftID string `json:"draft_id"`
	}
	if err := json.Unmarshal(draftRec.Body.Bytes(), &draft); err != nil {
		t.Fatalf("decode draft: %v", err)
	}
	taskRec := videoReq(t, srv, rawKey, "POST", "/v1/video-agent/drafts/"+draft.DraftID+"/tasks", `{}`)
	if taskRec.Code != http.StatusCreated {
		t.Fatalf("task code=%d body=%s", taskRec.Code, taskRec.Body.String())
	}

	if _, err := store.DB().Exec(`UPDATE video_tasks SET created_at = ?`, time.Now().Add(-time.Hour).UTC()); err != nil {
		t.Fatalf("age task: %v", err)
	}
	if err := srv.settlePendingVideoAgentTasksOnce(time.Now().UTC()); err != nil {
		t.Fatalf("settle pending: %v", err)
	}
	if bal := videoQuotaBalanceHTTP(t, store, firstCustomerPublicID(t, store)); bal != 994900 {
		t.Fatalf("balance=%d want draft + task settled", bal)
	}
	var usageRecordID int64
	if err := store.DB().QueryRow(`SELECT usage_record_id FROM video_tasks WHERE upstream_task_id = 'upt-comp'`).Scan(&usageRecordID); err != nil {
		t.Fatalf("read task usage: %v", err)
	}
	if usageRecordID <= 0 {
		t.Fatalf("usage_record_id not linked")
	}
}

func videoReq(t *testing.T, srv *Server, rawKey, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+rawKey)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	return rec
}

func newVideoAgentTestCustomer(t *testing.T, transport http.RoundTripper, balance int64) (*Server, *db.Store) {
	t.Helper()
	srv, store := newTestServer(t, transport)
	cust := &model.Customer{Company: "VC", Status: "Active"}
	if err := store.CreateCustomer(cust); err != nil {
		t.Fatalf("create customer: %v", err)
	}
	if err := store.UpdateCustomerQuotaBalance(cust.PublicID, balance); err != nil {
		t.Fatalf("fund quota: %v", err)
	}
	return srv, store
}

func seedVideoAPIKey(t *testing.T, store *db.Store) string {
	t.Helper()
	publicID := firstCustomerPublicID(t, store)
	_, rawKey, err := store.CreateAPIKey(publicID, "k", "")
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	return rawKey
}

func firstCustomerPublicID(t *testing.T, store *db.Store) string {
	t.Helper()
	var publicID string
	if err := store.DB().QueryRow(`SELECT public_id FROM customers ORDER BY id LIMIT 1`).Scan(&publicID); err != nil {
		t.Fatalf("read customer public id: %v", err)
	}
	return publicID
}

func videoQuotaBalanceHTTP(t *testing.T, store *db.Store, publicID string) int64 {
	t.Helper()
	c, err := store.GetCustomer(publicID)
	if err != nil {
		t.Fatalf("get customer: %v", err)
	}
	return c.BalanceQuota
}
