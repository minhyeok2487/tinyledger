package app

import (
	"database/sql"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const maxTodaySlots = 5
const taskColors = 6

func handleTimetable(w http.ResponseWriter, r *http.Request) {
	date := normalizeDate(r.URL.Query().Get("date"))

	tasks, err := listTasks(date)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	var backlog, planned, dawnBlocks, mainBlocks []Task
	for _, t := range tasks {
		if t.TodayOrder == 0 {
			backlog = append(backlog, t)
		}
		if t.TodayOrder > 0 {
			planned = append(planned, t)
		}
		if t.Scheduled() {
			if t.StartMin < 6*60 {
				dawnBlocks = append(dawnBlocks, t)
			} else {
				mainBlocks = append(mainBlocks, t)
			}
		}
	}
	sort.Slice(planned, func(i, j int) bool { return planned[i].TodayOrder < planned[j].TodayOrder })
	sort.Slice(dawnBlocks, func(i, j int) bool { return dawnBlocks[i].StartMin < dawnBlocks[j].StartMin })
	sort.Slice(mainBlocks, func(i, j int) bool { return mainBlocks[i].StartMin < mainBlocks[j].StartMin })

	today := currentDate()
	dawnHours := make([]int, 6)
	for h := range dawnHours {
		dawnHours[h] = h
	}
	dayHours := make([]int, 18)
	for i := range dayHours {
		dayHours[i] = i + 6
	}

	categories, err := listCategories()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	data := TimetableData{
		Date:        date,
		PrevDate:    shiftDate(date, -1),
		NextDate:    shiftDate(date, 1),
		TodayDate:   today,
		IsToday:     date == today,
		DawnHours:   dawnHours,
		DayHours:    dayHours,
		Backlog:     backlog,
		Planned:     planned,
		DawnBlocks:  dawnBlocks,
		MainBlocks:  mainBlocks,
		TotalBlocks: len(dawnBlocks) + len(mainBlocks),
		Categories:  categories,
		Nav:         "timetable",
	}
	if err := tpl.ExecuteTemplate(w, "timetable.html", data); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func listTasks(date string) ([]Task, error) {
	rows, err := db.Query(`SELECT id, title, note, date,
			COALESCE(today_order,0), COALESCE(start_min,-1), COALESCE(end_min,-1),
			color, done, sort_order, category, deadline
		FROM tasks WHERE date = ? ORDER BY sort_order, id`, date)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer rows.Close()

	var out []Task
	for rows.Next() {
		var t Task
		var note sql.NullString
		if err := rows.Scan(&t.ID, &t.Title, &note, &t.Date,
			&t.TodayOrder, &t.StartMin, &t.EndMin, &t.Color, &t.Done, &t.SortOrder,
			&t.Category, &t.Deadline); err != nil {
			continue
		}
		t.Note = note.String
		out = append(out, t)
	}
	return out, rows.Err()
}

// timetableRedirect preserves the date being viewed, so the partial update
// swaps in markup for the same day the user is looking at.
func timetableRedirect(r *http.Request) string {
	v := url.Values{}
	v.Set("date", normalizeDate(r.FormValue("date")))
	return "/timetable?" + v.Encode()
}

func handleDumpAdd(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	date := normalizeDate(r.FormValue("date"))
	if title := r.FormValue("title"); title != "" {
		var count int
		db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE date = ?`, date).Scan(&count)
		db.Exec(`INSERT INTO tasks(title, date, color, sort_order)
			VALUES (?, ?, ?, (SELECT COALESCE(MAX(sort_order),-1)+1 FROM tasks WHERE date = ?))`,
			title, date, count%taskColors, date)
	}
	http.Redirect(w, r, timetableRedirect(r), http.StatusSeeOther)
}

func handleDumpDelete(w http.ResponseWriter, r *http.Request) {
	db.Exec(`DELETE FROM tasks WHERE id = ?`, r.PathValue("id"))
	http.Redirect(w, r, timetableRedirect(r), http.StatusSeeOther)
}

// handleTodaySet appends a task to the end of 오늘의 계획 (capped at
// maxTodaySlots) — drops always land at the end rather than at a specific
// slot, so there's no drop-position math to get right for this list.
func handleTodaySet(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	setTodayOrder(r.PathValue("id"), normalizeDate(r.FormValue("date")))
	http.Redirect(w, r, timetableRedirect(r), http.StatusSeeOther)
}

// handleTodayUnset removes a task from 오늘의 계획 and re-packs the
// remaining slots to stay contiguous (1..N), so no gaps ever show up.
func handleTodayUnset(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	unsetTodayOrder(r.PathValue("id"), normalizeDate(r.FormValue("date")))
	http.Redirect(w, r, timetableRedirect(r), http.StatusSeeOther)
}

// setTodayOrder and unsetTodayOrder are shared by the today/set and
// today/unset routes (reached by drag) and by handleTaskEdit's "오늘의
// 계획에 추가" checkbox (the drag-free path, needed since native HTML5
// drag-and-drop doesn't work on touch browsers).
func setTodayOrder(id, date string) {
	res, err := db.Exec(`UPDATE tasks SET today_order =
			(SELECT COUNT(*) FROM tasks WHERE date = ? AND today_order IS NOT NULL) + 1
		WHERE id = ? AND date = ? AND today_order IS NULL
			AND (SELECT COUNT(*) FROM tasks WHERE date = ? AND today_order IS NOT NULL) < ?`,
		date, id, date, date, maxTodaySlots)
	if err != nil {
		return
	}
	// Only auto-place if the UPDATE actually claimed a slot — a full day or
	// an already-planned task must not get (re)scheduled.
	if n, _ := res.RowsAffected(); n > 0 {
		autoScheduleIfUnscheduled(id, date)
	}
}

const dayStartMin = 6 * 60 // 06:00, matching the main grid's start

// autoScheduleIfUnscheduled gives a freshly-planned task a spot on the grid
// so 오늘의 계획 and 시간표 stay in sync — a task never shows in "today's
// plan" while being invisible on the schedule. Leaves already-scheduled
// tasks untouched (re-planning one shouldn't move its existing time block).
//
// The free-slot search and the write happen in a single UPDATE statement
// rather than a SELECT-then-decide-then-UPDATE: two concurrent requests
// (e.g. two tabs dragging different tasks into 오늘의 계획 at once) would
// otherwise both read the schedule before either writes, and both land on
// the same "free" slot. A lone statement is one atomic write as far as
// SQLite/Turso is concerned, so the second request only starts once the
// first has committed and sees its result.
func autoScheduleIfUnscheduled(id, date string) {
	const duration = 30
	var starts []string
	var args []any
	for start := dayStartMin; start+duration <= 24*60; start += 30 {
		starts = append(starts, "(?)")
		args = append(args, start)
	}
	if len(starts) == 0 {
		return
	}

	query := `WITH slot(v) AS (
		SELECT COALESCE(MIN(c.column1), ?) FROM (VALUES ` + strings.Join(starts, ",") + `) AS c
		WHERE NOT EXISTS (
			SELECT 1 FROM tasks t2 WHERE t2.date = ? AND t2.start_min IS NOT NULL
				AND t2.start_min < c.column1 + ? AND c.column1 < t2.end_min
		)
	)
	UPDATE tasks SET
		start_min = (SELECT v FROM slot),
		end_min = (SELECT v FROM slot) + ?
	WHERE id = ? AND date = ? AND start_min IS NULL`

	full := append([]any{dayStartMin}, args...)
	full = append(full, date, duration, duration, id, date)
	if _, err := db.Exec(query, full...); err != nil {
		log.Println("autoScheduleIfUnscheduled:", err)
	}
}

func unsetTodayOrder(id, date string) {
	tx, err := db.Begin()
	if err != nil {
		return
	}
	var order sql.NullInt64
	if scanErr := tx.QueryRow(`SELECT today_order FROM tasks WHERE id = ?`, id).Scan(&order); scanErr == nil && order.Valid {
		tx.Exec(`UPDATE tasks SET today_order = NULL WHERE id = ?`, id)
		tx.Exec(`UPDATE tasks SET today_order = today_order - 1
			WHERE date = ? AND today_order > ?`, date, order.Int64)
		tx.Commit()
	} else {
		tx.Rollback()
	}
}

func handleTodayToggle(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	db.Exec(`UPDATE tasks SET done = 1 - done WHERE id = ?`, r.PathValue("id"))
	http.Redirect(w, r, timetableRedirect(r), http.StatusSeeOther)
}

// handleSchedule places a task on the grid (drag-from-Brain-Dump, drag-move,
// or drag-resize all commit here). The client already snaps visually, but
// the server snaps and clamps again rather than trusting the posted value.
func handleSchedule(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	start, err1 := strconv.Atoi(r.FormValue("start_min"))
	end, err2 := strconv.Atoi(r.FormValue("end_min"))
	if err1 != nil || err2 != nil {
		http.Error(w, "invalid input", 400)
		return
	}
	start, end = clampSchedule(start, end)

	db.Exec(`UPDATE tasks SET start_min = ?, end_min = ? WHERE id = ?`, start, end, r.PathValue("id"))
	http.Redirect(w, r, timetableRedirect(r), http.StatusSeeOther)
}

// clampSchedule is pure so it's testable without a database: snap both ends
// to the nearest 10 minutes, then clamp into [0,1440] and guarantee end>start
// (a zero-length or inverted block from a fast/short drag would otherwise
// render as nothing, or below its own start).
func clampSchedule(start, end int) (int, int) {
	start, end = snapMin(start), snapMin(end)
	if start < 0 {
		start = 0
	}
	if end > 24*60 {
		end = 24 * 60
	}
	if end <= start {
		end = start + 10
	}
	if end > 24*60 {
		start, end = 24*60-10, 24*60
	}
	return start, end
}

func handleUnschedule(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	db.Exec(`UPDATE tasks SET start_min = NULL, end_min = NULL WHERE id = ?`, r.PathValue("id"))
	http.Redirect(w, r, timetableRedirect(r), http.StatusSeeOther)
}

// handleTaskEdit is the modal's save action — the JS-off-safe way to both
// rename/annotate a task and to schedule it (leaving the time fields blank
// clears any existing schedule instead of leaving stale values).
func handleTaskEdit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	id := r.PathValue("id")
	date := normalizeDate(r.FormValue("date"))
	title := r.FormValue("title")
	if title == "" {
		http.Error(w, "title required", 400)
		return
	}
	note := r.FormValue("note")
	color, _ := strconv.Atoi(r.FormValue("color"))
	if color < 0 || color >= taskColors {
		color = 0
	}
	category := strings.TrimSpace(r.FormValue("category"))

	// Deadline is optional — a blank field just clears it. When present it
	// must still be a real date, same as the time fields below.
	deadline := r.FormValue("deadline")
	if deadline != "" {
		if _, err := time.Parse("2006-01-02", deadline); err != nil {
			http.Error(w, "invalid deadline", 400)
			return
		}
	}

	// The modal uses <input type=time>, which posts "HH:MM" — distinct from
	// handleSchedule's raw-minute integers, which come from the drag JS.
	startStr, endStr := r.FormValue("start_min"), r.FormValue("end_min")
	if startStr == "" || endStr == "" {
		db.Exec(`UPDATE tasks SET title = ?, note = ?, color = ?, category = ?, deadline = ?, start_min = NULL, end_min = NULL WHERE id = ?`,
			title, note, color, category, deadline, id)
	} else {
		start, err1 := time.Parse("15:04", startStr)
		end, err2 := time.Parse("15:04", endStr)
		if err1 != nil || err2 != nil {
			http.Error(w, "invalid time", 400)
			return
		}
		startMin, endMin := clampSchedule(start.Hour()*60+start.Minute(), end.Hour()*60+end.Minute())
		db.Exec(`UPDATE tasks SET title = ?, note = ?, color = ?, category = ?, deadline = ?, start_min = ?, end_min = ? WHERE id = ?`,
			title, note, color, category, deadline, startMin, endMin, id)
	}

	// "오늘의 계획에 추가" checkbox — the modal's non-drag path to the same
	// effect as dragging a Brain Dump item onto the plan list, needed since
	// native HTML5 drag-and-drop doesn't work on touch browsers.
	if r.FormValue("today") == "on" {
		setTodayOrder(id, date)
	} else {
		unsetTodayOrder(id, date)
	}
	http.Redirect(w, r, timetableRedirect(r), http.StatusSeeOther)
}

func handleTaskDelete(w http.ResponseWriter, r *http.Request) {
	db.Exec(`DELETE FROM tasks WHERE id = ?`, r.PathValue("id"))
	http.Redirect(w, r, timetableRedirect(r), http.StatusSeeOther)
}

func listCategories() ([]TaskCategory, error) {
	rows, err := db.Query(`SELECT id, name FROM task_categories ORDER BY sort_order, id`)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer rows.Close()

	var out []TaskCategory
	for rows.Next() {
		var c TaskCategory
		if rows.Scan(&c.ID, &c.Name) == nil {
			out = append(out, c)
		}
	}
	return out, rows.Err()
}

func handleCategoryAdd(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if name := strings.TrimSpace(r.FormValue("name")); name != "" {
		if _, err := db.Exec(`INSERT INTO task_categories(name, sort_order)
			VALUES (?, (SELECT COALESCE(MAX(sort_order),-1)+1 FROM task_categories))
			ON CONFLICT(name) DO NOTHING`, name); err != nil {
			log.Println("category add:", err)
		}
	}
	http.Redirect(w, r, timetableRedirect(r), http.StatusSeeOther)
}

func handleCategoryRename(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if name := strings.TrimSpace(r.FormValue("name")); name != "" {
		// A clash with an existing name is rejected by the UNIQUE index; log
		// it rather than pretending the rename happened.
		if _, err := db.Exec(`UPDATE task_categories SET name = ? WHERE id = ?`, name, r.PathValue("id")); err != nil {
			log.Println("category rename:", err)
		}
	}
	http.Redirect(w, r, timetableRedirect(r), http.StatusSeeOther)
}

// handleCategoryDelete only removes the managed label — tasks that already
// carry it in their (free-text) category column are left untouched, same as
// a hobby item rename doesn't rewrite past transactions.
func handleCategoryDelete(w http.ResponseWriter, r *http.Request) {
	db.Exec(`DELETE FROM task_categories WHERE id = ?`, r.PathValue("id"))
	http.Redirect(w, r, timetableRedirect(r), http.StatusSeeOther)
}
