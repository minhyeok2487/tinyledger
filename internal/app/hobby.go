package app

import (
	"database/sql"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
)

const hobbyCategory = "여가"

// hobbyTx carries the item link alongside the transaction so grouping can
// happen in Go after a single query.
type hobbyTx struct {
	Transaction
	ItemID   int64
	ItemName string
	Archived bool
}

func handleHobby(w http.ResponseWriter, r *http.Request) {
	scope := r.URL.Query().Get("scope")
	if scope != "all" {
		scope = "month"
	}
	month := normalizeMonth(r.URL.Query().Get("month"))

	txs, err := hobbyTransactions(scope, month)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	items, err := listHobbyItems(false)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	groups, total := groupHobby(txs)
	data := HobbyData{
		Scope:     scope,
		Month:     month,
		PrevMonth: shiftMonth(month, -1),
		NextMonth: shiftMonth(month, 1),
		IsCurrent: month == currentMonth(),
		Groups:    groups,
		Items:     items,
		Total:     total,
		Nav:       "hobby",
	}
	if err := tpl.ExecuteTemplate(w, "hobby.html", data); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func hobbyTransactions(scope, month string) ([]hobbyTx, error) {
	query := `SELECT t.id, t.date, t.amount, COALESCE(t.memo,''),
			COALESCE(t.hobby_item_id,0), COALESCE(h.name,''), COALESCE(h.archived,0)
		FROM transactions t LEFT JOIN hobby_items h ON h.id = t.hobby_item_id
		WHERE t.type = 'expense' AND t.category = ?`
	args := []any{hobbyCategory}
	if scope != "all" {
		start, end := monthRange(month)
		query += ` AND t.date >= ? AND t.date < ?`
		args = append(args, start, end)
	}
	query += ` ORDER BY t.date DESC, t.id DESC`

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer rows.Close()

	var out []hobbyTx
	for rows.Next() {
		var t hobbyTx
		var memo sql.NullString
		if err := rows.Scan(&t.ID, &t.Date, &t.Amount, &memo, &t.ItemID, &t.ItemName, &t.Archived); err != nil {
			continue
		}
		t.Memo = memo.String
		t.Type = "expense"
		t.Category = hobbyCategory
		out = append(out, t)
	}
	return out, rows.Err()
}

// groupHobby buckets transactions by sub-item, biggest spend first, with the
// unclassified ones last so they read as a to-do rather than a category.
// Pure, so it can be tested without a database.
func groupHobby(txs []hobbyTx) ([]HobbyGroup, int64) {
	byItem := map[int64]*HobbyGroup{}
	var total int64
	for _, t := range txs {
		g := byItem[t.ItemID]
		if g == nil {
			name := t.ItemName
			if t.ItemID == 0 {
				name = "미분류"
			}
			g = &HobbyGroup{ItemID: t.ItemID, Name: name, Archived: t.Archived}
			byItem[t.ItemID] = g
		}
		g.Total += t.Amount
		g.Txs = append(g.Txs, t.Transaction)
		total += t.Amount
	}

	groups := make([]HobbyGroup, 0, len(byItem))
	for _, g := range byItem {
		if total > 0 {
			g.Percent = float64(g.Total) / float64(total) * 100
		}
		groups = append(groups, *g)
	}
	sort.Slice(groups, func(i, j int) bool {
		a, b := groups[i], groups[j]
		if (a.ItemID == 0) != (b.ItemID == 0) {
			return b.ItemID == 0 // 미분류 always sinks to the bottom
		}
		if a.Total != b.Total {
			return a.Total > b.Total
		}
		return a.Name < b.Name
	})
	return groups, total
}

func listHobbyItems(includeArchived bool) ([]HobbyItem, error) {
	query := `SELECT id, name, archived FROM hobby_items`
	if !includeArchived {
		query += ` WHERE archived = 0`
	}
	query += ` ORDER BY sort_order, id`

	rows, err := db.Query(query)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer rows.Close()
	var out []HobbyItem
	for rows.Next() {
		var it HobbyItem
		if rows.Scan(&it.ID, &it.Name, &it.Archived) == nil {
			out = append(out, it)
		}
	}
	return out, rows.Err()
}

// hobbyRedirect preserves the scope and month being viewed, so the partial
// update swaps in markup for the same view the user is looking at.
func hobbyRedirect(r *http.Request) string {
	v := url.Values{}
	scope := r.FormValue("scope")
	if scope != "all" {
		scope = "month"
		v.Set("month", normalizeMonth(r.FormValue("month")))
	}
	v.Set("scope", scope)
	return "/hobby?" + v.Encode()
}

func handleHobbyItemAdd(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if name := r.FormValue("name"); name != "" {
		db.Exec(`INSERT OR IGNORE INTO hobby_items(name, sort_order)
			VALUES (?, (SELECT COALESCE(MAX(sort_order),-1)+1 FROM hobby_items))`, name)
	}
	http.Redirect(w, r, hobbyRedirect(r), http.StatusSeeOther)
}

func handleHobbyItemRename(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if name := r.FormValue("name"); name != "" {
		db.Exec(`UPDATE hobby_items SET name = ? WHERE id = ?`, name, r.PathValue("id"))
	}
	http.Redirect(w, r, hobbyRedirect(r), http.StatusSeeOther)
}

// handleHobbyItemDelete archives instead of deleting when transactions point
// at the item — dropping it would make that spending vanish from the totals.
func handleHobbyItemDelete(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	id := r.PathValue("id")
	var used int
	db.QueryRow(`SELECT COUNT(*) FROM transactions WHERE hobby_item_id = ?`, id).Scan(&used)
	if used > 0 {
		db.Exec(`UPDATE hobby_items SET archived = 1 WHERE id = ?`, id)
	} else {
		db.Exec(`DELETE FROM hobby_items WHERE id = ?`, id)
	}
	http.Redirect(w, r, hobbyRedirect(r), http.StatusSeeOther)
}

// handleHobbyAssign is the only way to change an existing transaction. It
// touches one column on 여가 rows only — deliberately not a general edit route.
func handleHobbyAssign(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	itemID, _ := strconv.ParseInt(r.FormValue("item_id"), 10, 64)
	db.Exec(`UPDATE transactions SET hobby_item_id = NULLIF(?,0)
		WHERE id = ? AND category = ?`, itemID, r.PathValue("id"), hobbyCategory)
	http.Redirect(w, r, hobbyRedirect(r), http.StatusSeeOther)
}
