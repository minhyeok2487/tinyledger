package app

import (
	"log"
	"net/http"
	"strconv"
)

func handleTemplates(w http.ResponseWriter, r *http.Request) {
	data := TemplatesData{
		Items:       listTemplates(),
		Accounts:    listAccounts(),
		ExpenseCats: expenseCategories,
		IncomeCats:  incomeCategories,
		Nav:         "templates",
	}
	if err := tpl.ExecuteTemplate(w, "templates.html", data); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func handleTemplateAdd(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	typ := r.FormValue("type")
	category := r.FormValue("category")
	memo := r.FormValue("memo")
	amount, err := strconv.ParseInt(r.FormValue("amount"), 10, 64)
	accountID, _ := strconv.ParseInt(r.FormValue("account_id"), 10, 64)
	if accountID == 0 {
		accountID = 1
	}
	if err != nil || amount <= 0 || (typ != "income" && typ != "expense") {
		http.Error(w, "invalid input", 400)
		return
	}

	var maxOrder int
	db.QueryRow(`SELECT COALESCE(MAX(sort_order),-1) FROM templates`).Scan(&maxOrder)
	db.Exec(`INSERT INTO templates(account_id, type, category, amount, memo, sort_order) VALUES (?, ?, ?, ?, ?, ?)`,
		accountID, typ, category, amount, memo, maxOrder+1)

	http.Redirect(w, r, "/templates", http.StatusSeeOther)
}

func handleTemplateDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	db.Exec(`DELETE FROM templates WHERE id = ?`, id)
	http.Redirect(w, r, "/templates", http.StatusSeeOther)
}

// unspentTemplates returns favorites that don't already have a matching
// transaction (same type/category/amount/memo) logged in the given month.
func unspentTemplates(month string) []Template {
	all := listTemplates()
	if len(all) == 0 {
		return nil
	}

	monthStart, monthEnd := monthRange(month)
	spent := map[string]bool{}
	rows, err := db.Query(`SELECT type, category, amount, COALESCE(memo,'') FROM transactions
		WHERE date >= ? AND date < ?`, monthStart, monthEnd)
	if err != nil {
		log.Println(err)
		return all
	}
	defer rows.Close()
	for rows.Next() {
		var typ, category, memo string
		var amount int64
		if rows.Scan(&typ, &category, &amount, &memo) == nil {
			spent[templateKey(typ, category, amount, memo)] = true
		}
	}

	var out []Template
	for _, t := range all {
		if !spent[templateKey(t.Type, t.Category, t.Amount, t.Memo)] {
			out = append(out, t)
		}
	}
	return out
}

func templateKey(typ, category string, amount int64, memo string) string {
	return typ + "\x00" + category + "\x00" + strconv.FormatInt(amount, 10) + "\x00" + memo
}
