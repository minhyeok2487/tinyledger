package app

import "testing"

func TestMonthRange(t *testing.T) {
	tests := []struct {
		month, start, end string
	}{
		{"2026-08", "2026-08-01", "2026-09-01"},
		{"2025-12", "2025-12-01", "2026-01-01"}, // year boundary
		{"2026-02", "2026-02-01", "2026-03-01"}, // 28 days
		{"2024-02", "2024-02-01", "2024-03-01"}, // leap year
		{"2026-01", "2026-01-01", "2026-02-01"},
	}
	for _, tt := range tests {
		start, end := monthRange(tt.month)
		if start != tt.start || end != tt.end {
			t.Errorf("monthRange(%q) = %q..%q, want %q..%q", tt.month, start, end, tt.start, tt.end)
		}
	}
}

// The range is half-open, so a transaction on the last day of the month must
// fall inside it and the first of the next month must not.
func TestMonthRangeIncludesLastDay(t *testing.T) {
	start, end := monthRange("2026-08")
	for _, date := range []string{"2026-08-01", "2026-08-15", "2026-08-31"} {
		if !(date >= start && date < end) {
			t.Errorf("%s should be inside %s..%s", date, start, end)
		}
	}
	for _, date := range []string{"2026-07-31", "2026-09-01"} {
		if date >= start && date < end {
			t.Errorf("%s should be outside %s..%s", date, start, end)
		}
	}
}

func TestNormalizeMonth(t *testing.T) {
	if got := normalizeMonth("2026-08"); got != "2026-08" {
		t.Errorf("valid month changed: got %q", got)
	}
	now := currentMonth()
	// Anything malformed must fall back rather than be echoed back to the page.
	for _, bad := range []string{"", "abc", "2026-13", "2026-8", "2026-01-15", "26-01"} {
		if got := normalizeMonth(bad); got != now {
			t.Errorf("normalizeMonth(%q) = %q, want %q", bad, got, now)
		}
	}
}

func TestShiftMonth(t *testing.T) {
	tests := []struct {
		month string
		delta int
		want  string
	}{
		{"2026-08", 1, "2026-09"},
		{"2026-12", 1, "2027-01"},
		{"2026-01", -1, "2025-12"},
		{"2026-03", -1, "2026-02"},
	}
	for _, tt := range tests {
		if got := shiftMonth(tt.month, tt.delta); got != tt.want {
			t.Errorf("shiftMonth(%q, %d) = %q, want %q", tt.month, tt.delta, got, tt.want)
		}
	}
}

func TestComma(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1,000"},
		{1539582, "1,539,582"},
		{-32000, "-32,000"},
	}
	for _, tt := range tests {
		if got := comma(tt.in); got != tt.want {
			t.Errorf("comma(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
