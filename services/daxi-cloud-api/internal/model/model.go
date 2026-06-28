package model

import "time"

type Lead struct {
	ID           int64     `json:"id"`
	PublicID     string    `json:"public_id"`
	Source       string    `json:"source"`
	Scenario     string    `json:"scenario"`
	Company      string    `json:"company"`
	Country      string    `json:"country"`
	ContactName  string    `json:"contact_name"`
	Email        string    `json:"email"`
	Phone        string    `json:"phone"`
	UsageProfile string    `json:"usage_profile"`
	Budget       string    `json:"budget"`
	Notes        string    `json:"notes"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type LeadActivity struct {
	ID           int64     `json:"id"`
	PublicID     string    `json:"public_id"`
	LeadID       int64     `json:"lead_id"`
	LeadPublicID string    `json:"lead_public_id"`
	Actor        string    `json:"actor"`
	Action       string    `json:"action"`
	Note         string    `json:"note"`
	NextStep     string    `json:"next_step"`
	CreatedAt    time.Time `json:"created_at"`
}

type ComputeInquiry struct {
	ID           int64     `json:"id"`
	PublicID     string    `json:"public_id"`
	Company      string    `json:"company"`
	Country      string    `json:"country"`
	ResourceType string    `json:"resource_type"`
	GPU          string    `json:"gpu"`
	Quantity     string    `json:"quantity"`
	LeasePeriod  string    `json:"lease_period"`
	Budget       string    `json:"budget"`
	Email        string    `json:"email"`
	Phone        string    `json:"phone"`
	Notes        string    `json:"notes"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Customer struct {
	ID        int64     `json:"id"`
	PublicID  string    `json:"public_id"`
	Company   string    `json:"company"`
	Country   string    `json:"country"`
	Email     string    `json:"email"`
	Status    string    `json:"status"`
	// Balance 是「模型额度(token)」钱包,chat 等按 token 扣。
	// 注意:这是 token,不是 quota。video-agent 走独立的 BalanceQuota(quota 单位)钱包,两栏不混显/不混扣。
	// 后续任务:chat 扣费对齐到 quota 后再考虑合并(见 project_daxi_video_agent 记忆)。
	Balance int64 `json:"balance_tokens"`
	// BalanceQuota 是「视频额度(quota)」钱包,video-agent 按 quota 扣(乙方案,与 Balance 物理隔离)。
	BalanceQuota int64     `json:"balance_quota"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type APIKey struct {
	ID         int64     `json:"id"`
	PublicID   string    `json:"public_id"`
	CustomerID int64     `json:"customer_id"`
	Name       string    `json:"name"`
	KeyHash    string    `json:"-"`
	KeyPrefix  string    `json:"key_prefix"`
	Scopes     string    `json:"scopes"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type UsageRecord struct {
	ID               int64     `json:"id"`
	RequestID        string    `json:"request_id"`
	CustomerID       int64     `json:"customer_id"`
	APIKeyID         int64     `json:"api_key_id"`
	Scenario         string    `json:"scenario"`
	Model            string    `json:"model"`
	PromptTokens     int64     `json:"prompt_tokens"`
	CompletionTokens int64     `json:"completion_tokens"`
	TotalTokens      int64     `json:"total_tokens"`
	OverspendTokens  int64     `json:"overspend_tokens"`
	StatusCode       int       `json:"status_code"`
	CreatedAt        time.Time `json:"created_at"`
}

type UsageRecordView struct {
	UsageRecord
	CustomerPublicID string `json:"customer_public_id"`
	CustomerCompany  string `json:"customer_company"`
	APIKeyPublicID   string `json:"api_key_public_id"`
	APIKeyName       string `json:"api_key_name"`
}

type UsageSummary struct {
	RequestCount     int64 `json:"request_count"`
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
	OverspendTokens  int64 `json:"overspend_tokens"`
}

type TokenPlan struct {
	ID            int64     `json:"id"`
	PublicID      string    `json:"public_id"`
	Name          string    `json:"name"`
	ScenarioScope string    `json:"scenario_scope"`
	Tokens        int64     `json:"tokens"`
	PriceCents    int64     `json:"price_cents"`
	Currency      string    `json:"currency"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type TokenOrder struct {
	ID          int64     `json:"id"`
	PublicID    string    `json:"public_id"`
	CustomerID  int64     `json:"customer_id"`
	PlanID      int64     `json:"plan_id"`
	PlanName    string    `json:"plan_name"`
	Tokens      int64     `json:"tokens"`
	AmountCents int64     `json:"amount_cents"`
	Currency    string    `json:"currency"`
	Status      string    `json:"status"`
	Notes       string    `json:"notes"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type TokenRecharge struct {
	ID           int64     `json:"id"`
	PublicID     string    `json:"public_id"`
	CustomerID   int64     `json:"customer_id"`
	CustomerCode string    `json:"customer_public_id"`
	Tokens       int64     `json:"tokens"`
	BalanceAfter int64     `json:"balance_after"`
	Source       string    `json:"source"`
	ReferenceID  string    `json:"reference_id"`
	Notes        string    `json:"notes"`
	Operator     string    `json:"operator"`
	CreatedAt    time.Time `json:"created_at"`
}

type UpstreamStatus struct {
	ResellerCode      string   `json:"reseller_code"`
	BaseURL           string   `json:"base_url"`
	HasAPIKey         bool     `json:"has_api_key"`
	EmergencyDisabled bool     `json:"emergency_disabled"`
	AllowedModels     []string `json:"allowed_models"`
	ModelSource       string   `json:"model_source"`
	ModelSyncError    string   `json:"model_sync_error,omitempty"`
}

type AdminUser struct {
	ID        int64      `json:"id"`
	PublicID  string     `json:"public_id"`
	Username  string     `json:"username"`
	Role      string     `json:"role"`
	Status    string     `json:"status"`
	LastLogin *time.Time `json:"last_login_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type AdminAuditLog struct {
	ID        int64     `json:"id"`
	PublicID  string    `json:"public_id"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	Detail    string    `json:"detail"`
	CreatedAt time.Time `json:"created_at"`
}

type ModelRoute struct {
	ID                  int64     `json:"id"`
	PublicID            string    `json:"public_id"`
	Scenario            string    `json:"scenario"`
	PrimaryModel        string    `json:"primary_model"`
	FallbackModels      string    `json:"fallback_models"`
	MaxTokensPerRequest int64     `json:"max_tokens_per_request"`
	Status              string    `json:"status"`
	Notes               string    `json:"notes"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type VideoTask struct {
	ID               int64     `json:"id"`
	PublicID         string    `json:"public_id"`
	TaskType         string    `json:"task_type"`
	CustomerPublicID string    `json:"customer_public_id"`
	Title            string    `json:"title"`
	Prompt           string    `json:"prompt"`
	ProductName      string    `json:"product_name"`
	Script           string    `json:"script"`
	Language         string    `json:"language"`
	Status           string    `json:"status"`
	Progress         int       `json:"progress"`
	ResultURL        string    `json:"result_url"`
	ErrorMessage     string    `json:"error_message"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	// video-agent 薄代理新增(多步流程归属 + 上游映射 + 计费)
	CustomerID        int64  `json:"customer_id,omitempty"`
	APIKeyID          int64  `json:"api_key_id,omitempty"`
	Scenario          string `json:"scenario,omitempty"`
	UpstreamTaskID    string `json:"upstream_task_id,omitempty"`
	UpstreamRequestID string `json:"upstream_request_id,omitempty"`
	UsageRecordID     int64  `json:"usage_record_id,omitempty"`
	IdempotencyKey    string `json:"idempotency_key,omitempty"`
	EstimatedQuota    int64  `json:"estimated_quota,omitempty"`
	NeedsReview       bool   `json:"needs_review,omitempty"`
}

// VideoAgentDraft 是 DAXI 侧对上游富 agent draft 的轻量镜像(归属 + 幂等 + 上游 id 映射)。
type VideoAgentDraft struct {
	ID              int64     `json:"id"`
	PublicID        string    `json:"draft_id"`
	CustomerID      int64     `json:"-"`
	APIKeyID        int64     `json:"-"`
	Scenario        string    `json:"scenario,omitempty"`
	UpstreamDraftID string    `json:"-"`
	IdempotencyKey  string    `json:"-"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type PaymentRecord struct {
	ID               int64      `json:"id"`
	PublicID         string     `json:"public_id"`
	OrderPublicID    string     `json:"order_public_id"`
	CustomerPublicID string     `json:"customer_public_id"`
	Provider         string     `json:"provider"`
	AmountCents      int64      `json:"amount_cents"`
	Currency         string     `json:"currency"`
	Status           string     `json:"status"`
	ReferenceID      string     `json:"reference_id"`
	Notes            string     `json:"notes"`
	PaidAt           *time.Time `json:"paid_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}
