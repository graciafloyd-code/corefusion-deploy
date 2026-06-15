package httpapi

import (
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"daxi-cloud-api/internal/config"
	"daxi-cloud-api/internal/db"
	"daxi-cloud-api/internal/model"
	"daxi-cloud-api/internal/upstream"
)

type Server struct {
	cfg           config.Config
	store         *db.Store
	upstream      *upstream.Client
	mux           *http.ServeMux
	sessionMu     sync.RWMutex
	adminSessions map[string]adminSession
}

type adminSession struct {
	Username  string
	Role      string
	ExpiresAt time.Time
}

func NewServer(cfg config.Config, store *db.Store, upstreamClient *upstream.Client) *Server {
	s := &Server{cfg: cfg, store: store, upstream: upstreamClient, mux: http.NewServeMux(), adminSessions: make(map[string]adminSession)}
	s.routes()
	go s.cleanupExpiredAdminSessions(30 * time.Minute)
	return s
}

func (s *Server) Router() http.Handler {
	return cors(s.limitBody(s.mux))
}

// limitBody caps the size of every request body to guard against oversized
// payloads exhausting memory. Handlers that read the body will receive an
// *http.MaxBytesError once the limit is exceeded.
func (s *Server) limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil && s.cfg.MaxBodyBytes > 0 {
			r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("POST /public/leads", s.handleCreateLead)
	s.mux.HandleFunc("POST /public/compute-inquiries", s.handleCreateComputeInquiry)
	s.mux.HandleFunc("POST /public/agent-requests", s.handleCreateAgentRequest)
	s.mux.HandleFunc("POST /public/token-package-requests", s.handleCreateTokenRequest)
	s.mux.HandleFunc("POST /public/video-requests", s.handleCreateVideoRequest)
	s.mux.HandleFunc("POST /public/video-tasks", s.handleCreatePublicVideoTask)

	s.mux.HandleFunc("POST /admin/sessions", s.handleCreateAdminSession)
	s.mux.HandleFunc("GET /admin/dashboard", s.admin(s.handleDashboard))
	s.mux.HandleFunc("GET /admin/users", s.admin(s.handleListAdminUsers))
	s.mux.HandleFunc("POST /admin/users", s.admin(s.handleCreateAdminUser))
	s.mux.HandleFunc("PATCH /admin/users/", s.admin(s.handlePatchAdminUser))
	s.mux.HandleFunc("GET /admin/audit-logs", s.admin(s.handleListAuditLogs))
	s.mux.HandleFunc("GET /admin/leads", s.admin(s.handleListLeads))
	s.mux.HandleFunc("PATCH /admin/leads/", s.admin(s.handlePatchLead))
	s.mux.HandleFunc("GET /admin/leads/", s.admin(s.handleListLeadActivities))
	s.mux.HandleFunc("POST /admin/leads/", s.admin(s.handleCreateLeadActivity))
	s.mux.HandleFunc("GET /admin/compute-inquiries", s.admin(s.handleListComputeInquiries))
	s.mux.HandleFunc("GET /admin/customers", s.admin(s.handleListCustomers))
	s.mux.HandleFunc("POST /admin/customers", s.admin(s.handleCreateCustomer))
	s.mux.HandleFunc("GET /admin/customers/", s.admin(s.handleGetCustomer))
	s.mux.HandleFunc("PATCH /admin/customers/", s.admin(s.handlePatchCustomer))
	s.mux.HandleFunc("GET /admin/api-keys", s.admin(s.handleListAPIKeys))
	s.mux.HandleFunc("POST /admin/api-keys", s.admin(s.handleCreateAPIKey))
	s.mux.HandleFunc("PATCH /admin/api-keys/", s.admin(s.handlePatchAPIKey))
	s.mux.HandleFunc("GET /admin/usage", s.admin(s.handleUsageSummary))
	s.mux.HandleFunc("GET /admin/usage-records", s.admin(s.handleListUsageRecords))
	s.mux.HandleFunc("GET /admin/token-plans", s.admin(s.handleListTokenPlans))
	s.mux.HandleFunc("GET /admin/token-orders", s.admin(s.handleListTokenOrders))
	s.mux.HandleFunc("POST /admin/token-orders", s.admin(s.handleCreateTokenOrder))
	s.mux.HandleFunc("PATCH /admin/token-orders/", s.admin(s.handlePatchTokenOrder))
	s.mux.HandleFunc("GET /admin/recharges", s.admin(s.handleListRecharges))
	s.mux.HandleFunc("POST /admin/recharges", s.admin(s.handleCreateRecharge))
	s.mux.HandleFunc("GET /admin/model-routes", s.admin(s.handleListModelRoutes))
	s.mux.HandleFunc("POST /admin/model-routes", s.admin(s.handleUpsertModelRoute))
	s.mux.HandleFunc("PATCH /admin/model-routes/", s.admin(s.handlePatchModelRoute))
	s.mux.HandleFunc("GET /admin/video-tasks", s.admin(s.handleListVideoTasks))
	s.mux.HandleFunc("POST /admin/video-tasks", s.admin(s.handleCreateAdminVideoTask))
	s.mux.HandleFunc("PATCH /admin/video-tasks/", s.admin(s.handlePatchVideoTask))
	s.mux.HandleFunc("GET /admin/payments", s.admin(s.handleListPayments))
	s.mux.HandleFunc("POST /admin/payments", s.admin(s.handleCreatePayment))
	s.mux.HandleFunc("PATCH /admin/payments/", s.admin(s.handlePatchPayment))
	s.mux.HandleFunc("GET /admin/upstream", s.admin(s.handleUpstreamStatus))

	s.mux.HandleFunc("GET /v1/models", s.handleModels)
	s.mux.HandleFunc("POST /v1/chat/completions", s.handleChatCompletions)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "daxi-cloud-api"})
}

