package main

import (
	"net/http"
	"strconv"
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
	if err != nil || amount <= 0 || date == "" || (typ != "income" && typ != "expense") {
		http.Error(w, "invalid input", 400)
		return
	}

	_, err = db.Exec(`INSERT INTO transactions(account_id, date, type, category, amount, memo) VALUES (?, ?, ?, ?, ?, ?)`,
		accountID, date, typ, category, amount, memo)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	if r.FormValue("save_template") == "on" {
		var maxOrder int
		db.QueryRow(`SELECT COALESCE(MAX(sort_order),-1) FROM templates`).Scan(&maxOrder)
		db.Exec(`INSERT INTO templates(account_id, type, category, amount, memo, sort_order) VALUES (?, ?, ?, ?, ?, ?)`,
			accountID, typ, category, amount, memo, maxOrder+1)
	}

	month := date[:7]
	http.Redirect(w, r, "/?month="+month, http.StatusSeeOther)
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, err := db.Exec(`DELETE FROM transactions WHERE id=?`, id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirect := r.URL.Query().Get("redirect")
	if redirect == "" {
		redirect = "/"
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}
