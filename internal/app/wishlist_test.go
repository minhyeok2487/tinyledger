package app

import "testing"

func TestMarkAffordable(t *testing.T) {
	items := []WishItem{
		{Name: "프라모델", Price: 120000},
		{Name: "모니터", Price: 900000},
		{Name: "딱 맞는 것", Price: 500000},
		{Name: "가격 미정", Price: 0},
	}
	got := markAffordable(items, 500000)

	if !got[0].Affordable {
		t.Error("120,000 should be affordable with 500,000")
	}
	if got[1].Affordable {
		t.Error("900,000 should not be affordable")
	}
	if got[1].Short != 400000 {
		t.Errorf("Short = %d, want 400000", got[1].Short)
	}
	if !got[2].Affordable {
		t.Error("a price equal to spendable should count as affordable")
	}
	if got[3].Affordable {
		t.Error("an item with no price set should not be marked affordable")
	}
	// Order is the user's and must survive untouched.
	if got[0].Name != "프라모델" || got[3].Name != "가격 미정" {
		t.Error("markAffordable reordered the list")
	}
}

func TestMarkAffordableNegativeSpendable(t *testing.T) {
	got := markAffordable([]WishItem{{Price: 1000}}, -50000)
	if got[0].Affordable {
		t.Error("nothing is affordable when spendable is negative")
	}
	if got[0].Short != 51000 {
		t.Errorf("Short = %d, want 51000", got[0].Short)
	}
}

func TestSafeWishURL(t *testing.T) {
	tests := map[string]string{
		"https://example.com/a": "https://example.com/a",
		"http://example.com":    "http://example.com",
		"  https://x.com  ":     "https://x.com",
		"javascript:alert(1)":   "",
		"data:text/html,x":      "",
		"example.com":           "", // no scheme
		"":                      "",
		"https://":              "", // no host
	}
	for in, want := range tests {
		if got := safeWishURL(in); got != want {
			t.Errorf("safeWishURL(%q) = %q, want %q", in, got, want)
		}
	}
}
