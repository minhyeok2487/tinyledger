package main

import (
	"bufio"
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

func shiftMonth(month string, delta int) string {
	t, err := time.Parse("2006-01", month)
	if err != nil {
		t = time.Now()
	}
	return t.AddDate(0, delta, 0).Format("2006-01")
}
