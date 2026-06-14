package db

import (
	"path/filepath"
	"testing"

	"daxi-cloud-api/internal/model"
)

func TestSettlePaidPaymentPostsTokenOrderRechargeOnce(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

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
