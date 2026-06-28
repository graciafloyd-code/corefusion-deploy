package db

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"daxi-cloud-api/internal/model"
)

func fundVideoQuota(t *testing.T, store *Store, balance int64) int64 {
	t.Helper()
	cust := &model.Customer{Company: "Video Co", Status: "Active"}
	if err := store.CreateCustomer(cust); err != nil {
		t.Fatalf("create customer: %v", err)
	}
	if _, err := store.db.Exec(`UPDATE customers SET balance_quota = ? WHERE id = ?`, balance, cust.ID); err != nil {
		t.Fatalf("fund balance_quota: %v", err)
	}
	return cust.ID
}

func videoQuotaBalance(t *testing.T, store *Store, customerID int64) int64 {
	t.Helper()
	var b int64
	if err := store.db.QueryRow(`SELECT balance_quota FROM customers WHERE id = ?`, customerID).Scan(&b); err != nil {
		t.Fatalf("read balance_quota: %v", err)
	}
	return b
}

// ① 并发 N 个【不同】计费事件、余额有限：断言永不为负、扣到 0 后记 overspend、每事件恰好一条 usage。
func TestSettleVideoBillingEvent_ConcurrentNeverNegative(t *testing.T) {
	store := openTestStore(t)
	const initial = 100
	const perEvent = 10
	const events = 50
	customerID := fundVideoQuota(t, store, initial)

	var wg sync.WaitGroup
	wg.Add(events)
	for i := 0; i < events; i++ {
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("task:evt-%d", i) // 各不相同 = N 个独立事件
			if _, _, _, err := store.SettleVideoBillingEvent(key, customerID, 1, "video-agent", perEvent, false); err != nil {
				t.Errorf("settle: %v", err)
			}
		}(i)
	}
	wg.Wait()

	bal := videoQuotaBalance(t, store, customerID)
	var rows, totalQuota, overspend int64
	if err := store.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(total_tokens),0), COALESCE(SUM(overspend_tokens),0) FROM usage_records WHERE customer_id = ?`, customerID).
		Scan(&rows, &totalQuota, &overspend); err != nil {
		t.Fatalf("agg: %v", err)
	}
	t.Logf("balance=%d rows=%d totalQuota=%d overspend=%d", bal, rows, totalQuota, overspend)
	if bal < 0 {
		t.Fatalf("balance went negative: %d", bal)
	}
	if bal != 0 {
		t.Fatalf("balance want 0, got %d", bal)
	}
	if rows != events {
		t.Fatalf("usage rows want %d, got %d (每事件恰好一条)", events, rows)
	}
	if want := int64(events*perEvent - initial); overspend != want {
		t.Fatalf("overspend want %d, got %d", want, overspend)
	}
}

// ② 同一计费事件并发/重试多次：断言只结算一次(唯一索引原子去重)。无索引时为红。
func TestSettleVideoBillingEvent_SameEventSettledOnce(t *testing.T) {
	store := openTestStore(t)
	const initial = 1000
	const quota = 100
	const tries = 50
	customerID := fundVideoQuota(t, store, initial)

	const key = "task:upstream-fixed-123"
	var settledCount int64
	var wg sync.WaitGroup
	wg.Add(tries)
	for i := 0; i < tries; i++ {
		go func() {
			defer wg.Done()
			_, _, settled, err := store.SettleVideoBillingEvent(key, customerID, 1, "video-agent", quota, false)
			if err != nil {
				t.Errorf("settle: %v", err)
				return
			}
			if settled {
				atomic.AddInt64(&settledCount, 1)
			}
		}()
	}
	wg.Wait()

	bal := videoQuotaBalance(t, store, customerID)
	var rows int64
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM usage_records WHERE upstream_event_key = ?`, key).Scan(&rows); err != nil {
		t.Fatalf("count: %v", err)
	}
	t.Logf("settledCount=%d rows=%d balance=%d", settledCount, rows, bal)
	if settledCount != 1 {
		t.Fatalf("settled count want 1, got %d (透传重试/并发重复结算)", settledCount)
	}
	if rows != 1 {
		t.Fatalf("usage rows for event want 1, got %d", rows)
	}
	if bal != initial-quota {
		t.Fatalf("balance want %d (只扣一次), got %d", initial-quota, bal)
	}
}

