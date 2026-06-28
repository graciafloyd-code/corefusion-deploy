package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"daxi-cloud-api/internal/model"
	"daxi-cloud-api/internal/upstream"
)

// mapUpstreamStatus 把上游富 agent 的 raw 状态枚举映射为 DAXI 五枚举,未知保守为 Running,
// 绝不把 raw 透出给客户(单点函数)。
func mapUpstreamStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "queued", "not_start", "submitted", "pending":
		return "Queued"
	case "in_progress", "running", "processing":
		return "Running"
	case "success", "succeeded", "completed", "done":
		return "Completed"
	case "failure", "failed", "error":
		return "Failed"
	case "canceled", "cancelled":
		return "Canceled"
	default:
		return "Running"
	}
}

// authVideoCustomer 复刻 proxy.go:52-56 的四查(key Active / customer Active / emergency / upstream key),
// 余额(balance_quota)在 create 处单独检查。
func (s *Server) authVideoCustomer(w http.ResponseWriter, r *http.Request) (model.APIKey, model.Customer, bool) {
	rawKey := bearerToken(r)
	if rawKey == "" {
		writeError(w, http.StatusUnauthorized, "DAXI customer API key required")
		return model.APIKey{}, model.Customer{}, false
	}
	apiKey, customer, err := s.store.FindAPIKey(rawKey)
	if err != nil || !strings.EqualFold(apiKey.Status, "Active") || !strings.EqualFold(customer.Status, "Active") {
		writeError(w, http.StatusUnauthorized, "invalid or inactive DAXI customer API key")
		return model.APIKey{}, model.Customer{}, false
	}
	if s.cfg.EmergencyDisabled {
		writeError(w, http.StatusServiceUnavailable, "DAXI upstream is temporarily disabled")
		return model.APIKey{}, model.Customer{}, false
	}
	if strings.TrimSpace(s.cfg.UpstreamAPIKey) == "" {
		writeError(w, http.StatusServiceUnavailable, "upstream API key is not configured")
		return model.APIKey{}, model.Customer{}, false
	}
	return apiKey, customer, true
}

func videoIdentity(apiKey model.APIKey, customer model.Customer) upstream.Identity {
	return upstream.Identity{CustomerPublicID: customer.PublicID, APIKeyPublicID: apiKey.PublicID, Scenario: "video-agent"}
}

func videoTaskIdentity(task model.VideoTask, apiKey model.APIKey, customer model.Customer) upstream.Identity {
	customerPublicID := task.CustomerPublicID
	if strings.TrimSpace(customerPublicID) == "" {
		customerPublicID = customer.PublicID
	}
	apiKeyPublicID := task.APIKeyPublicID
	if strings.TrimSpace(apiKeyPublicID) == "" {
		apiKeyPublicID = apiKey.PublicID
	}
	return upstream.Identity{CustomerPublicID: customerPublicID, APIKeyPublicID: apiKeyPublicID, Scenario: "video-agent"}
}

func videoScenario(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("X-DAXI-Scenario")); v != "" {
		return v
	}
	return "video-agent"
}

func videoBillingEventKey(kind, upstreamID string) (string, error) {
	upstreamID = strings.TrimSpace(upstreamID)
	if upstreamID == "" {
		return "", fmt.Errorf("missing upstream %s id", kind)
	}
	return kind + ":" + upstreamID, nil
}

// callUpstream 转发并读回响应体(含 MaxBody 上限);非 2xx 时按原状态码透传、不泄露上游 body。
func (s *Server) callUpstream(w http.ResponseWriter, r *http.Request, method, path string, body []byte, id upstream.Identity) ([]byte, string, bool) {
	resp, requestID, err := s.upstream.CallVideoAgent(r.Context(), method, path, body, id)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return nil, requestID, false
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeError(w, resp.StatusCode, "upstream video-agent error")
		return nil, requestID, false
	}
	return respBody, requestID, true
}

func decodeMap(body []byte) map[string]any {
	m := map[string]any{}
	_ = json.Unmarshal(body, &m)
	return m
}

