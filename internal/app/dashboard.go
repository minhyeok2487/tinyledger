package app

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
)

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month")
	if month == "" {
		month = currentMonth()
	}
	accountID := int64(0)
	if v := r.URL.Query().Get("account"); v != "" {
		accountID, _ = strconv.ParseInt(v, 10, 64)
	}

	data := buildDashboard(month, accountID)
	data.Nav = "dashboard"
	data.Note = getNote()

	if err := tpl.ExecuteTemplate(w, "dashboard.html", data); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func handleNoteSave(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	saveNote(r.FormValue("content"))
	month := r.FormValue("month")
	if month == "" {
		month = currentMonth()
	}
	http.Redirect(w, r, "/?month="+month, http.StatusSeeOther)
}

func buildDashboard(month string, accountID int64) DashboardData {
	prev := shiftMonth(month, -1)
	next := shiftMonth(month, 1)

	budgets := map[string]int64{}
	brows, err := db.Query(`SELECT category, amount FROM budgets WHERE month = ?`, month)
	if err == nil {
		for brows.Next() {
			var c string
			var a int64
			if brows.Scan(&c, &a) == nil {
				budgets[c] = a
			}
		}
		brows.Close()
	}

	monthStart, monthEnd := monthRange(month)
	query := `SELECT t.id, t.account_id, a.name, t.date, t.type, t.category, t.amount, t.memo
		FROM transactions t JOIN accounts a ON a.id = t.account_id
		WHERE t.date >= ? AND t.date < ?`
	args := []any{monthStart, monthEnd}
	if accountID > 0 {
		query += ` AND t.account_id = ?`
		args = append(args, accountID)
	}
	query += ` ORDER BY t.date DESC, t.id DESC`

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Println(err)
	}
	defer rows.Close()

	var txs []Transaction
	var income, expense int64
	catTotals := map[string]int64{}

	for rows.Next() {
		var tx Transaction
		var memo sql.NullString
		if err := rows.Scan(&tx.ID, &tx.AccountID, &tx.AccountName, &tx.Date, &tx.Type, &tx.Category, &tx.Amount, &memo); err != nil {
			continue
		}
		tx.Memo = memo.String
		txs = append(txs, tx)
		if tx.Type == "income" {
			income += tx.Amount
		} else {
			expense += tx.Amount
			catTotals[tx.Category] += tx.Amount
		}
	}

	catSet := map[string]bool{}
	var cats []CategorySum
	for c, amt := range catTotals {
		catSet[c] = true
		pct := 0.0
		if expense > 0 {
			pct = float64(amt) / float64(expense) * 100
		}
		budget := budgets[c]
		bpct := 0.0
		over := false
		if budget > 0 {
			bpct = float64(amt) / float64(budget) * 100
			if bpct > 100 {
				over = true
				bpct = 100
			}
		}
		cats = append(cats, CategorySum{Category: c, Icon: iconFor(c), Spent: amt, Budget: budget, Percent: pct, BPercent: bpct, Over: over})
	}
	// include budgeted categories with zero spend
	for c, b := range budgets {
		if !catSet[c] && b > 0 {
			cats = append(cats, CategorySum{Category: c, Icon: iconFor(c), Spent: 0, Budget: b, Percent: 0, BPercent: 0, Over: false})
		}
	}
	for i := 0; i < len(cats); i++ {
		for j := i + 1; j < len(cats); j++ {
			if cats[j].Spent > cats[i].Spent {
				cats[i], cats[j] = cats[j], cats[i]
			}
		}
	}

	accounts := listAccounts()
	var totalBalance int64
	var selectedBalance int64
	for _, a := range accounts {
		totalBalance += a.Balance
		if a.ID == accountID {
			selectedBalance = a.Balance
		}
	}
	realBalance := totalBalance
	if accountID > 0 {
		realBalance = selectedBalance
	}

	return DashboardData{
		Month:        month,
		PrevMonth:    prev,
		NextMonth:    next,
		IsCurrent:    month == currentMonth(),
		Accounts:     accounts,
		AccountID:    accountID,
		TotalBalance: totalBalance,
		Income:       income,
		Expense:      expense,
		Balance:      realBalance,
		Categories:   cats,
		Transactions: txs,
		Templates:    unspentTemplates(month),
		ExpenseCats:  expenseCategories,
		IncomeCats:   incomeCategories,
	}
}
