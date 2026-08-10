package app

import (
	"database/sql"
	"net/http"
	"strconv"
)

func handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	keyword := q.Get("keyword")
	category := q.Get("category")
	typ := q.Get("type")
	dateFrom := q.Get("from")
	dateTo := q.Get("to")
	accountID, _ := strconv.ParseInt(q.Get("account"), 10, 64)

	where := `WHERE 1=1`
	args := []any{}
	if keyword != "" {
		where += ` AND (t.memo LIKE ? OR t.category LIKE ?)`
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
	}
	if category != "" {
		where += ` AND t.category = ?`
		args = append(args, category)
	}
	if typ != "" {
		where += ` AND t.type = ?`
		args = append(args, typ)
	}
	if dateFrom != "" {
		where += ` AND t.date >= ?`
		args = append(args, dateFrom)
	}
	if dateTo != "" {
		where += ` AND t.date <= ?`
		args = append(args, dateTo)
	}
	if accountID > 0 {
		where += ` AND t.account_id = ?`
		args = append(args, accountID)
	}

	query := `SELECT t.id, t.account_id, a.name, t.date, t.type, t.category, t.amount, t.memo,
			COALESCE(t.hobby_item_id,0)
		FROM transactions t JOIN accounts a ON a.id = t.account_id ` + where + ` ORDER BY t.date DESC, t.id DESC LIMIT 300`

	rows, err := db.Query(query, args...)
	var txs []Transaction
	var total int64
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var tx Transaction
			var memo sql.NullString
			if rows.Scan(&tx.ID, &tx.AccountID, &tx.AccountName, &tx.Date, &tx.Type, &tx.Category, &tx.Amount, &memo, &tx.HobbyItemID) == nil {
				tx.Memo = memo.String
				txs = append(txs, tx)
				if tx.Type == "expense" {
					total -= tx.Amount
				} else {
					total += tx.Amount
				}
			}
		}
	}

	accounts, _ := listAccounts()
	hobbyItems, _ := listHobbyItems(false)
	data := SearchData{
		Keyword:      keyword,
		Category:     category,
		Type:         typ,
		DateFrom:     dateFrom,
		DateTo:       dateTo,
		AccountID:    accountID,
		Accounts:     accounts,
		ExpenseCats:  expenseCategories,
		IncomeCats:   incomeCategories,
		HobbyItems:   hobbyItems,
		Transactions: txs,
		Total:        total,
		Count:        len(txs),
		Nav:          "search",
	}

	if err := tpl.ExecuteTemplate(w, "search.html", data); err != nil {
		http.Error(w, err.Error(), 500)
	}
}
