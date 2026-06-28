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
