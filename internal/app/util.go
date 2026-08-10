package app

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

var expenseCategories = []string{"식비", "교통", "생활", "주거", "통신", "구독", "여가", "의료", "교육", "저축", "기타"}
var incomeCategories = []string{"급여", "부수입", "기타"}

var categoryIcons = map[string]string{
	"식비": "🍚", "교통": "🚌", "생활": "🧴", "주거": "🏠", "통신": "📱",
	"구독": "🔔", "여가": "🎮", "의료": "💊", "교육": "📚", "저축": "🐷", "기타": "📦",
	"급여": "💰", "부수입": "➕",
}

func iconFor(cat string) string {
	if v, ok := categoryIcons[cat]; ok {
		return v
	}
	return "📦"
}

func comma(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	out := []byte{}
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	res := string(out)
	if neg {
		res = "-" + res
	}
	return res
}

// noteLines splits free-form note text into non-empty lines for bullet display.
func noteLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimPrefix(line, "・")
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func notePreview(s string) string {
	lines := noteLines(s)
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, " · ")
}

// loadDotEnv reads KEY=VALUE lines from a .env file into the process
// environment, skipping keys that are already set. No-op if the file is missing.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func currentMonth() string {
	return time.Now().Format("2006-01")
}

func currentDate() string {
	return time.Now().Format("2006-01-02")
}

// normalizeDate accepts only a well-formed "2006-01-02" and falls back to
// today otherwise, mirroring normalizeMonth for the same reason: a bogus
// ?date= must not be echoed back as if it were valid.
func normalizeDate(date string) string {
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return currentDate()
	}
	return date
}

func shiftDate(date string, days int) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		t = time.Now()
	}
	return t.AddDate(0, 0, days).Format("2006-01-02")
}

// snapMin rounds to the nearest 10-minute mark. Used both to snap a drag's
// visual position and, on the server, to re-derive the value it must trust
// rather than the client's raw claim.
func snapMin(m int) int {
	return int(math.Round(float64(m)/10)) * 10
}

// minToHHMM renders minutes-since-midnight as "HH:MM", the value <input
// type=time> expects and displays.
func minToHHMM(m int64) string {
	if m < 0 {
		return ""
	}
	return fmt.Sprintf("%02d:%02d", m/60, m%60)
}

// fmtDuration renders whole minutes as "1H 30M", dropping a zero part
// ("2H", "45M") except when the total is itself zero ("0M").
func fmtDuration(mins int) string {
	if mins <= 0 {
		return "0M"
	}
	h, m := mins/60, mins%60
	switch {
	case h == 0:
		return fmt.Sprintf("%dM", m)
	case m == 0:
		return fmt.Sprintf("%dH", h)
	default:
		return fmt.Sprintf("%dH %dM", h, m)
	}
}

// normalizeMonth accepts only a well-formed "2006-01" and falls back to the
// current month otherwise. Without it a bogus ?month= would be echoed in the
// header while the queries silently reported some other month's data.
func normalizeMonth(month string) string {
	if _, err := time.Parse("2006-01", month); err != nil {
		return currentMonth()
	}
	return month
}

// monthRange turns "2006-01" into the half-open date bounds [start, end),
// so queries can compare the indexed date column directly instead of
// wrapping it in substr(), which would prevent an index scan.
func monthRange(month string) (string, string) {
	t, err := time.Parse("2006-01", month)
	if err != nil {
		t = time.Now()
	}
	t = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	return t.Format("2006-01-02"), t.AddDate(0, 1, 0).Format("2006-01-02")
}

func shiftMonth(month string, delta int) string {
	t, err := time.Parse("2006-01", month)
	if err != nil {
		t = time.Now()
	}
	return t.AddDate(0, delta, 0).Format("2006-01")
}

// daysUntil returns the whole-day gap between today and a "2006-01-02"
// deadline: positive when it's still ahead, 0 when due today, negative once
// overdue. ok is false for an unparsable deadline.
func daysUntil(deadline string) (days int, ok bool) {
	d, err := time.Parse("2006-01-02", deadline)
	if err != nil {
		return 0, false
	}
	t, _ := time.Parse("2006-01-02", currentDate())
	return int(d.Sub(t).Hours() / 24), true
}

// ddayLabel renders a deadline as a D-day badge relative to today. Returns
// "" for an empty or unparsable deadline, since the field is optional.
func ddayLabel(deadline string) string {
	days, ok := daysUntil(deadline)
	if !ok {
		return ""
	}
	switch {
	case days == 0:
		return "D-DAY"
	case days > 0:
		return fmt.Sprintf("D-%d", days)
	default:
		return fmt.Sprintf("D+%d", -days)
	}
}

func isExpenseCategory(c string) bool {
	for _, e := range expenseCategories {
		if e == c {
			return true
		}
	}
	return false
}

func isIncomeCategory(c string) bool {
	for _, e := range incomeCategories {
		if e == c {
			return true
		}
	}
	return false
}

// isValidCategory checks a category against the list for the given type.
// The edit modal rebuilds its <select> client-side before setting the value;
// a category that has fallen out of the known list (e.g. a stray API call,
// or a future category rename) would leave the select unselected and submit
// an empty string, silently blanking the transaction's category if the
// server didn't also check.
func isValidCategory(typ, category string) bool {
	if typ == "income" {
		return isIncomeCategory(category)
	}
	return isExpenseCategory(category)
}
