package app

type Account struct {
	ID      int64
	Name    string
	Icon    string
	Balance int64
	// Excluded keeps an account (e.g. 비상금/세이프박스) out of the
	// "쓸 수 있는 돈" calculation. It still shows everywhere else.
	Excluded bool
}

type Transaction struct {
	ID          int64
	AccountID   int64
	AccountName string
	Date        string
	Type        string // income | expense
	Category    string
	Amount      int64
	Memo        string
	// HobbyItemID is 0 when unset; only meaningful for 여가 expenses, kept
	// here so the edit modal can pre-select the current sub-item.
	HobbyItemID int64
}

type Template struct {
	ID          int64
	AccountID   int64
	AccountName string
	AccountIcon string
	Type        string
	Category    string
	Amount      int64
	Memo        string
}

type CategorySum struct {
	Category string
	Icon     string
	Spent    int64
	Budget   int64
	Percent  float64 // of total expense
	BPercent float64 // of budget, capped 100
	Over     bool
}

type DashboardData struct {
	Month        string
	PrevMonth    string
	NextMonth    string
	IsCurrent    bool
	Accounts     []Account
	AccountID    int64
	TotalBalance int64
	Income       int64
	Expense      int64
	Balance      int64
	// AvailableBalance is TotalBalance minus excluded accounts; Spendable is
	// that minus UpcomingFixed (this month's not-yet-logged fixed costs).
	AvailableBalance int64
	UpcomingFixed    int64
	Spendable        int64
	AllExcluded      bool
	Categories       []CategorySum
	Transactions     []Transaction
	Templates        []Template
	ExpenseCats      []string
	IncomeCats       []string
	HobbyItems       []HobbyItem
	Nav              string
	Note             string
}

type CalendarDay struct {
	Day     int
	Date    string
	InMonth bool
	Income  int64
	Expense int64
	Today   bool
}

type CalendarData struct {
	Month     string
	PrevMonth string
	NextMonth string
	Weeks     [][]CalendarDay
	Accounts  []Account
	AccountID int64
	Nav       string
}

type SearchData struct {
	Keyword      string
	Category     string
	AccountID    int64
	Type         string
	DateFrom     string
	DateTo       string
	Accounts     []Account
	ExpenseCats  []string
	IncomeCats   []string
	HobbyItems   []HobbyItem
	Transactions []Transaction
	Total        int64
	Count        int
	Nav          string
}

type TemplatesData struct {
	Items       []Template
	Accounts    []Account
	ExpenseCats []string
	IncomeCats  []string
	Nav         string
}

type AccountsData struct {
	Accounts []Account
	Balances map[int64]int64
	Nav      string
}

type LoginData struct {
	Next  string
	Error string
}

// HobbyItem is a sub-bucket of the 여가 category (명조, 프라모델, …), chosen
// when a transaction is entered. Archived items keep showing in totals but
// disappear from the entry select.
type HobbyItem struct {
	ID       int64
	Name     string
	Archived bool
}

type HobbyGroup struct {
	ItemID   int64 // 0 = 미분류
	Name     string
	Archived bool
	Total    int64
	Percent  float64
	Txs      []Transaction
}

type HobbyData struct {
	Scope     string // "month" | "all"
	Month     string
	PrevMonth string
	NextMonth string
	IsCurrent bool
	Groups    []HobbyGroup
	Items     []HobbyItem
	Total     int64
	Nav       string
}

type WishItem struct {
	ID       int64
	Name     string
	Price    int64
	URL      string
	Memo     string
	BoughtAt string // "" = 아직 안 삼
	// Affordable and Short are derived from the current 쓸 수 있는 돈.
	Affordable bool
	Short      int64
}

type WishlistData struct {
	Items       []WishItem
	Bought      []WishItem
	Spendable   SpendableInfo
	Accounts    []Account
	HobbyItems  []HobbyItem
	ExpenseCats []string
	Today       string
	Nav         string
}
