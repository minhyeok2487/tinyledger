package app

import "testing"

func TestComputeSpendable(t *testing.T) {
	accounts := []Account{
		{ID: 1, Balance: 1000000},
		{ID: 2, Balance: 700000, Excluded: true}, // 비상금
	}
	unspent := []Template{
		{AccountID: 1, Type: "expense", Amount: 170000}, // 통신비
		{AccountID: 1, Type: "income", Amount: 2000000}, // 월급 — 고정비가 아니다
		{AccountID: 2, Type: "expense", Amount: 500000}, // 제외 계좌에서 나감
	}

	got := computeSpendable(accounts, unspent)
	if got.Available != 1000000 {
		t.Errorf("Available = %d, want 1000000 (제외 계좌 빠져야)", got.Available)
	}
	if got.UpcomingFixed != 170000 {
		t.Errorf("UpcomingFixed = %d, want 170000 (수입·제외계좌 고정비 빼야)", got.UpcomingFixed)
	}
	if got.Spendable != 830000 {
		t.Errorf("Spendable = %d, want 830000", got.Spendable)
	}
	if got.AllExcluded {
		t.Error("AllExcluded should be false when one account is usable")
	}
}

func TestComputeSpendableAllExcluded(t *testing.T) {
	got := computeSpendable([]Account{{ID: 1, Balance: 500, Excluded: true}}, nil)
	if !got.AllExcluded {
		t.Error("AllExcluded should be true")
	}
	if got.Available != 0 {
		t.Errorf("Available = %d, want 0", got.Available)
	}
}

func TestComputeSpendableNoAccounts(t *testing.T) {
	// An empty ledger is not "everything is excluded" — the hint would be wrong.
	if computeSpendable(nil, nil).AllExcluded {
		t.Error("AllExcluded should be false with no accounts at all")
	}
}

func TestComputeSpendableCanGoNegative(t *testing.T) {
	got := computeSpendable(
		[]Account{{ID: 1, Balance: 10000}},
		[]Template{{AccountID: 1, Type: "expense", Amount: 40000}},
	)
	if got.Spendable != -30000 {
		t.Errorf("Spendable = %d, want -30000", got.Spendable)
	}
}
