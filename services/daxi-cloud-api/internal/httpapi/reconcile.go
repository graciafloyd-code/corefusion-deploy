package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"daxi-cloud-api/internal/model"
)

// 视频对账(reconciliation)—— 第一步:两个查询各自能出数,自动比对差异留待第二步。
// 口径:DAXI 每客户净额(Query A,自有 usage_records)应 = 上游该 customer_id 的(消费−退款)净额(Query B)。

// Query B 临时数据源的硬约束标注 —— 必须随响应返回,严禁任何人当正式对账依据。
const upstreamReconCaveat = "TEMPORARY/SMOKE-ONLY: 数据来自上游 /api/log/token,仅最近 1000 条、无时间窗/分页/聚合;" +
	"量大时会静默丢弃更早记录。仅供冒烟核对,不可作为正式对账依据。正式对账需上游提供 reseller 汇总端点。"

// GET /admin/video-recon/daxi —— Query A:DAXI 自有 usage_records 按 customer_id 净额。
func (s *Server) handleVideoReconDaxi(w http.ResponseWriter, r *http.Request) {
	start := strings.TrimSpace(r.URL.Query().Get("start"))
	end := strings.TrimSpace(r.URL.Query().Get("end"))
	rows, err := s.store.VideoReconByCustomer(start, end)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query daxi usage")
		return
	}
	var total int64
	for _, x := range rows {
		total += x.NetQuota
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"source":       "daxi.usage_records",
		"unit":         "quota",
		"window":       map[string]string{"start": start, "end": end},
		"total_net":    total,
		"by_customer":  rows,
	})
}

// GET /admin/video-recon/upstream —— Query B(临时版):拉上游 /log/token 本地聚合。
func (s *Server) handleVideoReconUpstream(w http.ResponseWriter, r *http.Request) {
	body, err := s.upstream.GetResellerConsumeLogs(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to pull upstream logs: "+err.Error())
		return
	}
	rows, err := aggregateUpstreamResellerLogs(body)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	var consume, refund int64
	for _, x := range rows {
		consume += x.ConsumeQuota
		refund += x.RefundQuota
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"source":          "upstream./api/log/token",
		"unit":            "quota",
		"caveat":          upstreamReconCaveat,
		"authoritative":   false,
		"max_records_cap": 1000,
		"total_consume":   consume,
		"total_refund":    refund,
		"total_net":       consume - refund,
		"by_customer":     rows,
	})
}

// aggregateUpstreamResellerLogs 解析上游 /api/log/token 响应(信封 {success,data:[]log}),
// 按 other.reseller.customer_id 聚合:type=2 消费 / type=6 退款,net=消费−退款。纯函数,便于测试。
func aggregateUpstreamResellerLogs(body []byte) ([]model.VideoReconUpstreamRow, error) {
	var env struct {
		Success bool    `json:"success"`
		Message string  `json:"message"`
		Data    []struct {
			Type  int    `json:"type"`
			Quota int64  `json:"quota"`
			Other string `json:"other"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("invalid upstream log response")
	}
	if !env.Success {
		msg := strings.TrimSpace(env.Message)
		if msg == "" {
			msg = "upstream rejected log query"
		}
		return nil, fmt.Errorf("%s", msg)
	}
	const (
		logTypeConsume = 2
		logTypeRefund  = 6
	)
	agg := map[string]*model.VideoReconUpstreamRow{}
	for _, l := range env.Data {
		if l.Type != logTypeConsume && l.Type != logTypeRefund {
			continue
		}
		cid := resellerCustomerIDFromOther(l.Other)
		if cid == "" {
			continue
		}
		row := agg[cid]
		if row == nil {
			row = &model.VideoReconUpstreamRow{ResellerCustomerID: cid}
			agg[cid] = row
		}
		row.Records++
		if l.Type == logTypeConsume {
			row.ConsumeQuota += l.Quota
		} else {
			row.RefundQuota += l.Quota
		}
	}
	out := make([]model.VideoReconUpstreamRow, 0, len(agg))
	for _, r := range agg {
		r.NetQuota = r.ConsumeQuota - r.RefundQuota
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].NetQuota > out[j].NetQuota })
	return out, nil
}

// resellerCustomerIDFromOther 从日志 other(JSON 字符串)里取 reseller.customer_id。
func resellerCustomerIDFromOther(other string) string {
	if strings.TrimSpace(other) == "" {
		return ""
	}
	var m struct {
		Reseller struct {
			CustomerID string `json:"customer_id"`
		} `json:"reseller"`
	}
	if err := json.Unmarshal([]byte(other), &m); err != nil {
		return ""
	}
	return strings.TrimSpace(m.Reseller.CustomerID)
}