func (s *Server) handleCreateLead(w http.ResponseWriter, r *http.Request) {
	var req model.Lead
	if err := readJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	req.Source = defaultString(req.Source, "get-started")
	req.Scenario = defaultString(req.Scenario, "General Requirement")
	if err := s.store.CreateLead(&req); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, req)
}

func (s *Server) handleCreateComputeInquiry(w http.ResponseWriter, r *http.Request) {
	var req model.ComputeInquiry
	if err := readJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if err := s.store.CreateComputeInquiry(&req); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	lead := model.Lead{
		Source:       "compute-inquiry",
		Scenario:     "AI Compute Requirement",
		Company:      req.Company,
		Country:      req.Country,
		Email:        req.Email,
		Phone:        req.Phone,
		UsageProfile: req.ResourceType,
		Budget:       req.Budget,
		Notes:        req.Notes,
	}
	_ = s.store.CreateLead(&lead)
	writeJSON(w, http.StatusCreated, map[string]any{"compute_inquiry": req, "lead": lead})
}

func (s *Server) handleCreateAgentRequest(w http.ResponseWriter, r *http.Request) {
	s.createScenarioLead(w, r, "agent-request", "Enterprise / Campus Agent")
}

func (s *Server) handleCreateTokenRequest(w http.ResponseWriter, r *http.Request) {
	s.createScenarioLead(w, r, "token-package-request", "Model API Access")
}

func (s *Server) handleCreateVideoRequest(w http.ResponseWriter, r *http.Request) {
	s.handleCreatePublicVideoTask(w, r)
}

func (s *Server) handleCreatePublicVideoTask(w http.ResponseWriter, r *http.Request) {
	var req model.VideoTask
	if err := readJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		req.Title = defaultString(req.ProductName, "AI video request")
	}
	if err := s.store.CreateVideoTask(&req); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	lead := model.Lead{
		Source:       "video-task",
		Scenario:     defaultString(req.TaskType, "AI Video Generation"),
		Company:      req.CustomerPublicID,
		UsageProfile: req.Title,
		Notes:        req.Prompt,
	}
	_ = s.store.CreateLead(&lead)
	writeJSON(w, http.StatusCreated, map[string]any{"video_task": req, "lead": lead})
}

