package app

import (
	"errors"
	"net/http"
	"strconv"
)

func handleAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, _ := listAccounts()
	balances := accountNetTotals()
	// Accounts with no transactions get no row above; the template indexes
	// this map directly, so give every account an entry.
	for _, a := range accounts {
		if _, ok := balances[a.ID]; !ok {
			balances[a.ID] = 0
		}
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
	// An unchecked checkbox posts nothing, so absence means "not excluded".
	excluded := 0
	if r.FormValue("excluded") != "" {
		excluded = 1
	}
	db.Exec(`UPDATE accounts SET name = ?, icon = ?, balance = ?, excluded = ? WHERE id = ?`, name, icon, balance, excluded, id)
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
	// Refusing to delete the last account isn't an error worth showing —
	// the page just comes back unchanged.
	if err := deleteAccountReassign(r.PathValue("id")); err != nil && !errors.Is(err, errLastAccount) {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/accounts", http.StatusSeeOther)
}
