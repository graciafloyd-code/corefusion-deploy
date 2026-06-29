package db

import "daxi-cloud-api/internal/model"

// VideoReconByCustomer 是视频对账 Query A:从 DAXI 自有 usage_records 按 customer_id 出每客户净额。
// 仅统计视频任务计费事件(upstream_event_key LIKE 'task:%'),排除 chat/模型代理的用量。
// total_tokens 存的是上游 actual_quota 全额(超余额部分另记 overspend_tokens),即 DAXI 记录的视频消费净额。
// start/end 为 RFC3339 字符串(空则不限);与 usage_records.created_at(DATETIME)直接比较。
func (s *Store) VideoReconByCustomer(start, end string) ([]model.VideoReconDaxiRow, error) {
	where := "WHERE u.upstream_event_key LIKE 'task:%'"
	args := []any{}
	if start != "" {
		where += " AND u.created_at >= ?"
		args = append(args, start)
	}
	if end != "" {
		where += " AND u.created_at < ?"
		args = append(args, end)
	}
	query := `SELECT u.customer_id, c.public_id, c.company,
			COALESCE(SUM(u.total_tokens), 0), COALESCE(SUM(u.overspend_tokens), 0),
			COUNT(*), COALESCE(SUM(u.needs_review), 0)
		FROM usage_records u JOIN customers c ON c.id = u.customer_id ` + where + `
		GROUP BY u.customer_id, c.public_id, c.company
		ORDER BY SUM(u.total_tokens) DESC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.VideoReconDaxiRow{}
	for rows.Next() {
		var r model.VideoReconDaxiRow
		if err := rows.Scan(&r.CustomerID, &r.CustomerPublicID, &r.Company, &r.NetQuota, &r.OverspendQuota, &r.Records, &r.NeedsReview); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
