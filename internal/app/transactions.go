package app

import (
	"net/http"
	"strconv"
	"time"
)

func handleAdd(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	date := r.FormValue("date")
	typ := r.FormValue("type")
	category := r.FormValue("category")
	memo := r.FormValue("memo")
	amountStr := r.FormValue("amount")
	accountStr := r.FormValue("account_id")

	amount, err := strconv.ParseInt(amountStr, 10, 64)
	accountID, _ := strconv.ParseInt(accountStr, 10, 64)
	if accountID == 0 {
		accountID = 1
	}
	// The date must be a full YYYY-MM-DD: month views compare it as a range,
	// so a short value would store a row that no month can ever show.
	if _, dErr := time.Parse("2006-01-02", date); dErr != nil {
		http.Error(w, "invalid date", 400)
		return
	}
	if err != nil || amount <= 0 || (typ != "income" && typ != "expense") {
		http.Error(w, "invalid input", 400)
		return
	}

	// The sub-item only means anything for 여가 expenses; anything else
	// stores NULL so the 여가 tab never shows a stray bucket.
	var hobbyItemID int64
	if typ == "expense" && category == hobbyCategory {
		hobbyItemID, _ = strconv.ParseInt(r.FormValue("hobby_item_id"), 10, 64)
	}
	_, err = db.Exec(`INSERT INTO transactions(account_id, date, type, category, amount, memo, hobby_item_id)
		VALUES (`+accountIDOrDefault+`, ?, ?, ?, ?, ?, `+hobbyItemOrNull+`)`,
		accountID, date, typ, category, amount, memo, hobbyItemID)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	if r.FormValue("save_template") == "on" {
		db.Exec(insertTemplateSQL, accountID, typ, category, amount, memo)
	}

	month := date[:7]
	http.Redirect(w, r, "/?month="+month, http.StatusSeeOther)
}

// handleTransactionUpdate is the only general-purpose edit route in the app
// (the 여가 tab's assign handler deliberately touches one column only). The
// edit modal posts here with the same field set as /add, plus a redirect
// target so it can send the user back to whatever filtered view they were on.
func handleTransactionUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	id := r.PathValue("id")
	date := r.FormValue("date")
	typ := r.FormValue("type")
	category := r.FormValue("category")
	memo := r.FormValue("memo")
	amount, err := strconv.ParseInt(r.FormValue("amount"), 10, 64)
	accountID, _ := strconv.ParseInt(r.FormValue("account_id"), 10, 64)

	if _, dErr := time.Parse("2006-01-02", date); dErr != nil {
		http.Error(w, "invalid date", 400)
		return
	}
	if err != nil || amount <= 0 || (typ != "income" && typ != "expense") {
		http.Error(w, "invalid input", 400)
		return
	}

	var hobbyItemID int64
	if typ == "expense" && category == hobbyCategory {
		hobbyItemID, _ = strconv.ParseInt(r.FormValue("hobby_item_id"), 10, 64)
	}

	_, err = db.Exec(`UPDATE transactions SET
			account_id = `+accountIDOrDefault+`,
			date = ?, type = ?, category = ?, amount = ?, memo = ?,
			hobby_item_id = `+hobbyItemOrNull+`
		WHERE id = ?`,
		accountID, date, typ, category, amount, memo, hobbyItemID, id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, safeNext(r.FormValue("redirect")), http.StatusSeeOther)
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, err := db.Exec(`DELETE FROM transactions WHERE id=?`, id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, safeNext(r.URL.Query().Get("redirect")), http.StatusSeeOther)
}
