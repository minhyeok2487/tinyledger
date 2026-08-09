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
	Categories   []CategorySum
	Transactions []Transaction
	Templates    []Template
	ExpenseCats  []string
	IncomeCats   []string
	Nav          string
	Note         string
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
	Transactions []Transaction
	Total        int64
	Count        int
	Nav          string
}

type BudgetRow struct {
	Category string
	Icon     string
	Amount   int64
}

type BudgetData struct {
	Month       string
	PrevMonth   string
	NextMonth   string
	Rows        []BudgetRow
	ExpenseCats []string
	Nav         string
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
