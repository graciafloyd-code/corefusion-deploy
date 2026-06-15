package db

import (
	"path/filepath"
	"strings"
	"testing"

	"daxi-cloud-api/internal/model"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return store
}

func TestSettlePaidPaymentPostsTokenOrderRechargeOnce(t *testing.T) {
	store := openTestStore(t)

	customer := &model.Customer{Company: "ACME", Balance: 100}
	if err := store.CreateCustomer(customer); err != nil {
		t.Fatalf("create customer: %v", err)
	}
	order, err := store.CreateTokenOrder(customer.PublicID, "DXP-TRIAL", "launch pack")
	if err != nil {
		t.Fatalf("create token order: %v", err)
	}
	payment := model.PaymentRecord{
		OrderPublicID:    order.PublicID,
		CustomerPublicID: customer.PublicID,
		Provider:         "manual",
		AmountCents:      order.AmountCents,
		Currency:         order.Currency,
		Status:           "Pending",
	}
	if err := store.CreatePaymentRecord(&payment); err != nil {
		t.Fatalf("create payment: %v", err)
	}

	updatedPayment, recharge, err := store.SettlePaymentStatus(payment.PublicID, "paid", "tester")
	if err != nil {
		t.Fatalf("settle payment: %v", err)
	}
	if updatedPayment.Status != "Paid" {
		t.Fatalf("payment status = %q, want Paid", updatedPayment.Status)
	}
	if updatedPayment.PaidAt == nil {
		t.Fatalf("payment paid_at was not set")
	}
	if recharge == nil {
		t.Fatalf("expected recharge from paid payment")
	}
	if recharge.Tokens != order.Tokens {
		t.Fatalf("recharge tokens = %d, want %d", recharge.Tokens, order.Tokens)
	}

	storedCustomer, err := store.GetCustomer(customer.PublicID)
	if err != nil {
		t.Fatalf("get customer: %v", err)
	}
	if storedCustomer.Balance != customer.Balance+order.Tokens {
		t.Fatalf("balance = %d, want %d", storedCustomer.Balance, customer.Balance+order.Tokens)
	}
	storedOrder, err := store.GetTokenOrder(order.PublicID)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if storedOrder.Status != "Paid" {
		t.Fatalf("order status = %q, want Paid", storedOrder.Status)
	}

	_, secondRecharge, err := store.SettlePaymentStatus(payment.PublicID, "Paid", "tester")
	if err != nil {
		t.Fatalf("settle payment again: %v", err)
	}
	if secondRecharge == nil || secondRecharge.PublicID != recharge.PublicID {
		t.Fatalf("second settlement did not return existing recharge")
	}
	recharges, err := store.ListRechargesByCustomer(customer.ID, 10)
	if err != nil {
		t.Fatalf("list recharges: %v", err)
	}
	if len(recharges) != 1 {
		t.Fatalf("recharge count = %d, want 1", len(recharges))
	}
	storedCustomer, err = store.GetCustomer(customer.PublicID)
	if err != nil {
		t.Fatalf("get customer after second settlement: %v", err)
	}
	if storedCustomer.Balance != customer.Balance+order.Tokens {
		t.Fatalf("balance after second settlement = %d, want %d", storedCustomer.Balance, customer.Balance+order.Tokens)
	}
}

func TestLeadActivitiesCaptureManualAndStatusFollowUp(t *testing.T) {
	store := openTestStore(t)
	lead := &model.Lead{
		Source:       "get-started",
		Scenario:     "Model API Access",
		Company:      "DAXI Test Customer",
		Email:        "buyer@example.com",
		UsageProfile: "Chat and image models",
	}
	if err := store.CreateLead(lead); err != nil {
		t.Fatalf("create lead: %v", err)
	}

	activity, err := store.CreateLeadActivity(lead.PublicID, model.LeadActivity{
		Actor:    "operator",
		Action:   "follow_up",
		Note:     "Customer asked for token package pricing.",
		NextStep: "Send package sheet.",
	})
	if err != nil {
		t.Fatalf("create lead activity: %v", err)
	}
	if activity.PublicID == "" || activity.LeadPublicID != lead.PublicID {
		t.Fatalf("unexpected activity identity: %#v", activity)
	}
	if activity.Actor != "operator" || activity.Action != "follow_up" {
		t.Fatalf("unexpected activity metadata: %#v", activity)
	}

	if _, err := store.CreateLeadActivity(lead.PublicID, model.LeadActivity{}); err == nil || !strings.Contains(err.Error(), "note or next_step") {
		t.Fatalf("empty activity error = %v, want validation error", err)
	}

	if err := store.UpdateLeadStatus(lead.PublicID, "Contacted"); err != nil {
		t.Fatalf("update lead status: %v", err)
	}
	activities, err := store.ListLeadActivities(lead.PublicID, 10)
	if err != nil {
		t.Fatalf("list activities: %v", err)
	}
	if len(activities) != 2 {
		t.Fatalf("activity count = %d, want 2", len(activities))
	}
	if activities[0].Action != "status.updated" || !strings.Contains(activities[0].Note, "Contacted") {
		t.Fatalf("latest activity = %#v, want status update", activities[0])
	}
	if activities[1].Action != "follow_up" || activities[1].NextStep != "Send package sheet." {
		t.Fatalf("older activity = %#v, want manual follow-up", activities[1])
	}
}