func jsonStr(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func jsonInt(m map[string]any, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return 0
}

// POST /v1/video-agent/drafts/generate —— 同步,生成即结算 draft 计费事件。
func (s *Server) handleVideoAgentGenerateDraft(w http.ResponseWriter, r *http.Request) {
	apiKey, customer, ok := s.authVideoCustomer(w, r)
	if !ok {
		return
	}
	if customer.BalanceQuota < s.cfg.VideoAgentEstDraftQuota {
		writeError(w, http.StatusPaymentRequired, "insufficient video quota balance")
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	scenario := videoScenario(r)
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if draft, found, err := s.store.FindVideoAgentDraftByIdempotency(customer.ID, apiKey.ID, idem); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check draft idempotency")
		return
	} else if found {
		writeJSON(w, http.StatusOK, map[string]any{"draft_id": draft.PublicID, "scenario": draft.Scenario, "usage": map[string]any{"quota": int64(0)}})
		return
	}
	respBody, _, ok := s.callUpstream(w, r, http.MethodPost, "/agents/video/drafts/generate", body, videoIdentity(apiKey, customer))
	if !ok {
		return
	}
	m := decodeMap(respBody)
	upstreamDraftID := jsonStr(m, "id", "draft_id")
	draftEventKey, err := videoBillingEventKey("draft", upstreamDraftID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	draft, _, err := s.store.UpsertVideoAgentDraft(customer.ID, apiKey.ID, scenario, upstreamDraftID, idem)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to persist draft")
		return
	}
	settledQuota := int64(0)
	if quota, found := upstream.ParseQuotaUsage(respBody); found && quota > 0 {
		if _, _, _, err := s.store.SettleVideoBillingEvent(draftEventKey, customer.ID, apiKey.ID, scenario, quota, false); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to settle draft usage")
			return
		}
		settledQuota = quota
	}
	out := map[string]any{
		"draft_id": draft.PublicID,
		"scenario": scenario,
		"draft":    sanitizeUpstream(m),
		"usage":    map[string]any{"quota": settledQuota},
	}
	writeJSON(w, http.StatusOK, out)
}

// GET /v1/video-agent/drafts/{id}
func (s *Server) handleVideoAgentGetDraft(w http.ResponseWriter, r *http.Request) {
	apiKey, customer, ok := s.authVideoCustomer(w, r)
	if !ok {
		return
	}
	draft, err := s.store.GetVideoAgentDraftForCustomer(r.PathValue("id"), customer.ID)
	if err != nil {
		writeError(w, http.StatusNotFound, "draft not found")
		return
	}
	respBody, _, ok := s.callUpstream(w, r, http.MethodGet, "/agents/video/drafts/"+draft.UpstreamDraftID, nil, videoIdentity(apiKey, customer))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"draft_id": draft.PublicID, "scenario": draft.Scenario, "draft": sanitizeUpstream(decodeMap(respBody))})
}

// POST /v1/video-agent/drafts/{id}/confirm
func (s *Server) handleVideoAgentConfirmDraft(w http.ResponseWriter, r *http.Request) {
	apiKey, customer, ok := s.authVideoCustomer(w, r)
	if !ok {
		return
	}
	draft, err := s.store.GetVideoAgentDraftForCustomer(r.PathValue("id"), customer.ID)
	if err != nil {
		writeError(w, http.StatusNotFound, "draft not found")
		return
	}
	body, _ := io.ReadAll(r.Body)
	respBody, _, ok := s.callUpstream(w, r, http.MethodPost, "/agents/video/drafts/"+draft.UpstreamDraftID+"/confirm", body, videoIdentity(apiKey, customer))
	if !ok {
		return
	}
	m := decodeMap(respBody)
	writeJSON(w, http.StatusOK, map[string]any{"draft_id": draft.PublicID, "status": mapUpstreamStatus(jsonStr(m, "status"))})
}

