package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"daxi-cloud-api/internal/model"
)

// resolveReconWindow 统一时间窗口口径:入参 start/end 为 Unix 秒(缺省 0 / 远期 = 全时段)。
// 上游正式端点要求 Unix 秒;DAXI usage_records.created_at 是 RFC3339 文本(按字符串可比 = 时序可比),
// 故同时返回两种格式,保证 A/B 用同一窗口。
func resolveReconWindow(r *http.Request) (unixStart, unixEnd, rfcStart, rfcEnd string) {
	si := parseUnixDefault(r.URL.Query().Get("start"), 0)
	ei := parseUnixDefault(r.URL.Query().Get("end"), 9999999999)
	return strconv.FormatInt(si, 10), strconv.FormatInt(ei, 10),
		time.Unix(si, 0).UTC().Format(time.RFC3339), time.Unix(ei, 0).UTC().Format(time.RFC3339)
}

func parseUnixDefault(s string, def int64) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		return v
	}
	return def
}

// 视频对账(reconciliation)—— 第一步:两个查询各自能出数,自动比对差异留待第二步。
// 口径:DAXI 每客户净额(Query A,自有 usage_records)应 = 上游该 customer_id 的(消费−退款)净额(Query B)。

// Query B 临时数据源的硬约束标注 —— 必须随响应返回,严禁任何人当正式对账依据。
const upstreamReconCaveat = "TEMPORARY/SMOKE-ONLY: 数据来自上游 /api/log/token,仅最近 1000 条、无时间窗/分页/聚合;" +
	"量大时会静默丢弃更早记录。仅供冒烟核对,不可作为正式对账依据。正式对账需上游提供 reseller 汇总端点。"

// GET /admin/video-recon/daxi —— Query A:DAXI 自有 usage_records 按 customer_id 净额。
func (s *Server) handleVideoReconDaxi(w http.ResponseWriter, r *http.Request) {
	unixStart, unixEnd, rfcStart, rfcEnd := resolveReconWindow(r)
	rows, err := s.store.VideoReconByCustomer(rfcStart, rfcEnd)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query daxi usage")
		return
	}
	var total int64
	for _, x := range rows {
		total += x.NetQuota
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"source":      "daxi.usage_records",
		"unit":        "quota",
		"window":      map[string]string{"start": unixStart, "end": unixEnd},
		"total_net":   total,
		"by_customer": rows,
	})
}

// GET /admin/video-recon/upstream —— Query B(临时版):拉上游 /log/token 本地聚合。
func (s *Server) handleVideoReconUpstream(w http.ResponseWriter, r *http.Request) {
	unixStart, unixEnd, _, _ := resolveReconWindow(r)
	scenario := strings.TrimSpace(r.URL.Query().Get("scenario"))
	rows, authoritative, source, caveat, err := s.upstreamReconRows(r.Context(), unixStart, unixEnd, scenario)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	var consume, refund int64
	for _, x := range rows {
		consume += x.ConsumeQuota
		refund += x.RefundQuota
	}
	out := map[string]any{
		"source":        source,
		"unit":          "quota",
		"authoritative": authoritative,
		"total_consume": consume,
		"total_refund":  refund,
		"total_net":     consume - refund,
		"by_customer":   rows,
	}
	if !authoritative {
		out["caveat"] = caveat
		out["max_records_cap"] = 1000
	}
	writeJSON(w, http.StatusOK, out)
}

// upstreamReconRows 取上游每客户用量:优先正式 reseller 汇总端点(authoritative);
// 端点不可用(如尚未部署 → 404)时回退到 /log/token 临时版(authoritative=false,带 caveat)。
func (s *Server) upstreamReconRows(ctx context.Context, start, end, scenario string) ([]model.VideoReconUpstreamRow, bool, string, string, error) {
	if body, err := s.upstream.GetResellerUsageSummary(ctx, start, end, scenario); err == nil {
		if rows, perr := parseResellerUsageSummary(body); perr == nil {
			return rows, true, "upstream.reseller-usage-summary", "", nil
		}
	}
	// 回退:临时 /log/token(最近 1000 条,不可作正式对账)
	body, err := s.upstream.GetResellerConsumeLogs(ctx)
	if err != nil {
		return nil, false, "", "", fmt.Errorf("failed to pull upstream usage: %w", err)
	}
	rows, err := aggregateUpstreamResellerLogs(body)
	if err != nil {
		return nil, false, "", "", err
	}
	return rows, false, "upstream./api/log/token", upstreamReconCaveat, nil
}

