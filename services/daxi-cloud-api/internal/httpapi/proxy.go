package httpapi

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"

	"daxi-cloud-api/internal/model"
	"daxi-cloud-api/internal/upstream"
)

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	rawKey := bearerToken(r)
	if rawKey == "" {
		writeError(w, http.StatusUnauthorized, "DAXI customer API key required")
		return
	}
	apiKey, customer, err := s.store.FindAPIKey(rawKey)
	if err != nil || !strings.EqualFold(apiKey.Status, "Active") || !strings.EqualFold(customer.Status, "Active") {
		writeError(w, http.StatusUnauthorized, "invalid or inactive DAXI customer API key")
		return
	}
	if s.cfg.EmergencyDisabled {
		writeError(w, http.StatusServiceUnavailable, "DAXI upstream is temporarily disabled")
		return
	}
	if strings.TrimSpace(s.cfg.UpstreamAPIKey) == "" {
		writeError(w, http.StatusServiceUnavailable, "upstream API key is not configured")
		return
	}

	models, err := s.upstream.ListModels(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, models)
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	rawKey := bearerToken(r)
	if rawKey == "" {
		writeError(w, http.StatusUnauthorized, "DAXI customer API key required")
		return
	}

	apiKey, customer, err := s.store.FindAPIKey(rawKey)
	if err != nil || !strings.EqualFold(apiKey.Status, "Active") || !strings.EqualFold(customer.Status, "Active") {
		writeError(w, http.StatusUnauthorized, "invalid or inactive DAXI customer API key")
		return
	}
	if customer.Balance <= 0 {
		writeError(w, http.StatusPaymentRequired, "DAXI customer token balance is empty")
		return
	}
	if s.cfg.EmergencyDisabled {
		writeError(w, http.StatusServiceUnavailable, "DAXI upstream is temporarily disabled")
		return
	}
	if strings.TrimSpace(s.cfg.UpstreamAPIKey) == "" {
		writeError(w, http.StatusServiceUnavailable, "upstream API key is not configured")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	payload := map[string]any{}
	_ = json.Unmarshal(body, &payload)
	modified := false

	modelName, _ := payload["model"].(string)
	scenario := r.Header.Get("X-DAXI-Scenario")
	if scenario == "" {
		scenario = r.Header.Get("X-Reseller-Scenario")
	}
	if scenario == "" {
		scenario = "model-api"
	}
	route, routeErr := s.store.GetModelRouteByScenario(scenario)
	if errors.Is(routeErr, sql.ErrNoRows) && scenario != "model-api" {
		// Unknown scenario values are treated as generic model API traffic instead
		// of silently bypassing route controls and falling back to the global allowlist.
		scenario = "model-api"
		route, routeErr = s.store.GetModelRouteByScenario(scenario)
	}
	if errors.Is(routeErr, sql.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "model-api route is not configured")
		return
	}
	if routeErr != nil && !errors.Is(routeErr, sql.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "failed to load model route")
		return
	}
	if routeErr == nil && !strings.EqualFold(route.Status, "Active") {
		writeError(w, http.StatusForbidden, "model route is disabled for this scenario")
		return
	}
	if modelName == "" {
		if routeErr == nil && route.PrimaryModel != "" {
			modelName = route.PrimaryModel
		} else {
			modelName = s.cfg.DefaultProxyModel
		}
		payload["model"] = modelName
		modified = true
	}
	upstreamModels, err := s.upstream.ListModels(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if !modelListedUpstream(modelName, upstreamModels) || !modelAllowedForScenario(modelName, route, routeErr == nil) {
		writeError(w, http.StatusForbidden, "model is not allowed for this DAXI scenario")
		return
	}

	// Enforce the per-scenario output cap when a route defines one.
	if routeErr == nil && route.MaxTokensPerRequest > 0 {
		for _, field := range []string{"max_tokens", "max_completion_tokens"} {
			if requested, ok := payloadInt(payload, field); ok && requested > route.MaxTokensPerRequest {
				writeError(w, http.StatusBadRequest, field+" exceeds scenario limit")
				return
			}
		}
	}

	// Force usage reporting on streaming requests so token settlement still
	// happens locally; otherwise stream responses would carry no usage and the
	// customer balance would never be deducted.
	stream, _ := payload["stream"].(bool)
	if stream {
		opts, _ := payload["stream_options"].(map[string]any)
		if opts == nil {
			opts = map[string]any{}
		}
		opts["include_usage"] = true
		payload["stream_options"] = opts
		modified = true
	}

	if modified {
		if body, err = json.Marshal(payload); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	resp, requestID, err := s.upstream.ProxyChat(r.Context(), body, upstream.Identity{
		CustomerPublicID: customer.PublicID,
		APIKeyPublicID:   apiKey.PublicID,
		Scenario:         scenario,
		RequestModel:     modelName,
	}, stream)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		if strings.EqualFold(key, "Content-Length") {
			continue
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.Header().Set("X-DAXI-Request-ID", requestID)
	w.WriteHeader(resp.StatusCode)

	var usage upstream.Usage
	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		usage = streamAndCapture(w, resp.Body)
	} else {
		buf := new(bytes.Buffer)
		_, _ = io.Copy(buf, resp.Body)
		_, _ = w.Write(buf.Bytes())
		usage = upstream.ParseUsage(buf.Bytes())
	}

	// chat 按上游 usage.quota 扣(含 quota=0 即扣 0,与 video 同单位)。
	// 上游已全渠道覆盖 quota 并生产验证,故移除原「quota 缺失→回退按 token 扣」兜底(那是错误单位、会多扣)。
	// 若 quota 字段异常缺失(上游回归),按 usage.Quota(0)扣并打 WARN 暴露 —— 不静默免单、不按错误单位多扣。
	// 注:balance_tokens→balance_quota 字段合并是单独的第二步,勿在此一锅烩。
	charge := usage.Quota
	if !usage.QuotaSet && usage.TotalTokens > 0 {
		log.Printf("WARN chat usage.quota absent (upstream should always return it; charged %d): customer=%s model=%s total_tokens=%d request_id=%s",
			charge, customer.PublicID, modelName, usage.TotalTokens, requestID)
	}
	overspend := int64(0)
	if charge > 0 {
		overspend, _ = s.store.DecreaseCustomerBalance(customer.ID, charge)
	}
	// total_tokens 记「实扣额度」:quota 计费时即 quota(与 video 同单位,便于对账合并);回退时为原始 total。
	_ = s.store.RecordUsage(model.UsageRecord{
		RequestID:        requestID,
		CustomerID:       customer.ID,
		APIKeyID:         apiKey.ID,
		Scenario:         scenario,
		Model:            modelName,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      charge,
		OverspendTokens:  overspend,
		StatusCode:       resp.StatusCode,
	})
}

// streamAndCapture relays an SSE response to the client in real time (flushing
// every chunk) while scanning for the usage chunk emitted by include_usage.
func streamAndCapture(w http.ResponseWriter, body io.Reader) upstream.Usage {
	flusher, _ := w.(http.Flusher)
	reader := bufio.NewReader(body)
	var usage upstream.Usage
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			_, _ = w.Write(line)
			if flusher != nil {
				flusher.Flush()
			}
			if u, ok := upstream.ParseStreamUsage(line); ok {
				usage = u
			}
		}
		if err != nil {
			break
		}
	}
	return usage
}

func modelListedUpstream(modelName string, models upstream.ModelList) bool {
	if modelName == "" {
		return false
	}
	for _, item := range models.Data {
		if item.ID == modelName {
			return true
		}
	}
	return false
}

func modelAllowedForScenario(modelName string, route model.ModelRoute, hasRoute bool) bool {
	if modelName == "" {
		return false
	}
	if !hasRoute {
		return true
	}
	if route.Scenario == "model-api" {
		return true
	}
	if route.PrimaryModel == modelName {
		return true
	}
	for _, fallback := range strings.Split(route.FallbackModels, ",") {
		if strings.TrimSpace(fallback) == modelName {
			return true
		}
	}
	return false
}

// payloadInt reads a numeric field from a decoded JSON object. Numbers decoded
// into map[string]any arrive as float64.
func payloadInt(payload map[string]any, key string) (int64, bool) {
	switch n := payload[key].(type) {
	case float64:
		return int64(n), true
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, false
		}
		return i, true
	default:
		return 0, false
	}
}

func bearerToken(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(value), "bearer ") {
		return strings.TrimSpace(value[7:])
	}
	return ""
}
