package main

import (
	"database/sql"
	"net/http"
	"strconv"
)

func handleAccounts(w http.ResponseWriter, r *http.Request) {
	accounts := listAccounts()
	balances := map[int64]int64{}
	for _, a := range accounts {
		var income, expense sql.NullInt64
		db.QueryRow(`SELECT SUM(CASE WHEN type='income' THEN amount ELSE 0 END),
			SUM(CASE WHEN type='expense' THEN amount ELSE 0 END) FROM transactions WHERE account_id = ?`, a.ID).Scan(&income, &expense)
		balances[a.ID] = income.Int64 - expense.Int64
	}

	data := AccountsData{Accounts: accounts, Balances: balances, Nav: "accounts"}
	if err := tpl.ExecuteTemplate(w, "accounts.html", data); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func handleAccountAdd(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	name := r.FormValue("name")
	icon := r.FormValue("icon")
	if icon == "" {
		icon = "💳"
	}
	if name == "" {
		http.Error(w, "name required", 400)
		return
	}
	balance, _ := strconv.ParseInt(r.FormValue("balance"), 10, 64)
	db.Exec(`INSERT INTO accounts(name, icon, sort_order, balance) VALUES (?, ?, (SELECT COALESCE(MAX(sort_order),0)+1 FROM accounts), ?)`, name, icon, balance)
	http.Redirect(w, r, "/accounts", http.StatusSeeOther)
}

func handleAccountUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	name := r.FormValue("name")
	icon := r.FormValue("icon")
	if icon == "" {
		icon = "💳"
	}
	if name == "" {
		http.Error(w, "name required", 400)
		return
	}
	balance, _ := strconv.ParseInt(r.FormValue("balance"), 10, 64)
	db.Exec(`UPDATE accounts SET name = ?, icon = ?, balance = ? WHERE id = ?`, name, icon, balance, id)
	http.Redirect(w, r, "/accounts", http.StatusSeeOther)
}

func handleTransfer(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	fromID, err1 := strconv.ParseInt(r.FormValue("from_id"), 10, 64)
	toID, err2 := strconv.ParseInt(r.FormValue("to_id"), 10, 64)
	amount, err3 := strconv.ParseInt(r.FormValue("amount"), 10, 64)
	if err1 != nil || err2 != nil || err3 != nil || amount <= 0 || fromID == toID {
		http.Error(w, "invalid input", 400)
		return
	}
	if err := transferBetweenAccounts(fromID, toID, amount); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/accounts", http.StatusSeeOther)
}

func handleAccountDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM accounts`).Scan(&count)
	if count <= 1 {
		http.Redirect(w, r, "/accounts", http.StatusSeeOther)
		return
	}
	var first int64
	db.QueryRow(`SELECT id FROM accounts WHERE id != ? ORDER BY sort_order, id LIMIT 1`, id).Scan(&first)
	db.Exec(`UPDATE transactions SET account_id = ? WHERE account_id = ?`, first, id)
	db.Exec(`UPDATE templates SET account_id = ? WHERE account_id = ?`, first, id)
	db.Exec(`DELETE FROM accounts WHERE id = ?`, id)
	http.Redirect(w, r, "/accounts", http.StatusSeeOther)
}
