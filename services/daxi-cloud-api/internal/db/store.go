package db

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"daxi-cloud-api/internal/model"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	conn, err := sql.Open("sqlite3", path+"?_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, err
	}
	return &Store{db: conn}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Migrate() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS leads (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			public_id TEXT NOT NULL UNIQUE,
			source TEXT NOT NULL,
			scenario TEXT NOT NULL,
			company TEXT,
			country TEXT,
			contact_name TEXT,
			email TEXT,
			phone TEXT,
			usage_profile TEXT,
			budget TEXT,
			notes TEXT,
			status TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_leads_status_created ON leads(status, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS lead_activities (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			public_id TEXT NOT NULL UNIQUE,
			lead_id INTEGER NOT NULL REFERENCES leads(id),
			actor TEXT NOT NULL,
			action TEXT NOT NULL,
			note TEXT,
			next_step TEXT,
			created_at DATETIME NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_lead_activities_lead_created ON lead_activities(lead_id, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS compute_inquiries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			public_id TEXT NOT NULL UNIQUE,
			company TEXT,
			country TEXT,
			resource_type TEXT,
			gpu TEXT,
			quantity TEXT,
			lease_period TEXT,
			budget TEXT,
			email TEXT,
			phone TEXT,
			notes TEXT,
			status TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS customers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			public_id TEXT NOT NULL UNIQUE,
			company TEXT NOT NULL,
			country TEXT,
			email TEXT,
			status TEXT NOT NULL,
			balance_tokens INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS api_keys (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			public_id TEXT NOT NULL UNIQUE,
			customer_id INTEGER NOT NULL REFERENCES customers(id),
			name TEXT NOT NULL,
			key_hash TEXT NOT NULL UNIQUE,
			key_prefix TEXT NOT NULL,
			scopes TEXT,
			status TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS usage_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			request_id TEXT NOT NULL,
			customer_id INTEGER NOT NULL,
			api_key_id INTEGER NOT NULL,
			scenario TEXT,
			model TEXT,
			prompt_tokens INTEGER NOT NULL DEFAULT 0,
			completion_tokens INTEGER NOT NULL DEFAULT 0,
			total_tokens INTEGER NOT NULL DEFAULT 0,
			overspend_tokens INTEGER NOT NULL DEFAULT 0,
			status_code INTEGER NOT NULL,
			created_at DATETIME NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_customer_created ON usage_records(customer_id, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS token_plans (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			public_id TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			scenario_scope TEXT,
			tokens INTEGER NOT NULL,
			price_cents INTEGER NOT NULL,
			currency TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS token_orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			public_id TEXT NOT NULL UNIQUE,
			customer_id INTEGER NOT NULL REFERENCES customers(id),
			plan_id INTEGER NOT NULL REFERENCES token_plans(id),
			plan_name TEXT NOT NULL,
			tokens INTEGER NOT NULL,
			amount_cents INTEGER NOT NULL,
			currency TEXT NOT NULL,
			status TEXT NOT NULL,
			notes TEXT,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS token_recharges (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			public_id TEXT NOT NULL UNIQUE,
			customer_id INTEGER NOT NULL REFERENCES customers(id),
			tokens INTEGER NOT NULL,
			balance_after INTEGER NOT NULL,
			source TEXT NOT NULL,
			reference_id TEXT,
			notes TEXT,
			operator TEXT,
			created_at DATETIME NOT NULL
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_token_recharges_source_reference ON token_recharges(source, reference_id) WHERE reference_id IS NOT NULL AND reference_id != ''`,
		`CREATE TABLE IF NOT EXISTS admin_users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			public_id TEXT NOT NULL UNIQUE,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			password_salt TEXT NOT NULL,
			role TEXT NOT NULL,
			status TEXT NOT NULL,
			last_login_at DATETIME,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS admin_audit_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			public_id TEXT NOT NULL UNIQUE,
			actor TEXT NOT NULL,
			action TEXT NOT NULL,
			target TEXT,
			detail TEXT,
			created_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS model_routes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			public_id TEXT NOT NULL UNIQUE,
			scenario TEXT NOT NULL UNIQUE,
			primary_model TEXT NOT NULL,
			fallback_models TEXT,
			max_tokens_per_request INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL,
			notes TEXT,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS video_tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			public_id TEXT NOT NULL UNIQUE,
			task_type TEXT NOT NULL,
			customer_public_id TEXT,
			title TEXT,
			prompt TEXT,
			product_name TEXT,
			script TEXT,
			language TEXT,
			status TEXT NOT NULL,
			progress INTEGER NOT NULL DEFAULT 0,
			result_url TEXT,
			error_message TEXT,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_video_tasks_status_created ON video_tasks(status, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS payment_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			public_id TEXT NOT NULL UNIQUE,
			order_public_id TEXT,
			customer_public_id TEXT,
			provider TEXT NOT NULL,
			amount_cents INTEGER NOT NULL,
			currency TEXT NOT NULL,
			status TEXT NOT NULL,
			reference_id TEXT,
			notes TEXT,
			paid_at DATETIME,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,
	}
	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return err
		}
	}
	if err := s.ensureColumn("usage_records", "overspend_tokens", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.seedTokenPlans(); err != nil {
		return err
	}
	if err := s.seedModelRoutes(); err != nil {
		return err
	}
	return nil
}

func (s *Store) ensureColumn(table, column, definition string) error {
	rows, err := s.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` ` + definition)
	return err
}

func (s *Store) seedTokenPlans() error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM token_plans`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	now := time.Now().UTC()
	defaults := []model.TokenPlan{
		{PublicID: "DXP-TRIAL", Name: "API Trial Pack", ScenarioScope: "model-api", Tokens: 100000, PriceCents: 9900, Currency: "USD", Status: "Active"},
		{PublicID: "DXP-BUSINESS", Name: "Business API Pack", ScenarioScope: "model-api,agent,ecommerce-video", Tokens: 1000000, PriceCents: 69900, Currency: "USD", Status: "Active"},
		{PublicID: "DXP-ENTERPRISE", Name: "Enterprise Volume", ScenarioScope: "all", Tokens: 10000000, PriceCents: 399900, Currency: "USD", Status: "Active"},
	}
	for _, plan := range defaults {
		if _, err := s.db.Exec(`INSERT INTO token_plans(public_id, name, scenario_scope, tokens, price_cents, currency, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			plan.PublicID, plan.Name, plan.ScenarioScope, plan.Tokens, plan.PriceCents, plan.Currency, plan.Status, now, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) EnsureAdminUser(username, password string) error {
	username = strings.TrimSpace(username)
	if username == "" || strings.TrimSpace(password) == "" {
		return nil
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM admin_users`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	salt := randomHex(16)
	now := time.Now().UTC()
	_, err := s.db.Exec(`INSERT INTO admin_users(public_id, username, password_hash, password_salt, role, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		newPublicID("DXA"), username, hashPassword(password, salt), salt, "Owner", "Active", now, now)
	return err
}

func (s *Store) AuthenticateAdmin(username, password string) (model.AdminUser, error) {
	row := s.db.QueryRow(`SELECT id, public_id, username, password_hash, password_salt, role, status, last_login_at, created_at, updated_at FROM admin_users WHERE username = ?`, strings.TrimSpace(username))
	var item model.AdminUser
	var passwordHash, passwordSalt string
	var lastLogin sql.NullTime
	if err := row.Scan(&item.ID, &item.PublicID, &item.Username, &passwordHash, &passwordSalt, &item.Role, &item.Status, &lastLogin, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return model.AdminUser{}, err
	}
	if !strings.EqualFold(item.Status, "Active") || subtle.ConstantTimeCompare([]byte(hashPassword(password, passwordSalt)), []byte(passwordHash)) != 1 {
		return model.AdminUser{}, sql.ErrNoRows
	}
	if lastLogin.Valid {
		item.LastLogin = &lastLogin.Time
	}
	now := time.Now().UTC()
	_, _ = s.db.Exec(`UPDATE admin_users SET last_login_at = ?, updated_at = ? WHERE id = ?`, now, now, item.ID)
	item.LastLogin = &now
	return item, nil
}

func (s *Store) CreateAdminUser(username, password, role string) (model.AdminUser, error) {
	if strings.TrimSpace(username) == "" || strings.TrimSpace(password) == "" {
		return model.AdminUser{}, fmt.Errorf("username and password are required")
	}
	now := time.Now().UTC()
	salt := randomHex(16)
	item := model.AdminUser{
		PublicID:  newPublicID("DXA"),
		Username:  strings.TrimSpace(username),
		Role:      defaultString(role, "Operator"),
		Status:    "Active",
		CreatedAt: now,
		UpdatedAt: now,
	}
	result, err := s.db.Exec(`INSERT INTO admin_users(public_id, username, password_hash, password_salt, role, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		item.PublicID, item.Username, hashPassword(password, salt), salt, item.Role, item.Status, item.CreatedAt, item.UpdatedAt)
	if err != nil {
		return model.AdminUser{}, err
	}
	item.ID, _ = result.LastInsertId()
	return item, nil
}

func (s *Store) ListAdminUsers(limit int) ([]model.AdminUser, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT id, public_id, username, role, status, last_login_at, created_at, updated_at FROM admin_users ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.AdminUser{}
	for rows.Next() {
		var item model.AdminUser
		var lastLogin sql.NullTime
		if err := rows.Scan(&item.ID, &item.PublicID, &item.Username, &item.Role, &item.Status, &lastLogin, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		if lastLogin.Valid {
			item.LastLogin = &lastLogin.Time
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) UpdateAdminUserStatus(publicID, status string) error {
	result, err := s.db.Exec(`UPDATE admin_users SET status = ?, updated_at = ? WHERE public_id = ?`, status, time.Now().UTC(), publicID)
	if err != nil {
		return err
	}
	return checkAffected(result)
}

func (s *Store) RecordAdminAudit(actor, action, target, detail string) error {
	_, err := s.db.Exec(`INSERT INTO admin_audit_logs(public_id, actor, action, target, detail, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		newPublicID("DXG"), defaultString(actor, "admin"), action, target, detail, time.Now().UTC())
	return err
}

func (s *Store) ListAdminAuditLogs(limit int) ([]model.AdminAuditLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT id, public_id, actor, action, target, detail, created_at FROM admin_audit_logs ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.AdminAuditLog{}
	for rows.Next() {
		var item model.AdminAuditLog
		if err := rows.Scan(&item.ID, &item.PublicID, &item.Actor, &item.Action, &item.Target, &item.Detail, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) seedModelRoutes() error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM model_routes`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	now := time.Now().UTC()
	defaults := []model.ModelRoute{
		{Scenario: "model-api", PrimaryModel: "daxi-smart-router", FallbackModels: "gpt-4.1-mini,qwen-plus", MaxTokensPerRequest: 32000, Status: "Active", Notes: "Default OpenAI-compatible model API route."},
		{Scenario: "short-drama", PrimaryModel: "doubao-seedance", FallbackModels: "daxi-video-router", MaxTokensPerRequest: 64000, Status: "Active", Notes: "Short drama script, storyboard, and video generation path."},
		{Scenario: "ecommerce-video", PrimaryModel: "doubao-seedance", FallbackModels: "daxi-video-router", MaxTokensPerRequest: 64000, Status: "Active", Notes: "Product short video generation path."},
		{Scenario: "agent", PrimaryModel: "qwen-plus", FallbackModels: "daxi-smart-router", MaxTokensPerRequest: 32000, Status: "Active", Notes: "Enterprise and campus agent workflow route."},
	}
	for _, route := range defaults {
		if _, err := s.db.Exec(`INSERT INTO model_routes(public_id, scenario, primary_model, fallback_models, max_tokens_per_request, status, notes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			newPublicID("DXM"), route.Scenario, route.PrimaryModel, route.FallbackModels, route.MaxTokensPerRequest, route.Status, route.Notes, now, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CreateLead(lead *model.Lead) error {
	now := time.Now().UTC()
	lead.PublicID = newPublicID("DXL")
	lead.Status = defaultString(lead.Status, "New")
	lead.CreatedAt = now
	lead.UpdatedAt = now
	result, err := retryPublicID(func() { lead.PublicID = newPublicID("DXL") }, func() (sql.Result, error) {
		return s.db.Exec(`INSERT INTO leads
		(public_id, source, scenario, company, country, contact_name, email, phone, usage_profile, budget, notes, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			lead.PublicID, lead.Source, lead.Scenario, lead.Company, lead.Country, lead.ContactName, lead.Email, lead.Phone,
			lead.UsageProfile, lead.Budget, lead.Notes, lead.Status, lead.CreatedAt, lead.UpdatedAt)
	})
	if err != nil {
		return err
	}
	lead.ID, _ = result.LastInsertId()
	return nil
}

func (s *Store) ListLeads(limit int) ([]model.Lead, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT id, public_id, source, scenario, company, country, contact_name, email, phone, usage_profile, budget, notes, status, created_at, updated_at FROM leads ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var leads []model.Lead
	for rows.Next() {
		var lead model.Lead
		if err := rows.Scan(&lead.ID, &lead.PublicID, &lead.Source, &lead.Scenario, &lead.Company, &lead.Country, &lead.ContactName, &lead.Email, &lead.Phone, &lead.UsageProfile, &lead.Budget, &lead.Notes, &lead.Status, &lead.CreatedAt, &lead.UpdatedAt); err != nil {
			return nil, err
		}
		leads = append(leads, lead)
	}
	return leads, rows.Err()
}

func (s *Store) UpdateLeadStatus(publicID, status string) error {
	lead, err := s.GetLead(publicID)
	if err != nil {
		return err
	}
	result, err := s.db.Exec(`UPDATE leads SET status = ?, updated_at = ? WHERE public_id = ?`, status, time.Now().UTC(), publicID)
	if err != nil {
		return err
	}
	if err := checkAffected(result); err != nil {
		return err
	}
	_, _ = s.CreateLeadActivity(publicID, model.LeadActivity{
		Actor:  "admin",
		Action: "status.updated",
		Note:   fmt.Sprintf("Status changed from %s to %s.", defaultString(lead.Status, "New"), status),
	})
	return nil
}

func (s *Store) GetLead(publicID string) (model.Lead, error) {
	row := s.db.QueryRow(`SELECT id, public_id, source, scenario, company, country, contact_name, email, phone, usage_profile, budget, notes, status, created_at, updated_at FROM leads WHERE public_id = ?`, publicID)
	var lead model.Lead
	if err := row.Scan(&lead.ID, &lead.PublicID, &lead.Source, &lead.Scenario, &lead.Company, &lead.Country, &lead.ContactName, &lead.Email, &lead.Phone, &lead.UsageProfile, &lead.Budget, &lead.Notes, &lead.Status, &lead.CreatedAt, &lead.UpdatedAt); err != nil {
		return model.Lead{}, err
	}
	return lead, nil
}

func (s *Store) CreateLeadActivity(leadPublicID string, activity model.LeadActivity) (model.LeadActivity, error) {
	lead, err := s.GetLead(leadPublicID)
	if err != nil {
		return model.LeadActivity{}, err
	}
	now := time.Now().UTC()
	item := model.LeadActivity{
		PublicID:     newPublicID("DXN"),
		LeadID:       lead.ID,
		LeadPublicID: lead.PublicID,
		Actor:        defaultString(activity.Actor, "admin"),
		Action:       defaultString(activity.Action, "note.added"),
		Note:         strings.TrimSpace(activity.Note),
		NextStep:     strings.TrimSpace(activity.NextStep),
		CreatedAt:    now,
	}
	if strings.TrimSpace(item.Note) == "" && strings.TrimSpace(item.NextStep) == "" {
		return model.LeadActivity{}, fmt.Errorf("note or next_step is required")
	}
	result, err := retryPublicID(func() { item.PublicID = newPublicID("DXN") }, func() (sql.Result, error) {
		return s.db.Exec(`INSERT INTO lead_activities(public_id, lead_id, actor, action, note, next_step, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			item.PublicID, item.LeadID, item.Actor, item.Action, item.Note, item.NextStep, item.CreatedAt)
	})
	if err != nil {
		return model.LeadActivity{}, err
	}
	item.ID, _ = result.LastInsertId()
	return item, nil
}

func (s *Store) ListLeadActivities(leadPublicID string, limit int) ([]model.LeadActivity, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	lead, err := s.GetLead(leadPublicID)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT id, public_id, lead_id, actor, action, note, next_step, created_at FROM lead_activities WHERE lead_id = ? ORDER BY created_at DESC LIMIT ?`, lead.ID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.LeadActivity{}
	for rows.Next() {
		var item model.LeadActivity
		if err := rows.Scan(&item.ID, &item.PublicID, &item.LeadID, &item.Actor, &item.Action, &item.Note, &item.NextStep, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.LeadPublicID = lead.PublicID
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) CreateComputeInquiry(inquiry *model.ComputeInquiry) error {
	now := time.Now().UTC()
	inquiry.PublicID = newPublicID("DXC")
	inquiry.Status = defaultString(inquiry.Status, "Submitted")
	inquiry.CreatedAt = now
	inquiry.UpdatedAt = now
	result, err := retryPublicID(func() { inquiry.PublicID = newPublicID("DXC") }, func() (sql.Result, error) {
		return s.db.Exec(`INSERT INTO compute_inquiries
		(public_id, company, country, resource_type, gpu, quantity, lease_period, budget, email, phone, notes, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			inquiry.PublicID, inquiry.Company, inquiry.Country, inquiry.ResourceType, inquiry.GPU, inquiry.Quantity, inquiry.LeasePeriod,
			inquiry.Budget, inquiry.Email, inquiry.Phone, inquiry.Notes, inquiry.Status, inquiry.CreatedAt, inquiry.UpdatedAt)
	})
	if err != nil {
		return err
	}
	inquiry.ID, _ = result.LastInsertId()
	return nil
}

func (s *Store) ListComputeInquiries(limit int) ([]model.ComputeInquiry, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT id, public_id, company, country, resource_type, gpu, quantity, lease_period, budget, email, phone, notes, status, created_at, updated_at FROM compute_inquiries ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ComputeInquiry
	for rows.Next() {
		var item model.ComputeInquiry
		if err := rows.Scan(&item.ID, &item.PublicID, &item.Company, &item.Country, &item.ResourceType, &item.GPU, &item.Quantity, &item.LeasePeriod, &item.Budget, &item.Email, &item.Phone, &item.Notes, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) CreateCustomer(customer *model.Customer) error {
	now := time.Now().UTC()
	customer.PublicID = newPublicID("DXT")
	customer.Status = canonicalStatus(defaultString(customer.Status, "Active"), "Active", "Disabled")
	customer.CreatedAt = now
	customer.UpdatedAt = now
	result, err := retryPublicID(func() { customer.PublicID = newPublicID("DXT") }, func() (sql.Result, error) {
		return s.db.Exec(`INSERT INTO customers(public_id, company, country, email, status, balance_tokens, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			customer.PublicID, customer.Company, customer.Country, customer.Email, customer.Status, customer.Balance, customer.CreatedAt, customer.UpdatedAt)
	})
	if err != nil {
		return err
	}
	customer.ID, _ = result.LastInsertId()
	return nil
}

func (s *Store) ListCustomers(limit int) ([]model.Customer, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT id, public_id, company, country, email, status, balance_tokens, created_at, updated_at FROM customers ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Customer
	for rows.Next() {
		var item model.Customer
		if err := rows.Scan(&item.ID, &item.PublicID, &item.Company, &item.Country, &item.Email, &item.Status, &item.Balance, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetCustomer(publicID string) (model.Customer, error) {
	row := s.db.QueryRow(`SELECT id, public_id, company, country, email, status, balance_tokens, created_at, updated_at FROM customers WHERE public_id = ?`, publicID)
	var item model.Customer
	if err := row.Scan(&item.ID, &item.PublicID, &item.Company, &item.Country, &item.Email, &item.Status, &item.Balance, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return model.Customer{}, err
	}
	return item, nil
}

func (s *Store) UpdateCustomerBalance(publicID string, balance int64) error {
	result, err := s.db.Exec(`UPDATE customers SET balance_tokens = ?, updated_at = ? WHERE public_id = ?`, balance, time.Now().UTC(), publicID)
	if err != nil {
		return err
	}
	return checkAffected(result)
}

func (s *Store) CreateAPIKey(customerPublicID, name, scopes string) (model.APIKey, string, error) {
	var customerID int64
	if err := s.db.QueryRow(`SELECT id FROM customers WHERE public_id = ?`, customerPublicID).Scan(&customerID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.APIKey{}, "", fmt.Errorf("customer not found")
		}
		return model.APIKey{}, "", err
	}
	now := time.Now().UTC()
	rawKey := ""
	item := model.APIKey{
		CustomerID: customerID,
		Name:       defaultString(name, "Default API Key"),
		Scopes:     scopes,
		Status:     "Active",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	result, err := retryPublicID(func() {
		rawKey = "daxi_" + randomHex(24)
		item.PublicID = newPublicID("DXK")
		item.KeyHash = hashKey(rawKey)
		item.KeyPrefix = rawKey[:12]
	}, func() (sql.Result, error) {
		return s.db.Exec(`INSERT INTO api_keys(public_id, customer_id, name, key_hash, key_prefix, scopes, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			item.PublicID, item.CustomerID, item.Name, item.KeyHash, item.KeyPrefix, item.Scopes, item.Status, item.CreatedAt, item.UpdatedAt)
	})
	if err != nil {
		return model.APIKey{}, "", err
	}
	item.ID, _ = result.LastInsertId()
	return item, rawKey, nil
}

func (s *Store) ListAPIKeys(limit int) ([]model.APIKey, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT id, public_id, customer_id, name, key_hash, key_prefix, scopes, status, created_at, updated_at FROM api_keys ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.APIKey{}
	for rows.Next() {
		var item model.APIKey
		if err := rows.Scan(&item.ID, &item.PublicID, &item.CustomerID, &item.Name, &item.KeyHash, &item.KeyPrefix, &item.Scopes, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ListAPIKeysByCustomer(customerID int64) ([]model.APIKey, error) {
	rows, err := s.db.Query(`SELECT id, public_id, customer_id, name, key_hash, key_prefix, scopes, status, created_at, updated_at FROM api_keys WHERE customer_id = ? ORDER BY created_at DESC`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.APIKey{}
	for rows.Next() {
		var item model.APIKey
		if err := rows.Scan(&item.ID, &item.PublicID, &item.CustomerID, &item.Name, &item.KeyHash, &item.KeyPrefix, &item.Scopes, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) UpdateAPIKeyStatus(publicID, status string) error {
	result, err := s.db.Exec(`UPDATE api_keys SET status = ?, updated_at = ? WHERE public_id = ?`,
		canonicalStatus(defaultString(status, "Active"), "Active", "Disabled"), time.Now().UTC(), publicID)
	if err != nil {
		return err
	}
	return checkAffected(result)
}

func (s *Store) FindAPIKey(rawKey string) (model.APIKey, model.Customer, error) {
	hash := hashKey(strings.TrimSpace(rawKey))
	row := s.db.QueryRow(`SELECT k.id, k.public_id, k.customer_id, k.name, k.key_hash, k.key_prefix, k.scopes, k.status, k.created_at, k.updated_at,
		c.id, c.public_id, c.company, c.country, c.email, c.status, c.balance_tokens, c.created_at, c.updated_at
		FROM api_keys k JOIN customers c ON c.id = k.customer_id WHERE k.key_hash = ?`, hash)
	var key model.APIKey
	var customer model.Customer
	if err := row.Scan(&key.ID, &key.PublicID, &key.CustomerID, &key.Name, &key.KeyHash, &key.KeyPrefix, &key.Scopes, &key.Status, &key.CreatedAt, &key.UpdatedAt,
		&customer.ID, &customer.PublicID, &customer.Company, &customer.Country, &customer.Email, &customer.Status, &customer.Balance, &customer.CreatedAt, &customer.UpdatedAt); err != nil {
		return model.APIKey{}, model.Customer{}, err
	}
	return key, customer, nil
}

func (s *Store) RecordUsage(record model.UsageRecord) error {
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(`INSERT INTO usage_records(request_id, customer_id, api_key_id, scenario, model, prompt_tokens, completion_tokens, total_tokens, overspend_tokens, status_code, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.RequestID, record.CustomerID, record.APIKeyID, record.Scenario, record.Model, record.PromptTokens, record.CompletionTokens, record.TotalTokens, record.OverspendTokens, record.StatusCode, record.CreatedAt)
	return err
}

func (s *Store) DecreaseCustomerBalance(customerID, tokens int64) (int64, error) {
	if tokens <= 0 {
		return 0, nil
	}
	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return 0, err
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(ctx, `ROLLBACK`)
		}
	}()

	var balance int64
	if err := conn.QueryRowContext(ctx, `SELECT balance_tokens FROM customers WHERE id = ?`, customerID).Scan(&balance); err != nil {
		return 0, err
	}

	overspend := int64(0)
	nextBalance := balance - tokens
	if nextBalance < 0 {
		overspend = -nextBalance
		nextBalance = 0
	}

	result, err := conn.ExecContext(ctx, `UPDATE customers SET balance_tokens = ?, updated_at = ? WHERE id = ?`,
		nextBalance, time.Now().UTC(), customerID)
	if err != nil {
		return 0, err
	}
	if err := checkAffected(result); err != nil {
		return 0, err
	}
	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return 0, err
	}
	committed = true
	return overspend, nil
}

func (s *Store) UsageSummary() (model.UsageSummary, error) {
	row := s.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(prompt_tokens), 0), COALESCE(SUM(completion_tokens), 0), COALESCE(SUM(total_tokens), 0), COALESCE(SUM(overspend_tokens), 0) FROM usage_records`)
	var summary model.UsageSummary
	err := row.Scan(&summary.RequestCount, &summary.PromptTokens, &summary.CompletionTokens, &summary.TotalTokens, &summary.OverspendTokens)
	return summary, err
}

func (s *Store) CustomerUsageSummary(customerID int64) (model.UsageSummary, error) {
	row := s.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(prompt_tokens), 0), COALESCE(SUM(completion_tokens), 0), COALESCE(SUM(total_tokens), 0), COALESCE(SUM(overspend_tokens), 0) FROM usage_records WHERE customer_id = ?`, customerID)
	var summary model.UsageSummary
	err := row.Scan(&summary.RequestCount, &summary.PromptTokens, &summary.CompletionTokens, &summary.TotalTokens, &summary.OverspendTokens)
	return summary, err
}

func (s *Store) ListUsageRecords(limit int) ([]model.UsageRecordView, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	return s.listUsageRecords(``, 0, limit)
}

func (s *Store) ListUsageRecordsByCustomer(customerID int64, limit int) ([]model.UsageRecordView, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.listUsageRecords(`WHERE u.customer_id = ?`, customerID, limit)
}

func (s *Store) listUsageRecords(where string, customerID int64, limit int) ([]model.UsageRecordView, error) {
	query := `SELECT u.id, u.request_id, u.customer_id, u.api_key_id, u.scenario, u.model, u.prompt_tokens, u.completion_tokens, u.total_tokens, u.overspend_tokens, u.status_code, u.created_at,
		c.public_id, c.company, k.public_id, k.name
		FROM usage_records u
		JOIN customers c ON c.id = u.customer_id
		JOIN api_keys k ON k.id = u.api_key_id ` + where + ` ORDER BY u.created_at DESC LIMIT ?`
	args := []any{limit}
	if where != "" {
		args = []any{customerID, limit}
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.UsageRecordView{}
	for rows.Next() {
		var item model.UsageRecordView
		if err := rows.Scan(&item.ID, &item.RequestID, &item.CustomerID, &item.APIKeyID, &item.Scenario, &item.Model, &item.PromptTokens, &item.CompletionTokens, &item.TotalTokens, &item.OverspendTokens, &item.StatusCode, &item.CreatedAt,
			&item.CustomerPublicID, &item.CustomerCompany, &item.APIKeyPublicID, &item.APIKeyName); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ListTokenPlans() ([]model.TokenPlan, error) {
	rows, err := s.db.Query(`SELECT id, public_id, name, scenario_scope, tokens, price_cents, currency, status, created_at, updated_at FROM token_plans ORDER BY tokens ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.TokenPlan{}
	for rows.Next() {
		var item model.TokenPlan
		if err := rows.Scan(&item.ID, &item.PublicID, &item.Name, &item.ScenarioScope, &item.Tokens, &item.PriceCents, &item.Currency, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) CreateTokenOrder(customerPublicID, planPublicID, notes string) (model.TokenOrder, error) {
	customer, err := s.GetCustomer(customerPublicID)
	if err != nil {
		return model.TokenOrder{}, err
	}
	var plan model.TokenPlan
	if err := s.db.QueryRow(`SELECT id, public_id, name, scenario_scope, tokens, price_cents, currency, status, created_at, updated_at FROM token_plans WHERE public_id = ?`, planPublicID).
		Scan(&plan.ID, &plan.PublicID, &plan.Name, &plan.ScenarioScope, &plan.Tokens, &plan.PriceCents, &plan.Currency, &plan.Status, &plan.CreatedAt, &plan.UpdatedAt); err != nil {
		return model.TokenOrder{}, err
	}
	now := time.Now().UTC()
	order := model.TokenOrder{
		PublicID:    newPublicID("DXO"),
		CustomerID:  customer.ID,
		PlanID:      plan.ID,
		PlanName:    plan.Name,
		Tokens:      plan.Tokens,
		AmountCents: plan.PriceCents,
		Currency:    plan.Currency,
		Status:      "Pending",
		Notes:       notes,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	result, err := retryPublicID(func() { order.PublicID = newPublicID("DXO") }, func() (sql.Result, error) {
		return s.db.Exec(`INSERT INTO token_orders(public_id, customer_id, plan_id, plan_name, tokens, amount_cents, currency, status, notes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			order.PublicID, order.CustomerID, order.PlanID, order.PlanName, order.Tokens, order.AmountCents, order.Currency, order.Status, order.Notes, order.CreatedAt, order.UpdatedAt)
	})
	if err != nil {
		return model.TokenOrder{}, err
	}
	order.ID, _ = result.LastInsertId()
	return order, nil
}

func (s *Store) GetTokenOrder(publicID string) (model.TokenOrder, error) {
	row := s.db.QueryRow(`SELECT id, public_id, customer_id, plan_id, plan_name, tokens, amount_cents, currency, status, notes, created_at, updated_at FROM token_orders WHERE public_id = ?`, publicID)
	var item model.TokenOrder
	if err := row.Scan(&item.ID, &item.PublicID, &item.CustomerID, &item.PlanID, &item.PlanName, &item.Tokens, &item.AmountCents, &item.Currency, &item.Status, &item.Notes, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return model.TokenOrder{}, err
	}
	return item, nil
}

func (s *Store) ListTokenOrders(limit int) ([]model.TokenOrder, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT id, public_id, customer_id, plan_id, plan_name, tokens, amount_cents, currency, status, notes, created_at, updated_at FROM token_orders ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.TokenOrder{}
	for rows.Next() {
		var item model.TokenOrder
		if err := rows.Scan(&item.ID, &item.PublicID, &item.CustomerID, &item.PlanID, &item.PlanName, &item.Tokens, &item.AmountCents, &item.Currency, &item.Status, &item.Notes, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) UpdateTokenOrderStatus(publicID, status string) error {
	result, err := s.db.Exec(`UPDATE token_orders SET status = ?, updated_at = ? WHERE public_id = ?`,
		canonicalStatus(defaultString(status, "Pending"), "Pending", "Paid", "Cancelled"), time.Now().UTC(), publicID)
	if err != nil {
		return err
	}
	return checkAffected(result)
}

func (s *Store) CreateRecharge(customerPublicID string, tokens int64, source, referenceID, notes, operator string) (model.TokenRecharge, error) {
	if tokens <= 0 {
		return model.TokenRecharge{}, fmt.Errorf("tokens must be greater than 0")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return model.TokenRecharge{}, err
	}
	defer tx.Rollback()

	var customer model.Customer
	if err := tx.QueryRow(`SELECT id, public_id, company, country, email, status, balance_tokens, created_at, updated_at FROM customers WHERE public_id = ?`, customerPublicID).
		Scan(&customer.ID, &customer.PublicID, &customer.Company, &customer.Country, &customer.Email, &customer.Status, &customer.Balance, &customer.CreatedAt, &customer.UpdatedAt); err != nil {
		return model.TokenRecharge{}, err
	}
	balanceAfter := customer.Balance + tokens
	now := time.Now().UTC()
	if _, err := tx.Exec(`UPDATE customers SET balance_tokens = ?, updated_at = ? WHERE id = ?`, balanceAfter, now, customer.ID); err != nil {
		return model.TokenRecharge{}, err
	}
	item := model.TokenRecharge{
		PublicID:     newPublicID("DXR"),
		CustomerID:   customer.ID,
		CustomerCode: customer.PublicID,
		Tokens:       tokens,
		BalanceAfter: balanceAfter,
		Source:       defaultString(source, "manual"),
		ReferenceID:  referenceID,
		Notes:        notes,
		Operator:     defaultString(operator, "admin"),
		CreatedAt:    now,
	}
	result, err := tx.Exec(`INSERT INTO token_recharges(public_id, customer_id, tokens, balance_after, source, reference_id, notes, operator, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.PublicID, item.CustomerID, item.Tokens, item.BalanceAfter, item.Source, item.ReferenceID, item.Notes, item.Operator, item.CreatedAt)
	if err != nil {
		return model.TokenRecharge{}, err
	}
	item.ID, _ = result.LastInsertId()
	if err := tx.Commit(); err != nil {
		return model.TokenRecharge{}, err
	}
	return item, nil
}

func (s *Store) ListRecharges(limit int) ([]model.TokenRecharge, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	return s.listRecharges("", 0, limit)
}

func (s *Store) ListRechargesByCustomer(customerID int64, limit int) ([]model.TokenRecharge, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.listRecharges("WHERE r.customer_id = ?", customerID, limit)
}

func (s *Store) listRecharges(where string, customerID int64, limit int) ([]model.TokenRecharge, error) {
	query := `SELECT r.id, r.public_id, r.customer_id, c.public_id, r.tokens, r.balance_after, r.source, r.reference_id, r.notes, r.operator, r.created_at
		FROM token_recharges r JOIN customers c ON c.id = r.customer_id ` + where + ` ORDER BY r.created_at DESC LIMIT ?`
	args := []any{limit}
	if where != "" {
		args = []any{customerID, limit}
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.TokenRecharge{}
	for rows.Next() {
		var item model.TokenRecharge
		if err := rows.Scan(&item.ID, &item.PublicID, &item.CustomerID, &item.CustomerCode, &item.Tokens, &item.BalanceAfter, &item.Source, &item.ReferenceID, &item.Notes, &item.Operator, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ListModelRoutes() ([]model.ModelRoute, error) {
	rows, err := s.db.Query(`SELECT id, public_id, scenario, primary_model, fallback_models, max_tokens_per_request, status, notes, created_at, updated_at FROM model_routes ORDER BY scenario ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.ModelRoute{}
	for rows.Next() {
		var item model.ModelRoute
		if err := rows.Scan(&item.ID, &item.PublicID, &item.Scenario, &item.PrimaryModel, &item.FallbackModels, &item.MaxTokensPerRequest, &item.Status, &item.Notes, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetModelRouteByScenario(scenario string) (model.ModelRoute, error) {
	row := s.db.QueryRow(`SELECT id, public_id, scenario, primary_model, fallback_models, max_tokens_per_request, status, notes, created_at, updated_at FROM model_routes WHERE scenario = ?`, strings.TrimSpace(scenario))
	var item model.ModelRoute
	if err := row.Scan(&item.ID, &item.PublicID, &item.Scenario, &item.PrimaryModel, &item.FallbackModels, &item.MaxTokensPerRequest, &item.Status, &item.Notes, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return model.ModelRoute{}, err
	}
	return item, nil
}

func (s *Store) UpsertModelRoute(route *model.ModelRoute) error {
	if strings.TrimSpace(route.Scenario) == "" || strings.TrimSpace(route.PrimaryModel) == "" {
		return fmt.Errorf("scenario and primary_model are required")
	}
	now := time.Now().UTC()
	if route.PublicID == "" {
		route.PublicID = newPublicID("DXM")
	}
	route.Status = canonicalStatus(defaultString(route.Status, "Active"), "Active", "Disabled")
	route.CreatedAt = now
	route.UpdatedAt = now
	_, err := s.db.Exec(`INSERT INTO model_routes(public_id, scenario, primary_model, fallback_models, max_tokens_per_request, status, notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(scenario) DO UPDATE SET primary_model = excluded.primary_model, fallback_models = excluded.fallback_models, max_tokens_per_request = excluded.max_tokens_per_request, status = excluded.status, notes = excluded.notes, updated_at = excluded.updated_at`,
		route.PublicID, route.Scenario, route.PrimaryModel, route.FallbackModels, route.MaxTokensPerRequest, route.Status, route.Notes, route.CreatedAt, route.UpdatedAt)
	return err
}

func (s *Store) UpdateModelRoute(publicID string, route model.ModelRoute) error {
	result, err := s.db.Exec(`UPDATE model_routes SET primary_model = ?, fallback_models = ?, max_tokens_per_request = ?, status = ?, notes = ?, updated_at = ? WHERE public_id = ?`,
		route.PrimaryModel, route.FallbackModels, route.MaxTokensPerRequest, canonicalStatus(defaultString(route.Status, "Active"), "Active", "Disabled"), route.Notes, time.Now().UTC(), publicID)
	if err != nil {
		return err
	}
	return checkAffected(result)
}

func (s *Store) CreateVideoTask(task *model.VideoTask) error {
	now := time.Now().UTC()
	task.PublicID = newPublicID("DXV")
	task.TaskType = defaultString(task.TaskType, "short-drama")
	task.Language = defaultString(task.Language, "en")
	task.Status = defaultString(task.Status, "Queued")
	if task.Progress < 0 {
		task.Progress = 0
	}
	if task.Progress > 100 {
		task.Progress = 100
	}
	task.CreatedAt = now
	task.UpdatedAt = now
	result, err := retryPublicID(func() { task.PublicID = newPublicID("DXV") }, func() (sql.Result, error) {
		return s.db.Exec(`INSERT INTO video_tasks(public_id, task_type, customer_public_id, title, prompt, product_name, script, language, status, progress, result_url, error_message, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			task.PublicID, task.TaskType, task.CustomerPublicID, task.Title, task.Prompt, task.ProductName, task.Script, task.Language, task.Status, task.Progress, task.ResultURL, task.ErrorMessage, task.CreatedAt, task.UpdatedAt)
	})
	if err != nil {
		return err
	}
	task.ID, _ = result.LastInsertId()
	return nil
}

func (s *Store) ListVideoTasks(limit int) ([]model.VideoTask, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT id, public_id, task_type, customer_public_id, title, prompt, product_name, script, language, status, progress, result_url, error_message, created_at, updated_at FROM video_tasks ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.VideoTask{}
	for rows.Next() {
		var item model.VideoTask
		if err := rows.Scan(&item.ID, &item.PublicID, &item.TaskType, &item.CustomerPublicID, &item.Title, &item.Prompt, &item.ProductName, &item.Script, &item.Language, &item.Status, &item.Progress, &item.ResultURL, &item.ErrorMessage, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) UpdateVideoTask(publicID string, task model.VideoTask) error {
	if task.Progress < 0 {
		task.Progress = 0
	}
	if task.Progress > 100 {
		task.Progress = 100
	}
	result, err := s.db.Exec(`UPDATE video_tasks SET status = ?, progress = ?, result_url = ?, error_message = ?, updated_at = ? WHERE public_id = ?`,
		defaultString(task.Status, "Queued"), task.Progress, task.ResultURL, task.ErrorMessage, time.Now().UTC(), publicID)
	if err != nil {
		return err
	}
	return checkAffected(result)
}

func (s *Store) CreatePaymentRecord(item *model.PaymentRecord) error {
	now := time.Now().UTC()
	item.PublicID = newPublicID("DXY")
	item.Provider = defaultString(item.Provider, "manual")
	item.Currency = defaultString(item.Currency, "USD")
	item.Status = canonicalStatus(defaultString(item.Status, "Pending"), "Pending", "Paid", "Failed", "Cancelled")
	item.CreatedAt = now
	item.UpdatedAt = now
	result, err := retryPublicID(func() { item.PublicID = newPublicID("DXY") }, func() (sql.Result, error) {
		return s.db.Exec(`INSERT INTO payment_records(public_id, order_public_id, customer_public_id, provider, amount_cents, currency, status, reference_id, notes, paid_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			item.PublicID, item.OrderPublicID, item.CustomerPublicID, item.Provider, item.AmountCents, item.Currency, item.Status, item.ReferenceID, item.Notes, item.PaidAt, item.CreatedAt, item.UpdatedAt)
	})
	if err != nil {
		return err
	}
	item.ID, _ = result.LastInsertId()
	return nil
}

func (s *Store) GetPaymentRecord(publicID string) (model.PaymentRecord, error) {
	row := s.db.QueryRow(`SELECT id, public_id, order_public_id, customer_public_id, provider, amount_cents, currency, status, reference_id, notes, paid_at, created_at, updated_at FROM payment_records WHERE public_id = ?`, publicID)
	return scanPaymentRecord(row)
}

func (s *Store) ListPaymentRecords(limit int) ([]model.PaymentRecord, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT id, public_id, order_public_id, customer_public_id, provider, amount_cents, currency, status, reference_id, notes, paid_at, created_at, updated_at FROM payment_records ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.PaymentRecord{}
	for rows.Next() {
		item, err := scanPaymentRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) UpdatePaymentStatus(publicID, status string) error {
	_, _, err := s.SettlePaymentStatus(publicID, status, "admin")
	return err
}

func (s *Store) SettlePaymentStatus(publicID, status, operator string) (model.PaymentRecord, *model.TokenRecharge, error) {
	status = canonicalStatus(defaultString(status, "Pending"), "Pending", "Paid", "Failed", "Cancelled")
	tx, err := s.db.Begin()
	if err != nil {
		return model.PaymentRecord{}, nil, err
	}
	defer tx.Rollback()

	payment, err := scanPaymentRecord(tx.QueryRow(`SELECT id, public_id, order_public_id, customer_public_id, provider, amount_cents, currency, status, reference_id, notes, paid_at, created_at, updated_at FROM payment_records WHERE public_id = ?`, publicID))
	if err != nil {
		return model.PaymentRecord{}, nil, err
	}

	now := time.Now().UTC()
	var paidAt any
	if strings.EqualFold(status, "Paid") {
		paidAt = now
	}
	result, err := tx.Exec(`UPDATE payment_records SET status = ?, paid_at = COALESCE(?, paid_at), updated_at = ? WHERE public_id = ?`, status, paidAt, now, publicID)
	if err != nil {
		return model.PaymentRecord{}, nil, err
	}
	if err := checkAffected(result); err != nil {
		return model.PaymentRecord{}, nil, err
	}

	payment, err = scanPaymentRecord(tx.QueryRow(`SELECT id, public_id, order_public_id, customer_public_id, provider, amount_cents, currency, status, reference_id, notes, paid_at, created_at, updated_at FROM payment_records WHERE public_id = ?`, publicID))
	if err != nil {
		return model.PaymentRecord{}, nil, err
	}
	if !strings.EqualFold(status, "Paid") || strings.TrimSpace(payment.OrderPublicID) == "" {
		if err := tx.Commit(); err != nil {
			return model.PaymentRecord{}, nil, err
		}
		return payment, nil, nil
	}

	var existingRecharge model.TokenRecharge
	err = tx.QueryRow(`SELECT r.id, r.public_id, r.customer_id, c.public_id, r.tokens, r.balance_after, r.source, r.reference_id, r.notes, r.operator, r.created_at
		FROM token_recharges r JOIN customers c ON c.id = r.customer_id WHERE r.source = ? AND r.reference_id = ?`, "payment", payment.PublicID).
		Scan(&existingRecharge.ID, &existingRecharge.PublicID, &existingRecharge.CustomerID, &existingRecharge.CustomerCode, &existingRecharge.Tokens, &existingRecharge.BalanceAfter, &existingRecharge.Source, &existingRecharge.ReferenceID, &existingRecharge.Notes, &existingRecharge.Operator, &existingRecharge.CreatedAt)
	if err == nil {
		if err := tx.Commit(); err != nil {
			return model.PaymentRecord{}, nil, err
		}
		return payment, &existingRecharge, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return model.PaymentRecord{}, nil, err
	}

	var order model.TokenOrder
	if err := tx.QueryRow(`SELECT id, public_id, customer_id, plan_id, plan_name, tokens, amount_cents, currency, status, notes, created_at, updated_at FROM token_orders WHERE public_id = ?`, payment.OrderPublicID).
		Scan(&order.ID, &order.PublicID, &order.CustomerID, &order.PlanID, &order.PlanName, &order.Tokens, &order.AmountCents, &order.Currency, &order.Status, &order.Notes, &order.CreatedAt, &order.UpdatedAt); err != nil {
		return model.PaymentRecord{}, nil, err
	}
	if !strings.EqualFold(payment.Currency, order.Currency) {
		return model.PaymentRecord{}, nil, fmt.Errorf("payment currency does not match token order")
	}
	if payment.AmountCents < order.AmountCents {
		return model.PaymentRecord{}, nil, fmt.Errorf("payment amount is lower than token order amount")
	}

	var customer model.Customer
	if err := tx.QueryRow(`SELECT id, public_id, company, country, email, status, balance_tokens, created_at, updated_at FROM customers WHERE id = ?`, order.CustomerID).
		Scan(&customer.ID, &customer.PublicID, &customer.Company, &customer.Country, &customer.Email, &customer.Status, &customer.Balance, &customer.CreatedAt, &customer.UpdatedAt); err != nil {
		return model.PaymentRecord{}, nil, err
	}

	balanceAfter := customer.Balance + order.Tokens
	if _, err := tx.Exec(`UPDATE customers SET balance_tokens = ?, updated_at = ? WHERE id = ?`, balanceAfter, now, customer.ID); err != nil {
		return model.PaymentRecord{}, nil, err
	}
	recharge := model.TokenRecharge{
		PublicID:     newPublicID("DXR"),
		CustomerID:   customer.ID,
		CustomerCode: customer.PublicID,
		Tokens:       order.Tokens,
		BalanceAfter: balanceAfter,
		Source:       "payment",
		ReferenceID:  payment.PublicID,
		Notes:        "Token order " + order.PublicID + " settled by payment " + payment.PublicID,
		Operator:     defaultString(operator, "admin"),
		CreatedAt:    now,
	}
	result, err = tx.Exec(`INSERT INTO token_recharges(public_id, customer_id, tokens, balance_after, source, reference_id, notes, operator, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		recharge.PublicID, recharge.CustomerID, recharge.Tokens, recharge.BalanceAfter, recharge.Source, recharge.ReferenceID, recharge.Notes, recharge.Operator, recharge.CreatedAt)
	if err != nil {
		return model.PaymentRecord{}, nil, err
	}
	recharge.ID, _ = result.LastInsertId()
	if _, err := tx.Exec(`UPDATE token_orders SET status = ?, updated_at = ? WHERE id = ?`, "Paid", now, order.ID); err != nil {
		return model.PaymentRecord{}, nil, err
	}
	if err := tx.Commit(); err != nil {
		return model.PaymentRecord{}, nil, err
	}
	return payment, &recharge, nil
}

type paymentScanner interface {
	Scan(dest ...any) error
}

func scanPaymentRecord(scanner paymentScanner) (model.PaymentRecord, error) {
	var item model.PaymentRecord
	var paidAt sql.NullTime
	if err := scanner.Scan(&item.ID, &item.PublicID, &item.OrderPublicID, &item.CustomerPublicID, &item.Provider, &item.AmountCents, &item.Currency, &item.Status, &item.ReferenceID, &item.Notes, &paidAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return model.PaymentRecord{}, err
	}
	if paidAt.Valid {
		item.PaidAt = &paidAt.Time
	}
	return item, nil
}

func hashKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func hashPassword(password, salt string) string {
	sum := sha256.Sum256([]byte(salt + ":" + password))
	return hex.EncodeToString(sum[:])
}

func randomHex(bytes int) string {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}

func newPublicID(prefix string) string {
	return prefix + "-" + strings.ToUpper(randomHex(8))
}

func retryPublicID(assign func(), insert func() (sql.Result, error)) (sql.Result, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		assign()
		result, err := insert()
		if err == nil || !isUniqueConstraint(err) {
			return result, err
		}
		lastErr = err
	}
	return nil, lastErr
}

func isUniqueConstraint(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func canonicalStatus(value string, allowed ...string) string {
	for _, candidate := range allowed {
		if strings.EqualFold(strings.TrimSpace(value), candidate) {
			return candidate
		}
	}
	return strings.TrimSpace(value)
}

func checkAffected(result sql.Result) error {
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}