func TestSettleVideoBillingEvent_ConflictReturnsExistingRecordID(t *testing.T) {
	store := openTestStore(t)
	customerID := fundVideoQuota(t, store, 1000)

	const key = "task:upstream-fixed-return-id"
	firstID, _, settled, err := store.SettleVideoBillingEvent(key, customerID, 1, "video-agent", 100, false)
	if err != nil {
		t.Fatalf("first settle: %v", err)
	}
	if !settled || firstID <= 0 {
		t.Fatalf("first settle id=%d settled=%v", firstID, settled)
	}

	secondID, _, settled, err := store.SettleVideoBillingEvent(key, customerID, 1, "video-agent", 100, false)
	if err != nil {
		t.Fatalf("second settle: %v", err)
	}
	if settled {
		t.Fatalf("second settle should be idempotent")
	}
	if secondID != firstID {
		t.Fatalf("conflict rec id = %d, want existing id %d", secondID, firstID)
	}
	if bal := videoQuotaBalance(t, store, customerID); bal != 900 {
		t.Fatalf("balance=%d want 900", bal)
	}
}

func TestSettleVideoBillingEvent_RejectsEmptyStableIDKeys(t *testing.T) {
	store := openTestStore(t)
	customerID := fundVideoQuota(t, store, 1000)

	for _, key := range []string{"draft:", "task:"} {
		if _, _, _, err := store.SettleVideoBillingEvent(key, customerID, 1, "video-agent", 100, false); err == nil {
			t.Fatalf("SettleVideoBillingEvent(%q) should reject empty upstream id", key)
		}
	}
	if bal := videoQuotaBalance(t, store, customerID); bal != 1000 {
		t.Fatalf("balance changed on rejected keys: %d", bal)
	}
}

// ③ 终态无 usage 兜底：用 estimated quota 结算 + needs_review,不静默不扣。
func TestSettleVideoBillingEvent_EstimatedFallbackMarksNeedsReview(t *testing.T) {
	store := openTestStore(t)
	const initial = 500
	const estimated = 120
	customerID := fundVideoQuota(t, store, initial)

	recID, overspend, settled, err := store.SettleVideoBillingEvent("task:no-usage-1", customerID, 1, "video-agent", estimated, true)
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	if !settled {
		t.Fatalf("want settled=true")
	}
	if recID <= 0 {
		t.Fatalf("want usage_record id > 0 (供回链 task), got %d", recID)
	}
	if overspend != 0 {
		t.Fatalf("overspend want 0, got %d", overspend)
	}
	if bal := videoQuotaBalance(t, store, customerID); bal != initial-estimated {
		t.Fatalf("balance want %d, got %d", initial-estimated, bal)
	}
	var needsReview int
	if err := store.db.QueryRow(`SELECT needs_review FROM usage_records WHERE upstream_event_key = ?`, "task:no-usage-1").Scan(&needsReview); err != nil {
		t.Fatalf("read needs_review: %v", err)
	}
	if needsReview != 1 {
		t.Fatalf("needs_review want 1, got %d (兜底结算必须标待复核)", needsReview)
	}
}

// P1-2(批量场景):多个【空上游 id】计费事件不得互相去重导致批量漏扣。
// 退化键全部以 ":" 结尾(draft:/task:)。若无 HasSuffix(":") 守卫,部分唯一索引
// (WHERE upstream_event_key != '')会让首个 "draft:" 插入成功并扣费、其余撞键返回"已结算",
// N 个真实计费坍缩成 1 次 = 批量漏扣。守卫应让每个都被拒、零结算、余额完好。
func TestSettleVideoBillingEvent_EmptyIDsDoNotBatchDedupUndercharge(t *testing.T) {
	store := openTestStore(t)
	const initial = 1000
	customerID := fundVideoQuota(t, store, initial)

	const n = 50
	var settled, rejected int64
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, _, ok, err := store.SettleVideoBillingEvent("draft:", customerID, 1, "video-agent", 100, false)
			if err != nil {
				atomic.AddInt64(&rejected, 1)
				return
			}
			if ok {
				atomic.AddInt64(&settled, 1)
			}
		}()
	}
	wg.Wait()

	if settled != 0 {
		t.Fatalf("settled=%d want 0(空 id 事件不应有任何结算,否则即批量坍缩漏扣)", settled)
	}
	if rejected != n {
		t.Fatalf("rejected=%d want %d(每个退化键都应被守卫拒绝)", rejected, n)
	}
	if bal := videoQuotaBalance(t, store, customerID); bal != initial {
		t.Fatalf("balance=%d want %d(批量空 id 不得扣费)", bal, initial)
	}
	var rows int64
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM usage_records WHERE customer_id = ?`, customerID).Scan(&rows); err != nil {
		t.Fatalf("count usage: %v", err)
	}
	if rows != 0 {
		t.Fatalf("usage rows=%d want 0", rows)
	}
}