// POST /v1/video-agent/drafts/{id}/tasks —— 创建视频任务(异步,结算在终态 GET)。
func (s *Server) handleVideoAgentCreateTask(w http.ResponseWriter, r *http.Request) {
	apiKey, customer, ok := s.authVideoCustomer(w, r)
	if !ok {
		return
	}
	if customer.BalanceQuota < s.cfg.VideoAgentEstTaskQuota {
		writeError(w, http.StatusPaymentRequired, "insufficient video quota balance")
		return
	}
	draft, err := s.store.GetVideoAgentDraftForCustomer(r.PathValue("id"), customer.ID)
	if err != nil {
		writeError(w, http.StatusNotFound, "draft not found")
		return
	}
	body, _ := io.ReadAll(r.Body)
	respBody, requestID, ok := s.callUpstream(w, r, http.MethodPost, "/agents/video/drafts/"+draft.UpstreamDraftID+"/tasks", body, videoIdentity(apiKey, customer))
	if !ok {
		return
	}
	m := decodeMap(respBody)
	upstreamTaskID := jsonStr(m, "id", "task_id")
	if _, err := videoBillingEventKey("task", upstreamTaskID); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	task := &model.VideoTask{
		CustomerPublicID:  customer.PublicID,
		CustomerID:        customer.ID,
		APIKeyID:          apiKey.ID,
		APIKeyPublicID:    apiKey.PublicID,
		Scenario:          defaultScenario(draft.Scenario),
		UpstreamTaskID:    upstreamTaskID,
		UpstreamRequestID: requestID,
		EstimatedQuota:    s.cfg.VideoAgentEstTaskQuota,
		Status:            mapUpstreamStatus(jsonStr(m, "status")),
	}
	if err := s.store.CreateVideoAgentTaskMapping(task); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to persist task")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"task_id": task.PublicID, "status": task.Status, "scenario": task.Scenario, "estimated_quota": task.EstimatedQuota})
}

