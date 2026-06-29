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

// 真实上游契约(P1 实测):响应统一信封 {success,message,data};draft 不计费;
// task 计费走 cost_snapshot.estimated_quota(预扣)与终态 data.quota(实扣);任务 id 用 string task_id。
// 下列 mock 全部按真实形态构造。

func envelope(dataJSON string) string {
	return `{"success":true,"message":"","data":` + dataJSON + `}`
}

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

// 主链路:真实信封 + draft 不计费 + task 终态按 data.quota 实扣 + 状态映射不漏 raw + 结算恰好一次 + 不泄露上游内部字段。
func TestVideoAgentFlow_RealContract(t *testing.T) {
	var mu sync.Mutex
	seen := []http.Header{}
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		mu.Lock()
		seen = append(seen, r.Header.Clone())
		mu.Unlock()
		switch {
		case strings.HasSuffix(r.URL.Path, "/drafts/generate"):
			// draft.id 是数字;含 user_id 内部字段(用于验证不泄露);estimate.is_billable=false。
			return mockResponse("application/json", envelope(
				`{"draft":{"id":15,"user_id":1,"status":"draft","product_name":"x"},`+
					`"script":{"title":"t","hook":"h"},`+
					`"estimate":{"currency":"CNY","estimated_amount":57,"is_billable":false,"require_confirm":true}}`)), nil
		case strings.HasSuffix(r.URL.Path, "/confirm"):
			return mockResponse("application/json", envelope(`{"id":15,"status":"confirmed"}`)), nil
		case strings.HasSuffix(r.URL.Path, "/tasks") && r.Method == http.MethodPost:
			// 真实预扣在 data.data.cost_snapshot.estimated_quota;任务 id 用 string task_id。
			return mockResponse("application/json", envelope(
				`{"id":12,"task_id":"task_abc","user_id":1,"status":"QUEUED","quota":0,`+
					`"data":{"cost_snapshot":{"estimated_quota":570000,"quota_per_unit":10000,"estimated_amount":57,"billable":true}}}`)), nil
		case strings.Contains(r.URL.Path, "/tasks/task_abc"):
			// 终态实扣 data.quota=5000;progress 是 "100%" 字符串。
			return mockResponse("application/json", envelope(
				`{"id":12,"task_id":"task_abc","status":"SUCCESS","progress":"100%",`+
					`"result_url":"https://supchuang.com/v/x.mp4","quota":5000}`)), nil
		}
		return mockResponse("application/json", envelope(`{}`)), nil
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

	// 1) draft 生成 —— 不计费
	rec := do("POST", "/v1/video-agent/drafts/generate", `{"product_name":"x","selling_points":"y","video_model":"m"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("generate code=%d body=%s", rec.Code, rec.Body.String())
	}
	var gen struct {
		DraftID string `json:"draft_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &gen); err != nil {
		t.Fatalf("decode generate: %v", err)
	}
	if gen.DraftID == "" {
		t.Fatal("missing draft_id")
	}
	if strings.Contains(rec.Body.String(), "\"draft\":\"draft\"") || strings.Contains(rec.Body.String(), "draft_ready") {
		t.Fatalf("raw status leaked: %s", rec.Body.String())
	}
	// 不得泄露上游内部字段(user_id)
	if strings.Contains(rec.Body.String(), "user_id") {
		t.Fatalf("上游内部字段 user_id 泄露给客户: %s", rec.Body.String())
	}
	// draft 不计费:余额不变
	if bal := videoQuotaBalanceHTTP(t, store, cust.PublicID); bal != 1_000_000 {
		t.Fatalf("draft 计费了:balance=%d want 1000000(draft 不计费)", bal)
	}

	// 2) 建任务(QUEUED → Queued),真实预扣 570000 写入 estimated_quota
	rec = do("POST", "/v1/video-agent/drafts/"+gen.DraftID+"/tasks", `{}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create task code=%d body=%s", rec.Code, rec.Body.String())
	}
	var tk struct {
		TaskID         string `json:"task_id"`
		Status         string `json:"status"`
		EstimatedQuota int64  `json:"estimated_quota"`
	}
	json.Unmarshal(rec.Body.Bytes(), &tk)
	if tk.TaskID == "" {
		t.Fatal("missing task_id")
	}
	if tk.Status != "Queued" {
		t.Fatalf("task status=%q want Queued", tk.Status)
	}
	if tk.EstimatedQuota != 570000 {
		t.Fatalf("estimated_quota=%d want 570000(真实 cost_snapshot)", tk.EstimatedQuota)
	}
	if strings.Contains(rec.Body.String(), "user_id") {
		t.Fatalf("task 响应泄露 user_id: %s", rec.Body.String())
	}

	// 3) 查询任务(终态 SUCCESS)→ 实扣 data.quota=5000
	rec = do("GET", "/v1/video-agent/tasks/"+tk.TaskID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get task code=%d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "SUCCESS") {
		t.Fatalf("raw 枚举泄露: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Completed") {
		t.Fatalf("缺映射后状态: %s", rec.Body.String())
	}

	// 重试不重复结算
	do("GET", "/v1/video-agent/tasks/"+tk.TaskID, "")
	do("GET", "/v1/video-agent/tasks/"+tk.TaskID, "")

	c, err := store.GetCustomer(cust.PublicID)
	if err != nil {
		t.Fatalf("get customer: %v", err)
	}
	if want := int64(1_000_000 - 5000); c.BalanceQuota != want {
		t.Fatalf("balance_quota=%d want %d(draft 0 + task 实扣 5000,不重复)", c.BalanceQuota, want)
	}

	gotTask, err := store.GetVideoAgentTaskForCustomer(tk.TaskID, cust.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if gotTask.UsageRecordID <= 0 {
		t.Fatalf("task.usage_record_id 未回链, got %d", gotTask.UsageRecordID)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(seen) == 0 {
		t.Fatal("未捕获上游调用")
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
		if h.Get("Authorization") != "Bearer upstream-secret" {
			t.Errorf("call %d 上游 Authorization=%q", i, h.Get("Authorization"))
		}
	}
}

func TestVideoAgentRejectsMissingUpstreamDraftID(t *testing.T) {
	var calls atomic.Int64
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if strings.HasSuffix(r.URL.Path, "/drafts/generate") {
			// data.draft 无 id
			return mockResponse("application/json", envelope(`{"draft":{"status":"draft"},"script":{}}`)), nil
		}
		return mockResponse("application/json", envelope(`{}`)), nil
	})
	srv, store := newVideoAgentTestCustomer(t, transport, 1_000_000)
	rawKey := seedVideoAPIKey(t, store)

	rec := videoReq(t, srv, rawKey, "POST", "/v1/video-agent/drafts/generate", `{"product_name":"x","selling_points":"y"}`)
	if rec.Code == http.StatusOK {
		t.Fatalf("draft without upstream id should fail, body=%s", rec.Body.String())
	}
	if bal := videoQuotaBalanceHTTP(t, store, firstCustomerPublicID(t, store)); bal != 1_000_000 {
		t.Fatalf("balance changed: %d", bal)
	}
	if calls.Load() != 1 {
		t.Fatalf("calls=%d want 1", calls.Load())
	}
}

func TestVideoAgentRejectsMissingUpstreamTaskID(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/drafts/generate"):
			return mockResponse("application/json", envelope(`{"draft":{"id":15,"status":"draft"},"script":{}}`)), nil
		case strings.HasSuffix(r.URL.Path, "/tasks") && r.Method == http.MethodPost:
			// 无 task_id
			return mockResponse("application/json", envelope(`{"id":12,"status":"QUEUED"}`)), nil
		}
		return mockResponse("application/json", envelope(`{}`)), nil
	})
	srv, store := newVideoAgentTestCustomer(t, transport, 1_000_000)
	rawKey := seedVideoAPIKey(t, store)

	draftRec := videoReq(t, srv, rawKey, "POST", "/v1/video-agent/drafts/generate", `{"product_name":"x","selling_points":"y"}`)
	if draftRec.Code != http.StatusOK {
		t.Fatalf("draft code=%d body=%s", draftRec.Code, draftRec.Body.String())
	}
	var draft struct {
		DraftID string `json:"draft_id"`
	}
	json.Unmarshal(draftRec.Body.Bytes(), &draft)

	taskRec := videoReq(t, srv, rawKey, "POST", "/v1/video-agent/drafts/"+draft.DraftID+"/tasks", `{}`)
	if taskRec.Code == http.StatusCreated {
		t.Fatalf("task without upstream id should fail, body=%s", taskRec.Body.String())
	}
	var taskRows int
	if err := store.DB().QueryRow(`SELECT COUNT(*) FROM video_tasks WHERE task_type = 'video-agent'`).Scan(&taskRows); err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if taskRows != 0 {
		t.Fatalf("video task rows=%d want 0", taskRows)
	}
	if bal := videoQuotaBalanceHTTP(t, store, firstCustomerPublicID(t, store)); bal != 1_000_000 {
		t.Fatalf("balance=%d want 1000000(draft 不计费)", bal)
	}
}

func TestVideoAgentDraftIdempotencyPrecheckDoesNotCallUpstreamTwice(t *testing.T) {
	var draftCalls atomic.Int64
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/drafts/generate") {
			n := draftCalls.Add(1)
			return mockResponse("application/json", envelope(`{"draft":{"id":`+string(rune('0'+n))+`,"status":"draft"},"script":{}}`)), nil
		}
		return mockResponse("application/json", envelope(`{}`)), nil
	})
	srv, store := newVideoAgentTestCustomer(t, transport, 1_000_000)
	rawKey := seedVideoAPIKey(t, store)

	do := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/v1/video-agent/drafts/generate", strings.NewReader(`{"product_name":"x","selling_points":"y"}`))
		req.Header.Set("Authorization", "Bearer "+rawKey)
		req.Header.Set("Idempotency-Key", "idem-1")
		rec := httptest.NewRecorder()
		srv.Router().ServeHTTP(rec, req)
		return rec
	}

	if first := do(); first.Code != http.StatusOK {
		t.Fatalf("first code=%d body=%s", first.Code, first.Body.String())
	}
	if second := do(); second.Code != http.StatusOK {
		t.Fatalf("second code=%d body=%s", second.Code, second.Body.String())
	}
	if draftCalls.Load() != 1 {
		t.Fatalf("draft upstream calls=%d want 1", draftCalls.Load())
	}
}

func TestVideoAgentCompensationSettlesUnpolledCompletedTask(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/drafts/generate"):
			return mockResponse("application/json", envelope(`{"draft":{"id":15,"status":"draft"},"script":{}}`)), nil
		case strings.HasSuffix(r.URL.Path, "/tasks") && r.Method == http.MethodPost:
			return mockResponse("application/json", envelope(`{"id":12,"task_id":"task_comp","status":"QUEUED","data":{"cost_snapshot":{"estimated_quota":570000}}}`)), nil
		case strings.Contains(r.URL.Path, "/tasks/task_comp"):
			return mockResponse("application/json", envelope(`{"task_id":"task_comp","status":"SUCCESS","progress":"100%","result_url":"https://supchuang.com/v/x.mp4","quota":5000}`)), nil
		}
		return mockResponse("application/json", envelope(`{}`)), nil
	})
	srv, store := newVideoAgentTestCustomer(t, transport, 1_000_000)
	rawKey := seedVideoAPIKey(t, store)

	draftRec := videoReq(t, srv, rawKey, "POST", "/v1/video-agent/drafts/generate", `{"product_name":"x","selling_points":"y"}`)
	if draftRec.Code != http.StatusOK {
		t.Fatalf("draft code=%d body=%s", draftRec.Code, draftRec.Body.String())
	}
	var draft struct {
		DraftID string `json:"draft_id"`
	}
	json.Unmarshal(draftRec.Body.Bytes(), &draft)
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
	if bal := videoQuotaBalanceHTTP(t, store, firstCustomerPublicID(t, store)); bal != 995_000 {
		t.Fatalf("balance=%d want 995000(draft 0 + task 5000)", bal)
	}
	var usageRecordID int64
	if err := store.DB().QueryRow(`SELECT usage_record_id FROM video_tasks WHERE upstream_task_id = 'task_comp'`).Scan(&usageRecordID); err != nil {
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
