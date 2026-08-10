package app

import "testing"

func TestNormalizeDate(t *testing.T) {
	if got := normalizeDate("2026-08-10"); got != "2026-08-10" {
		t.Errorf("valid date changed: got %q", got)
	}
	today := currentDate()
	for _, bad := range []string{"", "abc", "2026-13-01", "2026-08-32", "08-10-2026", "2026/08/10"} {
		if got := normalizeDate(bad); got != today {
			t.Errorf("normalizeDate(%q) = %q, want %q", bad, got, today)
		}
	}
}

func TestShiftDate(t *testing.T) {
	tests := []struct {
		date  string
		delta int
		want  string
	}{
		{"2026-08-10", 1, "2026-08-11"},
		{"2026-08-10", -1, "2026-08-09"},
		{"2026-08-31", 1, "2026-09-01"},  // month boundary
		{"2026-12-31", 1, "2027-01-01"},  // year boundary
		{"2024-02-28", 1, "2024-02-29"},  // leap year
		{"2026-03-01", -1, "2026-02-28"}, // non-leap year back over Feb
	}
	for _, tt := range tests {
		if got := shiftDate(tt.date, tt.delta); got != tt.want {
			t.Errorf("shiftDate(%q, %d) = %q, want %q", tt.date, tt.delta, got, tt.want)
		}
	}
}

func TestSnapMin(t *testing.T) {
	tests := []struct {
		in, want int
	}{
		{0, 0}, {4, 0}, {5, 10}, {9, 10}, {10, 10},
		{599, 600}, {600, 600}, {1439, 1440},
	}
	for _, tt := range tests {
		if got := snapMin(tt.in); got != tt.want {
			t.Errorf("snapMin(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestFmtDuration(t *testing.T) {
	tests := []struct {
		mins int
		want string
	}{
		{0, "0M"}, {-5, "0M"}, {30, "30M"}, {60, "1H"}, {90, "1H 30M"}, {150, "2H 30M"},
	}
	for _, tt := range tests {
		if got := fmtDuration(tt.mins); got != tt.want {
			t.Errorf("fmtDuration(%d) = %q, want %q", tt.mins, got, tt.want)
		}
	}
}

func TestMinToHHMM(t *testing.T) {
	tests := []struct {
		mins int64
		want string
	}{
		{-1, ""}, {0, "00:00"}, {90, "01:30"}, {1439, "23:59"},
	}
	for _, tt := range tests {
		if got := minToHHMM(tt.mins); got != tt.want {
			t.Errorf("minToHHMM(%d) = %q, want %q", tt.mins, got, tt.want)
		}
	}
}

func TestClampSchedule(t *testing.T) {
	tests := []struct {
		name         string
		start, end   int
		wantS, wantE int
	}{
		{"already aligned", 600, 660, 600, 660},
		{"snaps both ends", 573, 605, 570, 610},
		{"inverted end<=start becomes 10min block", 600, 590, 600, 610},
		{"equal start/end becomes 10min block", 600, 600, 600, 610},
		{"negative start clamps to 0", -30, 60, 0, 60},
		{"end past midnight clamps to 1440", 1400, 1500, 1400, 1440},
		{"both out of range clamp to last 10min of day", -30, 1500, 0, 1440},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotS, gotE := clampSchedule(tt.start, tt.end)
			if gotS != tt.wantS || gotE != tt.wantE {
				t.Errorf("clampSchedule(%d,%d) = (%d,%d), want (%d,%d)",
					tt.start, tt.end, gotS, gotE, tt.wantS, tt.wantE)
			}
			if gotE <= gotS {
				t.Errorf("clampSchedule(%d,%d) produced end<=start: (%d,%d)", tt.start, tt.end, gotS, gotE)
			}
		})
	}
}

func TestDdayLabel(t *testing.T) {
	today := currentDate()
	tests := []struct {
		name     string
		deadline string
		want     string
	}{
		{"no deadline", "", ""},
		{"due today", today, "D-DAY"},
		{"3 days out", shiftDate(today, 3), "D-3"},
		{"2 days overdue", shiftDate(today, -2), "D+2"},
		{"garbage input", "not-a-date", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ddayLabel(tt.deadline); got != tt.want {
				t.Errorf("ddayLabel(%q) = %q, want %q", tt.deadline, got, tt.want)
			}
		})
	}
}

func TestTaskDdayHelpers(t *testing.T) {
	today := currentDate()
	noDeadline := Task{Deadline: ""}
	if noDeadline.HasDeadline() {
		t.Error("empty deadline should report HasDeadline() == false")
	}
	if got := noDeadline.DdayClass(); got != "" {
		t.Errorf("DdayClass() on empty deadline = %q, want empty", got)
	}

	dueToday := Task{Deadline: today}
	if !dueToday.HasDeadline() {
		t.Error("non-empty deadline should report HasDeadline() == true")
	}
	if got := dueToday.DdayClass(); got != "tt-dday-today" {
		t.Errorf("DdayClass() due today = %q, want tt-dday-today", got)
	}

	overdue := Task{Deadline: shiftDate(today, -1)}
	if got := overdue.DdayClass(); got != "tt-dday-over" {
		t.Errorf("DdayClass() overdue = %q, want tt-dday-over", got)
	}

	upcoming := Task{Deadline: shiftDate(today, 1)}
	if got := upcoming.DdayClass(); got != "" {
		t.Errorf("DdayClass() upcoming = %q, want empty", got)
	}
	if got := upcoming.DdayLabel(); got != "D-1" {
		t.Errorf("DdayLabel() upcoming = %q, want D-1", got)
	}
}

func TestTaskScheduledAndPositioning(t *testing.T) {
	unscheduled := Task{StartMin: -1, EndMin: -1}
	if unscheduled.Scheduled() {
		t.Error("StartMin -1 should mean not scheduled")
	}

	t10to11 := Task{StartMin: 600, EndMin: 660}
	if !t10to11.Scheduled() {
		t.Error("a task with a real start time should be scheduled")
	}
	if t10to11.TopPx() != 600 {
		t.Errorf("TopPx() = %d, want 600 (dawn grid is 0-relative)", t10to11.TopPx())
	}
	// The main grid starts at 06:00 (360 minutes), so its blocks must be
	// rendered relative to that, or they land ~6 hours too low — the bug
	// this test guards against.
	if t10to11.MainTopPx() != 240 {
		t.Errorf("MainTopPx() = %d, want 240 (600 - 6*60)", t10to11.MainTopPx())
	}
	if t10to11.HeightPx() != 60 {
		t.Errorf("HeightPx() = %d, want 60", t10to11.HeightPx())
	}
	if got := t10to11.Duration(); got != "1H" {
		t.Errorf("Duration() = %q, want 1H", got)
	}
}