// GET /v1/video-agent/tasks —— 仅本客户。
func (s *Server) handleVideoAgentListTasks(w http.ResponseWriter, r *http.Request) {
	_, customer, ok := s.authVideoCustomer(w, r)
	if !ok {
		return
	}
	tasks, err := s.store.ListVideoAgentTasksForCustomer(customer.ID, parseLimit(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tasks")
		return
	}
	data := make([]map[string]any, 0, len(tasks))
	for _, t := range tasks {
		data = append(data, map[string]any{"task_id": t.PublicID, "status": t.Status, "scenario": t.Scenario, "progress": t.Progress, "result_url": t.ResultURL, "created_at": t.CreatedAt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

// GET /v1/video-agent/tasks/{id} —— 透传上游状态 + 终态结算(含无 usage 兜底)。
func (s *Server) handleVideoAgentGetTask(w http.ResponseWriter, r *http.Request) {
	apiKey, customer, ok := s.authVideoCustomer(w, r)
	if !ok {
		return
	}
	task, err := s.store.GetVideoAgentTaskForCustomer(r.PathValue("id"), customer.ID)
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	out, err := s.refreshAndSettleVideoAgentTask(r.Context(), task, videoTaskIdentity(task, apiKey, customer))
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) refreshAndSettleVideoAgentTask(ctx context.Context, task model.VideoTask, id upstream.Identity) (map[string]any, error) {
	if _, err := videoBillingEventKey("task", task.UpstreamTaskID); err != nil {
		return nil, err
	}
	resp, _, err := s.upstream.CallVideoAgent(ctx, http.MethodGet, "/agents/video/tasks/"+task.UpstreamTaskID, nil, id)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upstream video-agent returned %s", resp.Status)
	}
	return s.settleVideoAgentTaskFromResponse(task, respBody)
}

func (s *Server) settleVideoAgentTaskFromResponse(task model.VideoTask, respBody []byte) (map[string]any, error) {
	m := decodeMap(respBody)
	daxiStatus := mapUpstreamStatus(jsonStr(m, "status"))
	progress := jsonInt(m, "progress")
	resultURL := jsonStr(m, "result_url", "final_video_url")
	quota, found := upstream.ParseQuotaUsage(respBody)

	settledQuota := int64(0)
	usageRecordID := int64(0)
	needsReview := task.NeedsReview
	eventKey, err := videoBillingEventKey("task", task.UpstreamTaskID)
	if err != nil {
		return nil, err
	}
	switch daxiStatus {
	case "Completed":
		if found && quota > 0 {
			recID, _, _, err := s.store.SettleVideoBillingEvent(eventKey, task.CustomerID, task.APIKeyID, defaultScenario(task.Scenario), quota, false)
			if err != nil {
				return nil, err
			}
			settledQuota = quota
			usageRecordID = recID
		} else {
			// 终态无 usage 兜底:按预估结算 + needs_review,绝不静默不扣。
			recID, _, _, err := s.store.SettleVideoBillingEvent(eventKey, task.CustomerID, task.APIKeyID, defaultScenario(task.Scenario), task.EstimatedQuota, true)
			if err != nil {
				return nil, err
			}
			settledQuota = task.EstimatedQuota
			usageRecordID = recID
			needsReview = true
		}
	case "Failed":
		if found && quota > 0 {
			recID, _, _, err := s.store.SettleVideoBillingEvent(eventKey, task.CustomerID, task.APIKeyID, defaultScenario(task.Scenario), quota, false)
			if err != nil {
				return nil, err
			}
			settledQuota = quota
			usageRecordID = recID
		}
	}
	// 回链 usage_record_id(仅本次真正结算时 recID>0;重试已结算返回 0,UpdateVideoAgentTaskState 内 CASE 保留原值)。
	if err := s.store.UpdateVideoAgentTaskState(task.PublicID, daxiStatus, progress, resultURL, usageRecordID, needsReview); err != nil {
		return nil, err
	}

	out := map[string]any{"task_id": task.PublicID, "status": daxiStatus, "progress": progress, "result_url": resultURL}
	if settledQuota > 0 || found {
		out["usage"] = map[string]any{"quota": settledQuota}
	}
	return out, nil
}

// GET /v1/video-agent/tasks/{id}/final-video
func (s *Server) handleVideoAgentFinalVideo(w http.ResponseWriter, r *http.Request) {
	apiKey, customer, ok := s.authVideoCustomer(w, r)
	if !ok {
		return
	}
	task, err := s.store.GetVideoAgentTaskForCustomer(r.PathValue("id"), customer.ID)
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	respBody, _, ok := s.callUpstream(w, r, http.MethodGet, "/agents/video/tasks/"+task.UpstreamTaskID+"/final-video", nil, videoIdentity(apiKey, customer))
	if !ok {
		return
	}
	m := decodeMap(respBody)
	// 本期允许透传 supchuang 域名(两边自营)。
	writeJSON(w, http.StatusOK, map[string]any{"task_id": task.PublicID, "result_url": jsonStr(m, "result_url", "final_video_url", "url")})
}

func defaultScenario(s string) string {
	if strings.TrimSpace(s) == "" {
		return "video-agent"
	}
	return s
}

func (s *Server) settleVideoAgentTasksLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		after := time.Duration(s.cfg.VideoAgentSettleAfterSecs) * time.Second
		if after <= 0 {
			after = 5 * time.Minute
		}
		if err := s.settlePendingVideoAgentTasksOnce(time.Now().UTC().Add(-after)); err != nil {
			log.Printf("video-agent compensation settle failed: %v", err)
		}
	}
}

func (s *Server) settlePendingVideoAgentTasksOnce(cutoff time.Time) error {
	tasks, err := s.store.ListUnsettledVideoAgentTasks(cutoff, 50)
	if err != nil {
		return err
	}
	var firstErr error
	for _, task := range tasks {
		id := upstream.Identity{CustomerPublicID: task.CustomerPublicID, APIKeyPublicID: task.APIKeyPublicID, Scenario: "video-agent"}
		if _, err := s.refreshAndSettleVideoAgentTask(context.Background(), task, id); err != nil {
			log.Printf("video-agent compensation failed for %s/%s: %v", task.PublicID, task.UpstreamTaskID, err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// sanitizeUpstream 把上游 map 里若存在的 raw 状态枚举映射成 DAXI 枚举,避免漏给客户。
func sanitizeUpstream(m map[string]any) map[string]any {
	if raw, ok := m["status"].(string); ok {
		m["status"] = mapUpstreamStatus(raw)
	}
	return m
}
