package app

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"sync"
)

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	month := normalizeMonth(r.URL.Query().Get("month"))
	accountID := int64(0)
	if v := r.URL.Query().Get("account"); v != "" {
		accountID, _ = strconv.ParseInt(v, 10, 64)
	}

	data, err := buildDashboard(month, accountID)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	data.Nav = "dashboard"

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
	month := normalizeMonth(r.FormValue("month"))
	http.Redirect(w, r, "/?month="+month, http.StatusSeeOther)
}

func buildDashboard(month string, accountID int64) (DashboardData, error) {
	prev := shiftMonth(month, -1)
	next := shiftMonth(month, 1)

	// These six reads are independent of each other, and against a remote
	// Turso database each one is a full network round trip. Run them
	// concurrently: every goroutine writes to its own variable, and nothing
	// is read until wg.Wait() returns, so no locking is needed.
	var (
		wg       sync.WaitGroup
		errs     [5]error
		budgets  map[string]int64
		txs      []Transaction
		accounts []Account
		allTpl   []Template
		spent    map[string]bool
		note     string
	)

	wg.Add(6)
	go func() {
		defer wg.Done()
		budgets, errs[0] = monthBudgets(month)
	}()
	go func() {
		defer wg.Done()
		txs, errs[1] = monthTransactions(month, accountID)
	}()
	go func() {
		defer wg.Done()
		accounts, errs[2] = listAccounts()
	}()
	go func() {
		defer wg.Done()
		allTpl, errs[3] = listTemplates()
	}()
	go func() {
		defer wg.Done()
		spent, errs[4] = spentTemplateKeys(month)
	}()
	go func() {
		defer wg.Done()
		note = getNote() // absent note is normal, not an error
	}()
	wg.Wait()

	// A failed query would otherwise render as a plausible-looking zero —
	// a 총 잔액 of 0 and a large negative 쓸 수 있는 돈. Fail loudly instead.
	for _, err := range errs {
		if err != nil {
			return DashboardData{}, err
		}
	}

	var income, expense int64
	catTotals := map[string]int64{}
	for _, tx := range txs {
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

	var totalBalance, availableBalance, selectedBalance int64
	included := 0
	for _, a := range accounts {
		totalBalance += a.Balance
		if !a.Excluded {
			included++
			availableBalance += a.Balance
		}
		if a.ID == accountID {
			selectedBalance = a.Balance
		}
	}
	realBalance := totalBalance
	if accountID > 0 {
		realBalance = selectedBalance
	}

	// Favorites still unspent this month are the fixed costs yet to go out,
	// so what's actually free to spend is the non-excluded balance minus them.
	templates := filterUnspent(allTpl, spent)
	excludedAcct := map[int64]bool{}
	for _, a := range accounts {
		if a.Excluded {
			excludedAcct[a.ID] = true
		}
	}
	var upcomingFixed int64
	for _, t := range templates {
		// A fixed cost paid from an excluded account was never part of
		// availableBalance, so subtracting it would double-count.
		if t.Type == "expense" && !excludedAcct[t.AccountID] {
			upcomingFixed += t.Amount
		}
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
		Templates:    templates,
		ExpenseCats:  expenseCategories,
		IncomeCats:   incomeCategories,
		Note:         note,

		AvailableBalance: availableBalance,
		UpcomingFixed:    upcomingFixed,
		Spendable:        availableBalance - upcomingFixed,
		AllExcluded:      len(accounts) > 0 && included == 0,
	}, nil
}

func monthBudgets(month string) (map[string]int64, error) {
	budgets := map[string]int64{}
	rows, err := db.Query(`SELECT category, amount FROM budgets WHERE month = ?`, month)
	if err != nil {
		log.Println(err)
		return budgets, err
	}
	defer rows.Close()
	for rows.Next() {
		var c string
		var a int64
		if rows.Scan(&c, &a) == nil {
			budgets[c] = a
		}
	}
	return budgets, rows.Err()
}

func monthTransactions(month string, accountID int64) ([]Transaction, error) {
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
		return nil, err
	}
	defer rows.Close()

	var out []Transaction
	for rows.Next() {
		var tx Transaction
		var memo sql.NullString
		if err := rows.Scan(&tx.ID, &tx.AccountID, &tx.AccountName, &tx.Date, &tx.Type, &tx.Category, &tx.Amount, &memo); err != nil {
			continue
		}
		tx.Memo = memo.String
		out = append(out, tx)
	}
	return out, rows.Err()
}