// parseResellerUsageSummary 解析上游正式端点响应 {success,data:{by_customer:[...]}}。
// by_customer 字段(reseller_customer_id/consume_quota/refund_quota/net_quota/records)与 VideoReconUpstreamRow 对齐。
func parseResellerUsageSummary(body []byte) ([]model.VideoReconUpstreamRow, error) {
	var env struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    struct {
			ByCustomer []model.VideoReconUpstreamRow `json:"by_customer"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("invalid usage-summary response")
	}
	if !env.Success {
		msg := strings.TrimSpace(env.Message)
		if msg == "" {
			msg = "upstream rejected usage-summary"
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return env.Data.ByCustomer, nil
}

// GET /admin/video-recon/diff —— step2:DAXI(A)与上游(B)按客户自动比差。
// 口径:DAXI 每客户净额(A,SUM(total_tokens))应 = 上游该 customer_id 的(消费−退款)净额(B)。
// 匹配键:DAXI customer_public_id == 上游 reseller_customer_id(DAXI 发的 X-Reseller-Customer-ID 即 customer.PublicID)。
func (s *Server) handleVideoReconDiff(w http.ResponseWriter, r *http.Request) {
	unixStart, unixEnd, rfcStart, rfcEnd := resolveReconWindow(r)
	scenario := strings.TrimSpace(r.URL.Query().Get("scenario"))

	daxiRows, err := s.store.VideoReconByCustomer(rfcStart, rfcEnd)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query daxi usage")
		return
	}
	upRows, authoritative, source, _, err := s.upstreamReconRows(r.Context(), unixStart, unixEnd, scenario)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	type diffRow struct {
		CustomerPublicID string `json:"customer_public_id"`
		Company          string `json:"company,omitempty"`
		DaxiNet          int64  `json:"daxi_net_quota"`
		UpstreamNet      int64  `json:"upstream_net_quota"`
		Diff             int64  `json:"diff"` // daxi - upstream(0 = 对平)
		Match            bool   `json:"match"`
	}
	merged := map[string]*diffRow{}
	for _, a := range daxiRows {
		merged[a.CustomerPublicID] = &diffRow{CustomerPublicID: a.CustomerPublicID, Company: a.Company, DaxiNet: a.NetQuota}
	}
	for _, b := range upRows {
		row := merged[b.ResellerCustomerID]
		if row == nil {
			row = &diffRow{CustomerPublicID: b.ResellerCustomerID}
			merged[b.ResellerCustomerID] = row
		}
		row.UpstreamNet = b.NetQuota
	}
	rows := make([]diffRow, 0, len(merged))
	var totalDaxi, totalUp, mismatches int64
	for _, row := range merged {
		row.Diff = row.DaxiNet - row.UpstreamNet
		row.Match = row.Diff == 0
		if !row.Match {
			mismatches++
		}
		totalDaxi += row.DaxiNet
		totalUp += row.UpstreamNet
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Diff != rows[j].Diff {
			return abs64(rows[i].Diff) > abs64(rows[j].Diff)
		}
		return rows[i].CustomerPublicID < rows[j].CustomerPublicID
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"unit":                  "quota",
		"window":                map[string]string{"start": unixStart, "end": unixEnd},
		"upstream_source":       source,
		"upstream_authoritative": authoritative,
		"total_daxi_net":        totalDaxi,
		"total_upstream_net":    totalUp,
		"total_diff":            totalDaxi - totalUp,
		"mismatch_count":        mismatches,
		"rows":                  rows,
	})
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
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
