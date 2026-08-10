package app

// SpendableInfo answers "how much can I actually spend right now": the
// balance of accounts not marked 제외, minus this month's fixed costs that
// haven't gone out yet. The dashboard and the wishlist both show it, so the
// rule lives here rather than in either page.
type SpendableInfo struct {
	Available     int64
	UpcomingFixed int64
	Spendable     int64
	AllExcluded   bool
}

// computeSpendable is pure so it can be tested without a database. unspent is
// the favorites list already filtered down to what hasn't been logged yet.
func computeSpendable(accounts []Account, unspent []Template) SpendableInfo {
	var info SpendableInfo
	excluded := map[int64]bool{}
	included := 0
	for _, a := range accounts {
		if a.Excluded {
			excluded[a.ID] = true
			continue
		}
		included++
		info.Available += a.Balance
	}
	for _, t := range unspent {
		// A fixed cost paid from an excluded account was never part of
		// Available, so subtracting it would double-count.
		if t.Type == "expense" && !excluded[t.AccountID] {
			info.UpcomingFixed += t.Amount
		}
	}
	info.Spendable = info.Available - info.UpcomingFixed
	info.AllExcluded = len(accounts) > 0 && included == 0
	return info
}

// loadSpendable fetches what computeSpendable needs. buildDashboard does not
// use it — that page already has the accounts and templates in hand and
// shouldn't pay for the queries twice.
func loadSpendable() (SpendableInfo, error) {
	accounts, err := listAccounts()
	if err != nil {
		return SpendableInfo{}, err
	}
	allTpl, err := listTemplates()
	if err != nil {
		return SpendableInfo{}, err
	}
	spent, err := spentTemplateKeys(currentMonth())
	if err != nil {
		return SpendableInfo{}, err
	}
	return computeSpendable(accounts, filterUnspent(allTpl, spent)), nil
}
