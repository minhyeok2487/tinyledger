package app

import (
	"log"
	"net/http"
	"strconv"
)

// insertTemplateSQL appends a favorite at the end of the list, deriving
// sort_order in the same round trip as the insert.
const insertTemplateSQL = `INSERT INTO templates(account_id, type, category, amount, memo, sort_order)
	VALUES (?, ?, ?, ?, ?, (SELECT COALESCE(MAX(sort_order),-1)+1 FROM templates))`

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

	db.Exec(insertTemplateSQL, accountID, typ, category, amount, memo)

	http.Redirect(w, r, "/templates", http.StatusSeeOther)
}

func handleTemplateDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	db.Exec(`DELETE FROM templates WHERE id = ?`, id)
	http.Redirect(w, r, "/templates", http.StatusSeeOther)
}

// spentTemplateKeys indexes the month's transactions by template identity, so
// favorites already logged this month can be filtered out. Split from
// filterUnspent so buildDashboard can run this query alongside the others.
func spentTemplateKeys(month string) map[string]bool {
	monthStart, monthEnd := monthRange(month)
	rows, err := db.Query(`SELECT type, category, amount, COALESCE(memo,'') FROM transactions
		WHERE date >= ? AND date < ?`, monthStart, monthEnd)
	if err != nil {
		log.Println(err)
		return nil
	}
	defer rows.Close()

	spent := map[string]bool{}
	for rows.Next() {
		var typ, category, memo string
		var amount int64
		if rows.Scan(&typ, &category, &amount, &memo) == nil {
			spent[templateKey(typ, category, amount, memo)] = true
		}
	}
	return spent
}

// filterUnspent returns the favorites with no matching transaction
// (same type/category/amount/memo) in the month spent was built from.
func filterUnspent(all []Template, spent map[string]bool) []Template {
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
