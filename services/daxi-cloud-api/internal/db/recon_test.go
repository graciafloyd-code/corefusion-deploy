package db

import "testing"

// Query A:DAXI 自有 usage_records 按 customer_id 出视频消费净额,且只算视频任务事件、排除 chat。
func TestVideoReconByCustomer(t *testing.T) {
	store := openTestStore(t)
	c1 := fundVideoQuota(t, store, 1_000_000)
	c2 := fundVideoQuota(t, store, 50_000)

	// c1:两笔视频结算,net=300000
	if _, _, _, err := store.SettleVideoBillingEvent("task:a", c1, 0, "video-agent", 100_000, false); err != nil {
		t.Fatalf("settle a: %v", err)
	}
	if _, _, _, err := store.SettleVideoBillingEvent("task:b", c1, 0, "video-agent", 200_000, false); err != nil {
		t.Fatalf("settle b: %v", err)
	}
	// c2:一笔超余额结算,total_tokens=80000(net),overspend=30000
	if _, _, _, err := store.SettleVideoBillingEvent("task:c", c2, 0, "video-agent", 80_000, true); err != nil {
		t.Fatalf("settle c: %v", err)
	}
	// 一条非视频(chat)用量,upstream_event_key 为空 → 不应计入对账
	if _, err := store.DB().Exec(`INSERT INTO usage_records(request_id, customer_id, api_key_id, scenario, model, prompt_tokens, completion_tokens, total_tokens, overspend_tokens, status_code, created_at) VALUES ('r1', ?, 0, 'chat', 'm', 1, 1, 999, 0, 200, '2026-06-29T00:00:00Z')`, c1); err != nil {
		t.Fatalf("insert chat usage: %v", err)
	}

	rows, err := store.VideoReconByCustomer("", "")
	if err != nil {
		t.Fatalf("recon: %v", err)
	}
	byID := map[int64]int64{}
	over := map[int64]int64{}
	rec := map[int64]int64{}
	review := map[int64]int64{}
	for _, r := range rows {
		byID[r.CustomerID] = r.NetQuota
		over[r.CustomerID] = r.OverspendQuota
		rec[r.CustomerID] = r.Records
		review[r.CustomerID] = r.NeedsReview
	}
	if byID[c1] != 300_000 {
		t.Fatalf("c1 net=%d want 300000(且不含 chat 的 999)", byID[c1])
	}
	if rec[c1] != 2 {
		t.Fatalf("c1 records=%d want 2", rec[c1])
	}
	if byID[c2] != 80_000 {
		t.Fatalf("c2 net=%d want 80000", byID[c2])
	}
	if over[c2] != 30_000 {
		t.Fatalf("c2 overspend=%d want 30000", over[c2])
	}
	if review[c2] != 1 {
		t.Fatalf("c2 needs_review=%d want 1", review[c2])
	}
}
