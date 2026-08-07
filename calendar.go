package main

import (
	"net/http"
	"strconv"
	"time"
)

func handleCalendar(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month")
	if month == "" {
		month = currentMonth()
	}
	accountID := int64(0)
	if v := r.URL.Query().Get("account"); v != "" {
		accountID, _ = strconv.ParseInt(v, 10, 64)
	}

	daily := map[string][2]int64{} // date -> [income, expense]
	query := `SELECT date, type, SUM(amount) FROM transactions WHERE substr(date,1,7) = ?`
	args := []any{month}
	if accountID > 0 {
		query += ` AND account_id = ?`
		args = append(args, accountID)
	}
	query += ` GROUP BY date, type`
	rows, err := db.Query(query, args...)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var date, typ string
			var amt int64
			if rows.Scan(&date, &typ, &amt) == nil {
				v := daily[date]
				if typ == "income" {
					v[0] = amt
				} else {
					v[1] = amt
				}
				daily[date] = v
			}
		}
	}

	t, err := time.Parse("2006-01", month)
	if err != nil {
		t = time.Now()
	}
	first := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.Local)
	startOffset := int(first.Weekday())
	gridStart := first.AddDate(0, 0, -startOffset)
	today := time.Now().Format("2006-01-02")

	var weeks [][]CalendarDay
	cur := gridStart
	for w := 0; w < 6; w++ {
		var week []CalendarDay
		for d := 0; d < 7; d++ {
			dateStr := cur.Format("2006-01-02")
			v := daily[dateStr]
			week = append(week, CalendarDay{
				Day:     cur.Day(),
				Date:    dateStr,
				InMonth: cur.Month() == first.Month(),
				Income:  v[0],
				Expense: v[1],
				Today:   dateStr == today,
			})
			cur = cur.AddDate(0, 0, 1)
		}
		weeks = append(weeks, week)
		if cur.Month() != first.Month() && w >= 3 {
			break
		}
	}

	data := CalendarData{
		Month:     month,
		PrevMonth: shiftMonth(month, -1),
		NextMonth: shiftMonth(month, 1),
		Weeks:     weeks,
		Accounts:  listAccounts(),
		AccountID: accountID,
		Nav:       "calendar",
	}

	if err := tpl.ExecuteTemplate(w, "calendar.html", data); err != nil {
		http.Error(w, err.Error(), 500)
	}
}
