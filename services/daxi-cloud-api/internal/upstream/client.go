package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"daxi-cloud-api/internal/config"
)

type Client struct {
	cfg          config.Config
	client       *http.Client
	streamClient *http.Client
}

type Usage struct {
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64
	// Quota 是上游实扣 quota(对齐 video 的 usage.quota)。
	Quota int64
	// QuotaSet 标识响应里 usage.quota 字段是否【存在】(哪怕值为 0)。
	// 关键:不能靠 Quota==0 判断 —— quota:0(存在且为0)与无 quota 字段在 int 零值上无法区分。
	// QuotaSet=true 即上游显式给了 quota(含 0)→ 按 quota 扣;false=字段缺失 → 防御式回退按 token 扣。
	QuotaSet bool
}

type ModelItem struct {
	ID      string         `json:"id"`
	Object  string         `json:"object,omitempty"`
	OwnedBy string         `json:"owned_by,omitempty"`
	Extra   map[string]any `json:"-"`
}

type ModelList struct {
	Object string      `json:"object"`
	Data   []ModelItem `json:"data"`
}

type Identity struct {
	CustomerPublicID string
	APIKeyPublicID   string
	Scenario         string
	RequestModel     string
}

func NewClient(cfg config.Config) *Client {
	timeout := 120 * time.Second
	if cfg.RequestTimeoutSecs > 0 {
		timeout = time.Duration(cfg.RequestTimeoutSecs) * time.Second
	}
	return &Client{
		cfg:    cfg,
		client: &http.Client{Timeout: timeout},
		// Streaming responses can outlive a fixed client timeout; rely on the
		// request context (client disconnect / server write deadline) instead.
		streamClient: &http.Client{Timeout: 0},
	}
}

func NewClientWithTransport(cfg config.Config, transport http.RoundTripper) *Client {
	client := NewClient(cfg)
	if transport != nil {
		client.client.Transport = transport
		client.streamClient.Transport = transport
	}
	return client
}

// ProxyChat forwards the chat completion request to the master platform and
// returns the raw response. The caller is responsible for streaming/copying the
// body and parsing usage (see ParseUsage / ParseStreamUsage).
func (c *Client) ProxyChat(ctx context.Context, body []byte, identity Identity, stream bool) (*http.Response, string, error) {
	requestID := "req-" + time.Now().UTC().Format("20060102150405.000000000")
	url := c.cfg.UpstreamBaseURL + "/chat/completions"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, requestID, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.UpstreamAPIKey)
	req.Header.Set("X-Reseller-Code", c.cfg.ResellerCode)
	req.Header.Set("X-Reseller-Customer-ID", identity.CustomerPublicID)
	req.Header.Set("X-Reseller-Key-ID", identity.APIKeyPublicID)
	req.Header.Set("X-Reseller-Scenario", defaultString(identity.Scenario, "model-api"))
	req.Header.Set("X-Reseller-Request-ID", requestID)
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}

	httpClient := c.client
	if stream {
		httpClient = c.streamClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, requestID, err
	}
	return resp, requestID, nil
}

// CallVideoAgent 转发一次富 video agent 调用到上游 /api/agents/video/* (Bearer 上游 key),
// 强制注入 X-Reseller-* 归因头(沿用 ProxyChat 的注入模式)。upstreamPath 形如
// "/agents/video/drafts/generate"。返回原始响应 + 本次生成的 requestID(= X-Reseller-Request-ID)。
func (c *Client) CallVideoAgent(ctx context.Context, method, upstreamPath string, body []byte, identity Identity) (*http.Response, string, error) {
	requestID := "req-" + time.Now().UTC().Format("20060102150405.000000000")
	base := strings.TrimSuffix(c.cfg.UpstreamBaseURL, "/v1")
	if c.cfg.VideoAgentBaseURL != "" {
		base = strings.TrimRight(c.cfg.VideoAgentBaseURL, "/")
	}
	url := base + "/api" + upstreamPath

	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, requestID, err
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.UpstreamAPIKey)
	req.Header.Set("X-Reseller-Code", c.cfg.ResellerCode)
	req.Header.Set("X-Reseller-Customer-ID", identity.CustomerPublicID)
	req.Header.Set("X-Reseller-Key-ID", identity.APIKeyPublicID)
	req.Header.Set("X-Reseller-Scenario", defaultString(identity.Scenario, "video-agent"))
	req.Header.Set("X-Reseller-Request-ID", requestID)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, requestID, err
	}
	return resp, requestID, nil
}

