package app

import "testing"

func TestFilterUnspent(t *testing.T) {
	all := []Template{
		{ID: 1, Type: "expense", Category: "통신", Amount: 170000, Memo: "통신비"},
		{ID: 2, Type: "expense", Category: "구독", Amount: 30000, Memo: "Claude"},
		{ID: 3, Type: "expense", Category: "식비", Amount: 9000}, // no memo
		{ID: 4, Type: "income", Category: "급여", Amount: 2000000, Memo: "월급"},
	}
	spent := map[string]bool{
		templateKey("expense", "통신", 170000, "통신비"): true,
		templateKey("expense", "식비", 9000, ""):      true,
	}

	got := filterUnspent(all, spent)
	if len(got) != 2 {
		t.Fatalf("got %d templates, want 2: %+v", len(got), got)
	}
	if got[0].ID != 2 || got[1].ID != 4 {
		t.Errorf("wrong templates survived: %d, %d", got[0].ID, got[1].ID)
	}
}

// A nil map means the lookup query failed; nothing should be treated as spent.
func TestFilterUnspentNilMap(t *testing.T) {
	all := []Template{{ID: 1, Type: "expense", Category: "통신", Amount: 1}}
	if got := filterUnspent(all, nil); len(got) != 1 {
		t.Errorf("got %d, want all 1 template back", len(got))
	}
}

// Templates that differ only in memo must not collapse into one key.
func TestTemplateKeyDistinguishesMemo(t *testing.T) {
	a := templateKey("expense", "구독", 30000, "Claude")
	b := templateKey("expense", "구독", 30000, "")
	if a == b {
		t.Error("memo should be part of the key")
	}
	if templateKey("expense", "구독", 30000, "x") == templateKey("income", "구독", 30000, "x") {
		t.Error("type should be part of the key")
	}
}
