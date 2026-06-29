package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
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

// decodeUpstream 拆上游统一信封 {success,message,data}(common/gin.go ApiSuccess)。
// success=false 返回带 message 的错误;无信封(老/测试)时原样返回顶层 map。
func decodeUpstream(body []byte) (map[string]any, error) {
	var env map[string]any
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("invalid upstream response")
	}
	if s, ok := env["success"].(bool); ok {
		if !s {
			msg, _ := env["message"].(string)
			if strings.TrimSpace(msg) == "" {
				msg = "upstream rejected request"
			}
			return nil, fmt.Errorf("%s", msg)
		}
		if d, ok := env["data"].(map[string]any); ok {
			return d, nil
		}
		return map[string]any{}, nil
	}
	return env, nil
}

func subMap(m map[string]any, key string) map[string]any {
	if v, ok := m[key].(map[string]any); ok {
		return v
	}
	return nil
}

func jsonStr(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// jsonIDString 读 id:上游 draft.id 是数字,task_id 是字符串,统一成 string。
func jsonIDString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		switch v := m[k].(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				return v
			}
		case float64:
			if v != 0 {
				return strconv.FormatInt(int64(v), 10)
			}
		}
	}
	return ""
}

func quotaInt(m map[string]any, keys ...string) (int64, bool) {
	for _, k := range keys {
		if v, ok := m[k].(float64); ok {
			return int64(v), true
		}
	}
	return 0, false
}

// taskEstimatedQuota 从建任务响应 data 提预扣 quota:data.data.cost_snapshot.estimated_quota(真实路径),含回退。
func taskEstimatedQuota(data map[string]any) (int64, bool) {
	if cs := subMap(subMap(data, "data"), "cost_snapshot"); cs != nil {
		if q, ok := quotaInt(cs, "estimated_quota"); ok {
			return q, true
		}
	}
	if cs := subMap(data, "cost_snapshot"); cs != nil {
		if q, ok := quotaInt(cs, "estimated_quota"); ok {
			return q, true
		}
	}
	return quotaInt(data, "estimated_quota")
}

// percentInt 解析 progress:真实是 "100%" 字符串,也容忍数字。
func percentInt(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case string:
		s := strings.TrimSuffix(strings.TrimSpace(v), "%")
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}
	}
	return 0
}

// POST /v1/video-agent/drafts/generate —— draft 不计费(上游 estimate.is_billable=false),仅生成脚本 + 建映射。
func (s *Server) handleVideoAgentGenerateDraft(w http.ResponseWriter, r *http.Request) {
	apiKey, customer, ok := s.authVideoCustomer(w, r)
	if !ok {
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
		writeJSON(w, http.StatusOK, map[string]any{"draft_id": draft.PublicID, "scenario": draft.Scenario})
		return
	}
	respBody, _, ok := s.callUpstream(w, r, http.MethodPost, "/agents/video/drafts/generate", body, videoIdentity(apiKey, customer))
	if !ok {
		return
	}
	data, err := decodeUpstream(respBody)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	// 真实形态:data.draft.id(数字)。draft 不计费,仅取 id 建映射。
	upstreamDraftID := jsonIDString(subMap(data, "draft"), "id", "draft_id")
	if strings.TrimSpace(upstreamDraftID) == "" {
		writeError(w, http.StatusBadGateway, "missing upstream draft id")
		return
	}
	draft, _, err := s.store.UpsertVideoAgentDraft(customer.ID, apiKey.ID, scenario, upstreamDraftID, idem)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to persist draft")
		return
	}
	// 只回客户需要的:draft_id + scenario + 生成的 script(不回 estimate 货币/成本字段,不回上游内部 id)。
	out := map[string]any{"draft_id": draft.PublicID, "scenario": scenario}
	if script := data["script"]; script != nil {
		out["script"] = script
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
	data, err := decodeUpstream(respBody)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	out := map[string]any{"draft_id": draft.PublicID, "scenario": draft.Scenario}
	if script := data["script"]; script != nil {
		out["script"] = script
	}
	writeJSON(w, http.StatusOK, out)
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
	if _, err := decodeUpstream(respBody); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	// draft 生命周期状态(draft/confirmed),非任务枚举;确认成功统一回 confirmed。
	writeJSON(w, http.StatusOK, map[string]any{"draft_id": draft.PublicID, "status": "confirmed"})
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
	data, err := decodeUpstream(respBody)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	// 真实形态:任务 id 用 string task_id(不是数字 id)。
	upstreamTaskID := jsonStr(data, "task_id")
	if strings.TrimSpace(upstreamTaskID) == "" {
		writeError(w, http.StatusBadGateway, "missing upstream task id")
		return
	}
	// 预扣额度取真实 cost_snapshot.estimated_quota;缺失回退到配置占位。
	estimatedQuota := s.cfg.VideoAgentEstTaskQuota
	if q, ok := taskEstimatedQuota(data); ok && q > 0 {
		estimatedQuota = q
	}
	task := &model.VideoTask{
		CustomerPublicID:  customer.PublicID,
		CustomerID:        customer.ID,
		APIKeyID:          apiKey.ID,
		APIKeyPublicID:    apiKey.PublicID,
		Scenario:          defaultScenario(draft.Scenario),
		UpstreamTaskID:    upstreamTaskID,
		UpstreamRequestID: requestID,
		EstimatedQuota:    estimatedQuota,
		Status:            mapUpstreamStatus(jsonStr(data, "status")),
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
	m, err := decodeUpstream(respBody)
	if err != nil {
		return nil, err
	}
	daxiStatus := mapUpstreamStatus(jsonStr(m, "status"))
	progress := percentInt(m, "progress")
	// 真实实扣在终态任务的 data.quota(= task.Quota,多退少补后);仅 Completed 时透传 result_url(失败时该字段会被塞 fail_reason)。
	resultURL := ""
	if daxiStatus == "Completed" {
		resultURL = jsonStr(m, "result_url", "final_video_url")
	}
	quota, found := quotaInt(m, "quota", "actual_quota")
	if quota <= 0 {
		found = false
	}

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
	// 终片未就绪时上游回 success=false("final video is not ready"),HTTP 200 —— 不是错误,回 ready:false。
	data, err := decodeUpstream(respBody)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"task_id": task.PublicID, "ready": false})
		return
	}
	// 本期允许透传 supchuang 域名(两边自营)。
	url := jsonStr(data, "result_url", "final_video_url", "url")
	writeJSON(w, http.StatusOK, map[string]any{"task_id": task.PublicID, "ready": url != "", "result_url": url})
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

