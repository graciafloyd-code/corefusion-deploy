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
}

func Load() Config {
	return Config{
		Addr:               env("DAXI_API_ADDR", ":8088"),
		DatabasePath:       env("DAXI_DB_PATH", "data/daxi-cloud-api.db"),
		AdminToken:         env("DAXI_ADMIN_TOKEN", "dev-admin-token"),
		AdminUsername:      env("DAXI_ADMIN_USERNAME", "admin"),
		AdminPassword:      env("DAXI_ADMIN_PASSWORD", "daxi-admin-dev"),
		UpstreamBaseURL:    strings.TrimRight(env("DAXI_UPSTREAM_BASE_URL", "https://supchuang.com/v1"), "/"),
		UpstreamAPIKey:     env("DAXI_UPSTREAM_API_KEY", ""),
		ResellerCode:       env("DAXI_RESELLER_CODE", "daxi-cloud"),
		EmergencyDisabled:  env("DAXI_EMERGENCY_DISABLED", "false") == "true",
		AllowedModels:      splitCSV(env("DAXI_ALLOWED_MODELS", "daxi-smart-router,gpt-4o,claude-sonnet,doubao-seedance,deepseek-chat,qwen-plus")),
		DefaultProxyModel:  env("DAXI_DEFAULT_PROXY_MODEL", "daxi-smart-router"),
		RequestTimeoutSecs: int(envInt64("DAXI_REQUEST_TIMEOUT_SECONDS", 120)),
		MaxBodyBytes:       envInt64("DAXI_MAX_BODY_BYTES", 1<<20),
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
