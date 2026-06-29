package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Addr               string
	DatabasePath       string
	AdminToken         string
	AdminUsername      string
	AdminPassword      string
	UpstreamBaseURL    string
	UpstreamAPIKey     string
	ResellerCode       string
	EmergencyDisabled  bool
	AllowedModels      []string
	DefaultProxyModel  string
	RequestTimeoutSecs int
	MaxBodyBytes       int64
	// video-agent 富 agent 上游基址(默认从 UpstreamBaseURL 去掉 /v1,因 agent 在 /api/agents/video/* 根路径)。
	VideoAgentBaseURL string
	// 预估 quota:真实契约下 draft 不计费(EstDraftQuota 已废弃保留);建任务的预扣以上游 cost_snapshot.estimated_quota 为准,
	// EstTaskQuota 仅作建任务前的余额闸阈值 + 上游缺失时的回退。默认 3000000:单点数据(仅实测 1 个 15s 任务,
	// 估价 615000 但实扣 actual_quota=2812500),跑几个不同时长任务后校准,后续考虑按时长动态算。
	VideoAgentEstDraftQuota int64 // 已废弃:draft 不计费(P1 实测 is_billable=false)
	VideoAgentEstTaskQuota  int64 // 建任务余额闸阈值 + 回退
	// 后台补偿结算:客户不轮询时,定期主动查上游终态并结算。
	VideoAgentSettleIntervalSecs int64
	VideoAgentSettleAfterSecs    int64
}

func Load() Config {
	upstreamBase := strings.TrimRight(env("DAXI_UPSTREAM_BASE_URL", "https://supchuang.com/v1"), "/")
	return Config{
		Addr:               env("DAXI_API_ADDR", ":8088"),
		DatabasePath:       env("DAXI_DB_PATH", "data/daxi-cloud-api.db"),
		AdminToken:         env("DAXI_ADMIN_TOKEN", "dev-admin-token"),
		AdminUsername:      env("DAXI_ADMIN_USERNAME", "admin"),
		AdminPassword:      env("DAXI_ADMIN_PASSWORD", "daxi-admin-dev"),
		UpstreamBaseURL:    upstreamBase,
		UpstreamAPIKey:     env("DAXI_UPSTREAM_API_KEY", ""),
		ResellerCode:       env("DAXI_RESELLER_CODE", "daxi-cloud"),
		EmergencyDisabled:  env("DAXI_EMERGENCY_DISABLED", "false") == "true",
		AllowedModels:      splitCSV(env("DAXI_ALLOWED_MODELS", "daxi-smart-router,gpt-4o,claude-sonnet,doubao-seedance,deepseek-chat,qwen-plus")),
		DefaultProxyModel:  env("DAXI_DEFAULT_PROXY_MODEL", "daxi-smart-router"),
		RequestTimeoutSecs: int(envInt64("DAXI_REQUEST_TIMEOUT_SECONDS", 120)),
		MaxBodyBytes:       envInt64("DAXI_MAX_BODY_BYTES", 1<<20),
		VideoAgentBaseURL: strings.TrimRight(env("DAXI_VIDEO_AGENT_UPSTREAM_BASE_URL",
			strings.TrimSuffix(upstreamBase, "/v1")), "/"),
		VideoAgentEstDraftQuota:      envInt64("DAXI_VIDEO_AGENT_EST_DRAFT_QUOTA", 0),
		VideoAgentEstTaskQuota:       envInt64("DAXI_VIDEO_AGENT_EST_TASK_QUOTA", 3000000),
		VideoAgentSettleIntervalSecs: envInt64("DAXI_VIDEO_AGENT_SETTLE_INTERVAL_SECONDS", 60),
		VideoAgentSettleAfterSecs:    envInt64("DAXI_VIDEO_AGENT_SETTLE_AFTER_SECONDS", 300),
	}
}

func envInt64(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
