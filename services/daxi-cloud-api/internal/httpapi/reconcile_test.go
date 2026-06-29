package httpapi

import "testing"

// Query B 聚合:按 other.reseller.customer_id 拆,type=2 消费 / type=6 退款,net=消费−退款。
func TestAggregateUpstreamResellerLogs(t *testing.T) {
	// 两个客户:A 两笔消费 + 一笔退款;B 一笔消费。混入一条非消费/退款(type=3)与一条无 reseller 的,应忽略。
	body := []byte(`{"success":true,"message":"","data":[
		{"type":2,"quota":2812500,"other":"{\"reseller\":{\"customer_id\":\"DXC-A\"}}"},
		{"type":2,"quota":1000000,"other":"{\"reseller\":{\"customer_id\":\"DXC-A\"}}"},
		{"type":6,"quota":300000,"other":"{\"reseller\":{\"customer_id\":\"DXC-A\"}}"},
		{"type":2,"quota":500000,"other":"{\"reseller\":{\"customer_id\":\"DXC-B\"}}"},
		{"type":3,"quota":999,"other":"{\"reseller\":{\"customer_id\":\"DXC-A\"}}"},
		{"type":2,"quota":777,"other":"{}"}
	]}`)
	rows, err := aggregateUpstreamResellerLogs(body)
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	got := map[string]struct{ consume, refund, net, recs int64 }{}
	for _, r := range rows {
		got[r.ResellerCustomerID] = struct{ consume, refund, net, recs int64 }{r.ConsumeQuota, r.RefundQuota, r.NetQuota, r.Records}
	}
	a := got["DXC-A"]
	if a.consume != 3_812_500 || a.refund != 300_000 || a.net != 3_512_500 || a.recs != 3 {
		t.Fatalf("DXC-A=%+v want consume=3812500 refund=300000 net=3512500 recs=3", a)
	}
	b := got["DXC-B"]
	if b.consume != 500_000 || b.refund != 0 || b.net != 500_000 || b.recs != 1 {
		t.Fatalf("DXC-B=%+v want consume=500000 net=500000 recs=1", b)
	}
	if len(rows) != 2 {
		t.Fatalf("rows=%d want 2(type=3 与无 reseller 的应忽略)", len(rows))
	}
	// net 降序
	if rows[0].ResellerCustomerID != "DXC-A" {
		t.Fatalf("排序错:rows[0]=%s want DXC-A(net 最大)", rows[0].ResellerCustomerID)
	}
}

func TestAggregateUpstreamResellerLogs_RejectsFailureEnvelope(t *testing.T) {
	_, err := aggregateUpstreamResellerLogs([]byte(`{"success":false,"message":"无效的令牌"}`))
	if err == nil {
		t.Fatal("success=false 应报错")
	}
}
