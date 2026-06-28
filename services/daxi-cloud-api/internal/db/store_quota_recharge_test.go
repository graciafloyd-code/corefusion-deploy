package db

import (
	"testing"

	"daxi-cloud-api/internal/model"
)

// CreateQuotaRecharge 只能注资「视频额度(quota)」钱包,绝不可碰「模型额度(token)」钱包;反向亦然。
// 两钱包物理隔离、单位不同、不可相加,这是乙方案的硬约束。
func TestCreateQuotaRecharge_FundsOnlyQuotaWallet(t *testing.T) {
	store := openTestStore(t)
	cust := &model.Customer{Company: "Wallet Iso Co", Status: "Active"}
	if err := store.CreateCustomer(cust); err != nil {
		t.Fatalf("create customer: %v", err)
	}

	// 给 quota 钱包充值 500000
	r1, err := store.CreateQuotaRecharge(cust.PublicID, 500_000, "manual", "", "", "admin")
	if err != nil {
		t.Fatalf("quota recharge: %v", err)
	}
	if r1.Wallet != "quota" {
		t.Fatalf("recharge wallet=%q want quota", r1.Wallet)
	}
	if r1.BalanceAfter != 500_000 {
		t.Fatalf("quota balance_after=%d want 500000", r1.BalanceAfter)
	}
	c, _ := store.GetCustomer(cust.PublicID)
	if c.BalanceQuota != 500_000 {
		t.Fatalf("balance_quota=%d want 500000", c.BalanceQuota)
	}
	if c.Balance != 0 {
		t.Fatalf("quota recharge leaked into token wallet: balance_tokens=%d want 0", c.Balance)
	}

	// 给 token 钱包充值 300,不得影响 quota 钱包
	r2, err := store.CreateRecharge(cust.PublicID, 300, "manual", "", "", "admin")
	if err != nil {
		t.Fatalf("token recharge: %v", err)
	}
	if r2.Wallet != "token" {
		t.Fatalf("recharge wallet=%q want token", r2.Wallet)
	}
	c, _ = store.GetCustomer(cust.PublicID)
	if c.Balance != 300 {
		t.Fatalf("balance_tokens=%d want 300", c.Balance)
	}
	if c.BalanceQuota != 500_000 {
		t.Fatalf("token recharge disturbed quota wallet: balance_quota=%d want 500000", c.BalanceQuota)
	}

	// 二次 quota 充值应累加
	if _, err := store.CreateQuotaRecharge(cust.PublicID, 100_000, "manual", "", "", "admin"); err != nil {
		t.Fatalf("second quota recharge: %v", err)
	}
	c, _ = store.GetCustomer(cust.PublicID)
	if c.BalanceQuota != 600_000 {
		t.Fatalf("balance_quota after accumulation=%d want 600000", c.BalanceQuota)
	}
}
