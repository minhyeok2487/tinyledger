package app

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func handleWishlist(w http.ResponseWriter, r *http.Request) {
	items, err := listWishItems()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	spendable, err := loadSpendable()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	accounts, err := listAccounts()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	hobbyItems, err := listHobbyItems(false)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	var open, bought []WishItem
	for _, it := range items {
		if it.BoughtAt == "" {
			open = append(open, it)
		} else {
			bought = append(bought, it)
		}
	}

	data := WishlistData{
		Items:       markAffordable(open, spendable.Spendable),
		Bought:      bought,
		Spendable:   spendable,
		Accounts:    accounts,
		HobbyItems:  hobbyItems,
		ExpenseCats: expenseCategories,
		Today:       time.Now().Format("2006-01-02"),
		Nav:         "wishlist",
	}
	if err := tpl.ExecuteTemplate(w, "wishlist.html", data); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

// markAffordable flags what today's 쓸 수 있는 돈 covers. It deliberately does
// not reorder the list — the order is the user's, and shuffling it every time
// the balance moves would make items hard to find. Pure, so it is testable.
func markAffordable(items []WishItem, spendable int64) []WishItem {
	out := make([]WishItem, len(items))
	for i, it := range items {
		it.Affordable = it.Price > 0 && it.Price <= spendable
		if !it.Affordable {
			it.Short = it.Price - spendable
		}
		out[i] = it
	}
	return out
}

func listWishItems() ([]WishItem, error) {
	rows, err := db.Query(`SELECT id, name, price, COALESCE(url,''), COALESCE(memo,''),
			COALESCE(bought_at,'')
		FROM wishlist ORDER BY sort_order, id`)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer rows.Close()
	var out []WishItem
	for rows.Next() {
		var it WishItem
		if rows.Scan(&it.ID, &it.Name, &it.Price, &it.URL, &it.Memo, &it.BoughtAt) == nil {
			out = append(out, it)
		}
	}
	return out, rows.Err()
}

// safeWishURL keeps only http(s) links. html/template already refuses to emit
// a javascript: href, but storing one at all is pointless.
func safeWishURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return raw
}

func handleWishAdd(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "name required", 400)
		return
	}
	price, _ := strconv.ParseInt(r.FormValue("price"), 10, 64)
	db.Exec(`INSERT INTO wishlist(name, price, url, memo, created_at, sort_order)
		VALUES (?, ?, ?, ?, ?, (SELECT COALESCE(MAX(sort_order),-1)+1 FROM wishlist))`,
		name, price, safeWishURL(r.FormValue("url")), r.FormValue("memo"),
		time.Now().Format("2006-01-02"))
	http.Redirect(w, r, "/wishlist", http.StatusSeeOther)
}

func handleWishUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "name required", 400)
		return
	}
	price, _ := strconv.ParseInt(r.FormValue("price"), 10, 64)
	db.Exec(`UPDATE wishlist SET name = ?, price = ?, url = ?, memo = ? WHERE id = ?`,
		name, price, safeWishURL(r.FormValue("url")), r.FormValue("memo"), r.PathValue("id"))
	http.Redirect(w, r, "/wishlist", http.StatusSeeOther)
}

func handleWishDelete(w http.ResponseWriter, r *http.Request) {
	db.Exec(`DELETE FROM wishlist WHERE id = ?`, r.PathValue("id"))
	http.Redirect(w, r, "/wishlist", http.StatusSeeOther)
}

// handleWishBuy records the purchase as a real transaction and marks the row
// bought. Both writes go in one transaction: a ledger entry with no wishlist
// update (or the reverse) would leave the two views disagreeing.
func handleWishBuy(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	id := r.PathValue("id")

	var name string
	var price int64
	var boughtAt sql.NullString
	if err := db.QueryRow(`SELECT name, price, bought_at FROM wishlist WHERE id = ?`, id).
		Scan(&name, &price, &boughtAt); err != nil {
		http.Error(w, "not found", 404)
		return
	}
	if boughtAt.Valid && boughtAt.String != "" {
		// Already bought — a double submit shouldn't log a second expense.
		http.Redirect(w, r, "/wishlist", http.StatusSeeOther)
		return
	}

	amount, err := strconv.ParseInt(r.FormValue("amount"), 10, 64)
	if err != nil || amount <= 0 {
		amount = price
	}
	if amount <= 0 {
		http.Error(w, "amount required", 400)
		return
	}
	category := r.FormValue("category")
	if !isExpenseCategory(category) {
		category = hobbyCategory
	}
	accountID, _ := strconv.ParseInt(r.FormValue("account_id"), 10, 64)
	hobbyItemID, _ := strconv.ParseInt(r.FormValue("hobby_item_id"), 10, 64)
	if category != hobbyCategory {
		hobbyItemID = 0
	}
	// A far-future date would park the expense in a month no view reaches,
	// so anything unparseable or ahead of today falls back to today.
	today := time.Now().Format("2006-01-02")
	date := r.FormValue("date")
	if _, dErr := time.Parse("2006-01-02", date); dErr != nil || date > today {
		date = today
	}

	if err := buyWishItem(id, name, date, category, amount, accountID, hobbyItemID); err != nil &&
		!errors.Is(err, errAlreadyBought) {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/wishlist", http.StatusSeeOther)
}

// errAlreadyBought means another request got there first.
var errAlreadyBought = errors.New("already bought")

func buyWishItem(id, name, date, category string, amount, accountID, hobbyItemID int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Claim the row first, and only if it is still unbought. Checking before
	// the transaction and updating unconditionally let two concurrent submits
	// both pass the check and both write an expense — a double-tap on 구매함
	// silently doubled the spending. The UPDATE is the lock: whoever changes
	// zero rows loses and writes nothing.
	res, err := tx.Exec(`UPDATE wishlist SET bought_at = ?
		WHERE id = ? AND (bought_at IS NULL OR bought_at = '')`, date, id)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return errAlreadyBought
	}

	if _, err := tx.Exec(`INSERT INTO transactions(account_id, date, type, category, amount, memo, hobby_item_id)
		VALUES (`+accountIDOrDefault+`, ?, 'expense', ?, ?, ?, `+hobbyItemOrNull+`)`,
		accountID, date, category, amount, name, hobbyItemID); err != nil {
		return err
	}
	return tx.Commit()
}
