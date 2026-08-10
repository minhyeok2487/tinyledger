package app

import "testing"

func tx(id, amount, itemID int64, itemName string) hobbyTx {
	return hobbyTx{
		Transaction: Transaction{ID: id, Amount: amount},
		ItemID:      itemID,
		ItemName:    itemName,
	}
}

func TestGroupHobby(t *testing.T) {
	groups, total := groupHobby([]hobbyTx{
		tx(1, 15000, 1, "명조"),
		tx(2, 45000, 2, "프라모델"),
		tx(3, 30000, 0, ""),
		tx(4, 5000, 1, "명조"),
	})

	if total != 95000 {
		t.Errorf("total = %d, want 95000", total)
	}
	// Biggest spend first, and 미분류 sinks to the bottom even though its
	// 30,000 outranks 명조's 20,000 — it's a to-do, not a category.
	want := []string{"프라모델", "명조", "미분류"}
	if len(groups) != len(want) {
		t.Fatalf("got %d groups, want %d: %+v", len(groups), len(want), groups)
	}
	for i, name := range want {
		if groups[i].Name != name {
			t.Errorf("group %d = %q, want %q", i, groups[i].Name, name)
		}
	}
	if groups[1].Total != 20000 {
		t.Errorf("명조 total = %d, want 20000 (two transactions summed)", groups[1].Total)
	}
	if len(groups[1].Txs) != 2 {
		t.Errorf("명조 kept %d transactions, want 2", len(groups[1].Txs))
	}
	if pct := groups[0].Percent; pct < 47 || pct > 48 {
		t.Errorf("프라모델 percent = %v, want ~47.4", pct)
	}
}

func TestGroupHobbyEmpty(t *testing.T) {
	groups, total := groupHobby(nil)
	if len(groups) != 0 || total != 0 {
		t.Errorf("got %d groups / total %d, want empty", len(groups), total)
	}
}

// A zero total must not produce NaN or a divide-by-zero panic.
func TestGroupHobbyZeroTotal(t *testing.T) {
	groups, total := groupHobby([]hobbyTx{tx(1, 0, 1, "명조")})
	if total != 0 {
		t.Fatalf("total = %d, want 0", total)
	}
	if groups[0].Percent != 0 {
		t.Errorf("percent = %v, want 0", groups[0].Percent)
	}
}

func TestGroupHobbyKeepsArchivedFlag(t *testing.T) {
	in := tx(1, 100, 7, "옛날취미")
	in.Archived = true
	groups, _ := groupHobby([]hobbyTx{in})
	if !groups[0].Archived {
		t.Error("archived flag lost — the group would look like a live item")
	}
}
