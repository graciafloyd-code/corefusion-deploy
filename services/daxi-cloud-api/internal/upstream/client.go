package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

// ParseUsage extracts token usage from a non-streaming JSON chat completion body.
func ParseUsage(body []byte) Usage {
	var payload struct {
		Usage struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
			TotalTokens      int64 `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return Usage{}
	}
	return Usage{
		PromptTokens:     payload.Usage.PromptTokens,
		CompletionTokens: payload.Usage.CompletionTokens,
		TotalTokens:      payload.Usage.TotalTokens,
	}
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
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
			TotalTokens      int64 `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &payload); err != nil || payload.Usage == nil {
		return Usage{}, false
	}
	return Usage{
		PromptTokens:     payload.Usage.PromptTokens,
		CompletionTokens: payload.Usage.CompletionTokens,
		TotalTokens:      payload.Usage.TotalTokens,
	}, true
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