func (s *Server) handleCreateAdminSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username   string `json:"username"`
		Password   string `json:"password"`
		AdminToken string `json:"admin_token"`
	}
	if err := readJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	var actor, role string
	if req.AdminToken != "" {
		if subtle.ConstantTimeCompare([]byte(req.AdminToken), []byte(s.cfg.AdminToken)) != 1 {
			writeError(w, http.StatusUnauthorized, "invalid admin token")
			return
		}
		actor = "admin-token"
		role = "Owner"
	} else {
		// Login always goes through the database. The env credentials only seed
		// the first owner account (see EnsureAdminUser); they are not a login
		// fallback, so a disabled account cannot be reached via env values.
		user, err := s.store.AuthenticateAdmin(req.Username, req.Password)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid admin credentials")
			return
		}
		actor = user.Username
		role = user.Role
	}
	token := "dxs_" + randomToken(24)
	expiresAt := time.Now().UTC().Add(12 * time.Hour)
	s.sessionMu.Lock()
	s.adminSessions[token] = adminSession{Username: actor, Role: role, ExpiresAt: expiresAt}
	s.sessionMu.Unlock()
	_ = s.store.RecordAdminAudit(actor, "admin.login", "admin_sessions", "Admin signed in")
	writeJSON(w, http.StatusCreated, map[string]any{"session_token": token, "expires_at": expiresAt, "username": actor, "role": role})
}

func (s *Server) createScenarioLead(w http.ResponseWriter, r *http.Request, source, fallbackScenario string) {
	var req model.Lead
	if err := readJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	req.Source = source
	req.Scenario = defaultString(req.Scenario, fallbackScenario)
	if err := s.store.CreateLead(&req); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, req)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	leads, _ := s.store.ListLeads(200)
	compute, _ := s.store.ListComputeInquiries(200)
	customers, _ := s.store.ListCustomers(200)
	keys, _ := s.store.ListAPIKeys(200)
	usage, _ := s.store.UsageSummary()
	videoTasks, _ := s.store.ListVideoTasks(200)
	writeJSON(w, http.StatusOK, map[string]any{
		"lead_count":            len(leads),
		"compute_inquiry_count": len(compute),
		"customer_count":        len(customers),
		"api_key_count":         len(keys),
		"video_task_count":      len(videoTasks),
		"usage":                 usage,
		"upstream":              s.upstreamStatus(),
	})
}

func (s *Server) handleListAdminUsers(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListAdminUsers(parseLimit(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleCreateAdminUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := readJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	item, err := s.store.CreateAdminUser(req.Username, req.Password, req.Role)
	if err != nil {
		badRequest(w, err)
		return
	}
	_ = s.store.RecordAdminAudit("admin", "admin_user.create", item.PublicID, item.Username)
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) handlePatchAdminUser(w http.ResponseWriter, r *http.Request) {
	publicID := strings.TrimPrefix(r.URL.Path, "/admin/users/")
	publicID = strings.TrimSuffix(publicID, "/status")
	var req struct {
		Status string `json:"status"`
	}
	if err := readJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if err := s.store.UpdateAdminUserStatus(publicID, defaultString(req.Status, "Active")); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "admin user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.store.RecordAdminAudit("admin", "admin_user.status", publicID, req.Status)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListAdminAuditLogs(parseLimit(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleListLeads(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r)
	leads, err := s.store.ListLeads(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": leads})
}

func (s *Server) handlePatchLead(w http.ResponseWriter, r *http.Request) {
	publicID := strings.TrimPrefix(r.URL.Path, "/admin/leads/")
	publicID = strings.TrimSuffix(publicID, "/status")
	var req struct {
		Status string `json:"status"`
	}
	if err := readJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if strings.TrimSpace(req.Status) == "" {
		writeError(w, http.StatusBadRequest, "status is required")
		return
	}
	if err := s.store.UpdateLeadStatus(publicID, req.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "lead not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleListLeadActivities(w http.ResponseWriter, r *http.Request) {
	publicID, ok := leadActivityPathID(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "lead activity route not found")
		return
	}
	items, err := s.store.ListLeadActivities(publicID, parseLimit(r))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "lead not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleCreateLeadActivity(w http.ResponseWriter, r *http.Request) {
	publicID, ok := leadActivityPathID(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "lead activity route not found")
		return
	}
	var req model.LeadActivity
	if err := readJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	req.Actor = defaultString(req.Actor, "admin")
	req.Action = defaultString(req.Action, "note.added")
	item, err := s.store.CreateLeadActivity(publicID, req)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "lead not found")
			return
		}
		badRequest(w, err)
		return
	}
	_ = s.store.RecordAdminAudit("admin", "lead.activity", publicID, item.Action)
	writeJSON(w, http.StatusCreated, item)
}