// ParseQuotaUsage 从富 agent 响应体提取 usage.quota(整数 quota 单位)。
// 返回 (quota, found);found=false 表示响应未带 usage.quota(终态兜底场景)。
func ParseQuotaUsage(body []byte) (int64, bool) {
	var payload struct {
		Usage *struct {
			Quota *int64 `json:"quota"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.Usage == nil || payload.Usage.Quota == nil {
		return 0, false
	}
	return *payload.Usage.Quota, true
}

func (c *Client) ListModels(ctx context.Context) (ModelList, error) {
	url := c.cfg.UpstreamBaseURL + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ModelList{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.UpstreamAPIKey)
	req.Header.Set("X-Reseller-Code", c.cfg.ResellerCode)

	resp, err := c.client.Do(req)
	if err != nil {
		return ModelList{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ModelList{}, fmt.Errorf("upstream models returned %s", resp.Status)
	}

	var payload struct {
		Object string           `json:"object"`
		Data   []map[string]any `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return ModelList{}, err
	}
	out := ModelList{Object: defaultString(payload.Object, "list")}
	for _, raw := range payload.Data {
		id, _ := raw["id"].(string)
		if strings.TrimSpace(id) == "" {
			continue
		}
		item := ModelItem{ID: id}
		if object, _ := raw["object"].(string); object != "" {
			item.Object = object
		}
		if ownedBy, _ := raw["owned_by"].(string); ownedBy != "" {
			item.OwnedBy = ownedBy
		}
		out.Data = append(out.Data, item)
	}
	return out, nil
}

// GetResellerConsumeLogs 拉取本 reseller token(= UpstreamAPIKey)的最近用量日志(上游 /api/log/token,TokenAuthReadOnly)。
// ⚠️ 上游该端点只回最近 MaxRecentItems(=1000)条、无时间窗/分页/聚合 —— 仅供冒烟核对,不可作正式对账依据。
func (c *Client) GetResellerConsumeLogs(ctx context.Context) ([]byte, error) {
	base := strings.TrimSuffix(c.cfg.UpstreamBaseURL, "/v1")
	if c.cfg.VideoAgentBaseURL != "" {
		base = strings.TrimRight(c.cfg.VideoAgentBaseURL, "/")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/log/token", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.UpstreamAPIKey)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upstream log/token returned %s", resp.Status)
	}
	return body, nil
}

// GetResellerUsageSummary 调上游正式对账端点 GET /api/agents/video/reseller/usage-summary(TokenAuthReadOnly,
// token+reseller 自我隔离;按 customer_id 聚合 type=2 消费/type=6 退款净额)。这是对账 Query B 的权威源。
func (c *Client) GetResellerUsageSummary(ctx context.Context, start, end, scenario string) ([]byte, error) {
	base := strings.TrimSuffix(c.cfg.UpstreamBaseURL, "/v1")
	if c.cfg.VideoAgentBaseURL != "" {
		base = strings.TrimRight(c.cfg.VideoAgentBaseURL, "/")
	}
	q := url.Values{}
	if start != "" {
		q.Set("start", start)
	}
	if end != "" {
		q.Set("end", end)
	}
	if scenario != "" {
		q.Set("scenario", scenario)
	}
	q.Set("group_by", "customer")
	u := base + "/api/agents/video/reseller/usage-summary?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.UpstreamAPIKey)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upstream usage-summary returned %s", resp.Status)
	}
	return body, nil
}

// ParseUsage extracts token usage from a non-streaming JSON chat completion body.
func ParseUsage(body []byte) Usage {
	var payload struct {
		Usage struct {
			PromptTokens     int64  `json:"prompt_tokens"`
			CompletionTokens int64  `json:"completion_tokens"`
			TotalTokens      int64  `json:"total_tokens"`
			Quota            *int64 `json:"quota"` // 指针:nil=字段缺失,非nil=存在(含0)
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return Usage{}
	}
	u := Usage{
		PromptTokens:     payload.Usage.PromptTokens,
		CompletionTokens: payload.Usage.CompletionTokens,
		TotalTokens:      payload.Usage.TotalTokens,
	}
	if payload.Usage.Quota != nil {
		u.Quota = *payload.Usage.Quota
		u.QuotaSet = true
	}
	return u
}

// ParseStreamUsage extracts usage from a single SSE line when it carries a usage
// object. With stream_options.include_usage=true the upstream emits a final
// chunk whose only meaningful field is usage.
func ParseStreamUsage(line []byte) (Usage, bool) {
	trimmed := bytes.TrimSpace(line)
	if !bytes.HasPrefix(trimmed, []byte("data:")) {
		return Usage{}, false
	}
	data := bytes.TrimSpace(trimmed[len("data:"):])
	if len(data) == 0 || bytes.Equal(data, []byte("[DONE]")) {
		return Usage{}, false
	}
	var payload struct {
		Usage *struct {
			PromptTokens     int64  `json:"prompt_tokens"`
			CompletionTokens int64  `json:"completion_tokens"`
			TotalTokens      int64  `json:"total_tokens"`
			Quota            *int64 `json:"quota"` // 指针:nil=字段缺失,非nil=存在(含0)
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &payload); err != nil || payload.Usage == nil {
		return Usage{}, false
	}
	u := Usage{
		PromptTokens:     payload.Usage.PromptTokens,
		CompletionTokens: payload.Usage.CompletionTokens,
		TotalTokens:      payload.Usage.TotalTokens,
	}
	if payload.Usage.Quota != nil {
		u.Quota = *payload.Usage.Quota
		u.QuotaSet = true
	}
	return u, true
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func (c *Client) String() string {
	return fmt.Sprintf("upstream(%s)", c.cfg.UpstreamBaseURL)
}