func leadActivityPathID(path string) (string, bool) {
	trimmed := strings.TrimPrefix(path, "/admin/leads/")
	if !strings.HasSuffix(trimmed, "/activities") {
		return "", false
	}
	publicID := strings.TrimSuffix(trimmed, "/activities")
	publicID = strings.Trim(publicID, "/")
	return publicID, publicID != ""
}

func (s *Server) handleListComputeInquiries(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListComputeInquiries(parseLimit(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleCreateCustomer(w http.ResponseWriter, r *http.Request) {
	var req model.Customer
	if err := readJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if strings.TrimSpace(req.Company) == "" {
		writeError(w, http.StatusBadRequest, "company is required")
		return
	}
	if err := s.store.CreateCustomer(&req); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, req)
}

func (s *Server) handleListCustomers(w http.ResponseWriter, r *http.Request) {
	customers, err := s.store.ListCustomers(parseLimit(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": customers})
}

func (s *Server) handleGetCustomer(w http.ResponseWriter, r *http.Request) {
	publicID := strings.TrimPrefix(r.URL.Path, "/admin/customers/")
	customer, err := s.store.GetCustomer(publicID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "customer not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	keys, _ := s.store.ListAPIKeysByCustomer(customer.ID)
	usage, _ := s.store.CustomerUsageSummary(customer.ID)
	usageRecords, _ := s.store.ListUsageRecordsByCustomer(customer.ID, 50)
	recharges, _ := s.store.ListRechargesByCustomer(customer.ID, 50)
	writeJSON(w, http.StatusOK, map[string]any{
		"customer":      customer,
		"api_keys":      keys,
		"usage":         usage,
		"usage_records": usageRecords,
		"recharges":     recharges,
	})
}

func (s *Server) handlePatchCustomer(w http.ResponseWriter, r *http.Request) {
	publicID := strings.TrimPrefix(r.URL.Path, "/admin/customers/")
	publicID = strings.TrimSuffix(publicID, "/balance")
	var req struct {
		BalanceTokens int64 `json:"balance_tokens"`
	}
	if err := readJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if req.BalanceTokens < 0 {
		writeError(w, http.StatusBadRequest, "balance_tokens must be greater than or equal to 0")
		return
	}
	if err := s.store.UpdateCustomerBalance(publicID, req.BalanceTokens); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "customer not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := s.store.ListAPIKeys(parseLimit(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": keys})
}

func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CustomerPublicID string `json:"customer_public_id"`
		Name             string `json:"name"`
		Scopes           string `json:"scopes"`
	}
	if err := readJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	key, raw, err := s.store.CreateAPIKey(req.CustomerPublicID, req.Name, req.Scopes)
	if err != nil {
		badRequest(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"api_key": key, "secret": raw})
}

func (s *Server) handlePatchAPIKey(w http.ResponseWriter, r *http.Request) {
	publicID := strings.TrimPrefix(r.URL.Path, "/admin/api-keys/")
	publicID = strings.TrimSuffix(publicID, "/status")
	var req struct {
		Status string `json:"status"`
	}
	if err := readJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if strings.TrimSpace(req.Status) == "" {
		writeError(w, http.StatusBadRequest, "status is required")
		return
	}
	if !validStatus(req.Status, "Active", "Disabled") {
		writeError(w, http.StatusBadRequest, "invalid api key status")
		return
	}
	req.Status = canonicalStatus(req.Status, "Active", "Disabled")
	if err := s.store.UpdateAPIKeyStatus(publicID, req.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "api key not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.store.RecordAdminAudit("admin", "api_key.status", publicID, req.Status)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleUsageSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := s.store.UsageSummary()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleListUsageRecords(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListUsageRecords(parseLimit(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleListTokenPlans(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListTokenPlans()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleCreateTokenOrder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CustomerPublicID string `json:"customer_public_id"`
		PlanPublicID     string `json:"plan_public_id"`
		Notes            string `json:"notes"`
	}
	if err := readJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	order, err := s.store.CreateTokenOrder(req.CustomerPublicID, req.PlanPublicID, req.Notes)
	if err != nil {
		badRequest(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, order)
}

func (s *Server) handleListTokenOrders(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListTokenOrders(parseLimit(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handlePatchTokenOrder(w http.ResponseWriter, r *http.Request) {
	publicID := strings.TrimPrefix(r.URL.Path, "/admin/token-orders/")
	publicID = strings.TrimSuffix(publicID, "/status")
	var req struct {
		Status string `json:"status"`
	}
	if err := readJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if !validStatus(req.Status, "Pending", "Paid", "Cancelled") {
		writeError(w, http.StatusBadRequest, "invalid token order status")
		return
	}
	req.Status = canonicalStatus(req.Status, "Pending", "Paid", "Cancelled")
	if err := s.store.UpdateTokenOrderStatus(publicID, req.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "token order not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.store.RecordAdminAudit("admin", "token_order.status", publicID, req.Status)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleCreateRecharge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CustomerPublicID string `json:"customer_public_id"`
		Tokens           int64  `json:"tokens"`
		Source           string `json:"source"`
		ReferenceID      string `json:"reference_id"`
		Notes            string `json:"notes"`
		Operator         string `json:"operator"`
	}
	if err := readJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	item, err := s.store.CreateRecharge(req.CustomerPublicID, req.Tokens, req.Source, req.ReferenceID, req.Notes, req.Operator)
	if err != nil {
		badRequest(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) handleListRecharges(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListRecharges(parseLimit(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleListModelRoutes(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListModelRoutes()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleUpsertModelRoute(w http.ResponseWriter, r *http.Request) {
	var req model.ModelRoute
	if err := readJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if req.Status != "" && !validStatus(req.Status, "Active", "Disabled") {
		writeError(w, http.StatusBadRequest, "invalid model route status")
		return
	}
	req.Status = canonicalStatus(defaultString(req.Status, "Active"), "Active", "Disabled")
	if err := s.store.UpsertModelRoute(&req); err != nil {
		badRequest(w, err)
		return
	}
	_ = s.store.RecordAdminAudit("admin", "model_route.upsert", req.Scenario, req.PrimaryModel)
	writeJSON(w, http.StatusCreated, req)
}

func (s *Server) handlePatchModelRoute(w http.ResponseWriter, r *http.Request) {
	publicID := strings.TrimPrefix(r.URL.Path, "/admin/model-routes/")
	var req model.ModelRoute
	if err := readJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if req.Status != "" && !validStatus(req.Status, "Active", "Disabled") {
		writeError(w, http.StatusBadRequest, "invalid model route status")
		return
	}
	req.Status = canonicalStatus(defaultString(req.Status, "Active"), "Active", "Disabled")
	if err := s.store.UpdateModelRoute(publicID, req); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "model route not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.store.RecordAdminAudit("admin", "model_route.update", publicID, req.PrimaryModel)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleListVideoTasks(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListVideoTasks(parseLimit(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleCreateAdminVideoTask(w http.ResponseWriter, r *http.Request) {
	var req model.VideoTask
	if err := readJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if err := s.store.CreateVideoTask(&req); err != nil {
		badRequest(w, err)
		return
	}
	_ = s.store.RecordAdminAudit("admin", "video_task.create", req.PublicID, req.TaskType)
	writeJSON(w, http.StatusCreated, req)
}

func (s *Server) handlePatchVideoTask(w http.ResponseWriter, r *http.Request) {
	publicID := strings.TrimPrefix(r.URL.Path, "/admin/video-tasks/")
	var req model.VideoTask
	if err := readJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if err := s.store.UpdateVideoTask(publicID, req); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "video task not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.store.RecordAdminAudit("admin", "video_task.update", publicID, req.Status)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleListPayments(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListPaymentRecords(parseLimit(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleCreatePayment(w http.ResponseWriter, r *http.Request) {
	var req model.PaymentRecord
	if err := readJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if req.AmountCents < 0 {
		writeError(w, http.StatusBadRequest, "amount_cents must be greater than or equal to 0")
		return
	}
	if req.Status != "" && !validStatus(req.Status, "Pending", "Paid", "Failed", "Cancelled") {
		writeError(w, http.StatusBadRequest, "invalid payment status")
		return
	}
	req.Status = canonicalStatus(defaultString(req.Status, "Pending"), "Pending", "Paid", "Failed", "Cancelled")
	if err := s.store.CreatePaymentRecord(&req); err != nil {
		badRequest(w, err)
		return
	}
	_ = s.store.RecordAdminAudit("admin", "payment.create", req.PublicID, req.Status)
	writeJSON(w, http.StatusCreated, req)
}

func (s *Server) handlePatchPayment(w http.ResponseWriter, r *http.Request) {
	publicID := strings.TrimPrefix(r.URL.Path, "/admin/payments/")
	publicID = strings.TrimSuffix(publicID, "/status")
	var req struct {
		Status string `json:"status"`
	}
	if err := readJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if strings.TrimSpace(req.Status) == "" {
		writeError(w, http.StatusBadRequest, "status is required")
		return
	}
	if !validStatus(req.Status, "Pending", "Paid", "Failed", "Cancelled") {
		writeError(w, http.StatusBadRequest, "invalid payment status")
		return
	}
	req.Status = canonicalStatus(req.Status, "Pending", "Paid", "Failed", "Cancelled")
	payment, recharge, err := s.store.SettlePaymentStatus(publicID, req.Status, "admin")
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "payment record not found")
			return
		}
		badRequest(w, err)
		return
	}
	_ = s.store.RecordAdminAudit("admin", "payment.status", publicID, req.Status)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "payment": payment, "recharge": recharge})
}

func (s *Server) handleUpstreamStatus(w http.ResponseWriter, r *http.Request) {
	status := s.upstreamStatus()
	if strings.TrimSpace(s.cfg.UpstreamAPIKey) != "" && !s.cfg.EmergencyDisabled {
		models, err := s.upstream.ListModels(r.Context())
		if err != nil {
			status.ModelSyncError = err.Error()
		} else {
			status.AllowedModels = modelIDs(models)
		}
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) upstreamStatus() model.UpstreamStatus {
	return model.UpstreamStatus{
		ResellerCode:      s.cfg.ResellerCode,
		BaseURL:           s.cfg.UpstreamBaseURL,
		HasAPIKey:         strings.TrimSpace(s.cfg.UpstreamAPIKey) != "",
		EmergencyDisabled: s.cfg.EmergencyDisabled,
		AllowedModels:     s.cfg.AllowedModels,
		ModelSource:       "supchuang-upstream",
	}
}

func modelIDs(models upstream.ModelList) []string {
	out := make([]string, 0, len(models.Data))
	for _, item := range models.Data {
		if strings.TrimSpace(item.ID) != "" {
			out = append(out, item.ID)
		}
	}
	return out
}

func (s *Server) admin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" {
			token = r.Header.Get("X-Admin-Token")
		}
		matchesAdminToken := subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.AdminToken)) == 1
		if !matchesAdminToken && !s.validAdminSession(token) {
			writeError(w, http.StatusUnauthorized, "admin token required")
			return
		}
		next(w, r)
	}
}

func (s *Server) validAdminSession(token string) bool {
	if !strings.HasPrefix(token, "dxs_") {
		return false
	}
	s.sessionMu.RLock()
	session, ok := s.adminSessions[token]
	s.sessionMu.RUnlock()
	if !ok || time.Now().UTC().After(session.ExpiresAt) {
		if ok {
			s.sessionMu.Lock()
			delete(s.adminSessions, token)
			s.sessionMu.Unlock()
		}
		return false
	}
	return true
}

func (s *Server) cleanupExpiredAdminSessions(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now().UTC()
		s.sessionMu.Lock()
		for token, session := range s.adminSessions {
			if now.After(session.ExpiresAt) {
				delete(s.adminSessions, token)
			}
		}
		s.sessionMu.Unlock()
	}
}

func parseLimit(r *http.Request) int {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	return limit
}

func readJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}

// badRequest reports a client error, mapping an exceeded body-size limit to 413
// and everything else to 400.
func badRequest(w http.ResponseWriter, err error) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
		return
	}
	writeError(w, http.StatusBadRequest, err.Error())
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func validStatus(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if strings.EqualFold(strings.TrimSpace(value), candidate) {
			return true
		}
	}
	return false
}

func canonicalStatus(value string, allowed ...string) string {
	for _, candidate := range allowed {
		if strings.EqualFold(strings.TrimSpace(value), candidate) {
			return candidate
		}
	}
	return strings.TrimSpace(value)
}

func randomToken(bytes int) string {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Admin-Token, X-DAXI-Scenario")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
