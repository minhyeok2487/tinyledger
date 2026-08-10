package app

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"html/template"
	"log"
	"net/http"
	"os"
	"sync"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

var tpl *template.Template

// cssVersion is a short hash of style.css's embedded contents, appended to
// its <link> URL as a cache-buster. static/ is served with a 1-hour
// max-age, so without this every CSS edit would stay invisible to a
// returning browser until that cache expired — a new content hash forces
// an immediate fetch of the new file on every deploy that changes it.
var cssVersion string

func computeCSSVersion() {
	b, err := staticFS.ReadFile("static/style.css")
	if err != nil {
		log.Println("computeCSSVersion:", err)
		return
	}
	sum := sha256.Sum256(b)
	cssVersion = hex.EncodeToString(sum[:])[:8]
}

var (
	initOnce sync.Once
	handler  http.Handler
)

// NewHandler lazily initializes the DB connection, templates, and routes on
// first call, then returns the same http.Handler on every subsequent call.
// Safe to call on every request (e.g. from a serverless function entrypoint).
func NewHandler() http.Handler {
	initOnce.Do(setup)
	return handler
}

func setup() {
	loadDotEnv(".env")

	if url := os.Getenv("TURSO_DATABASE_URL"); url != "" {
		initTursoDB(url, os.Getenv("TURSO_AUTH_TOKEN"))
	} else {
		dbPath := "gagyebu.db"
		if v := os.Getenv("GAGYEBU_DB"); v != "" {
			dbPath = v
		}
		initDB(dbPath)
	}

	computeCSSVersion()

	var err error
	tpl, err = template.New("").Funcs(template.FuncMap{
		"comma":       comma,
		"icon":        iconFor,
		"lines":       noteLines,
		"notePreview": notePreview,
		"authEnabled": authEnabled,
		"add1":        func(i int) int { return i + 1 },
		"cssVersion":  func() string { return cssVersion },
	}).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /login", handleLoginPage)
	mux.HandleFunc("POST /login", handleLogin)
	mux.HandleFunc("POST /logout", handleLogout)

	mux.HandleFunc("GET /{$}", handleDashboard)
	mux.HandleFunc("POST /add", handleAdd)
	mux.HandleFunc("POST /update/{id}", handleTransactionUpdate)
	mux.HandleFunc("POST /delete/{id}", handleDelete)

	mux.HandleFunc("GET /calendar", handleCalendar)

	mux.HandleFunc("GET /search", handleSearch)

	mux.HandleFunc("GET /templates", handleTemplates)
	mux.HandleFunc("POST /templates/add", handleTemplateAdd)
	mux.HandleFunc("POST /templates/delete/{id}", handleTemplateDelete)

	mux.HandleFunc("GET /accounts", handleAccounts)
	mux.HandleFunc("POST /accounts/add", handleAccountAdd)
	mux.HandleFunc("POST /accounts/update/{id}", handleAccountUpdate)
	mux.HandleFunc("POST /accounts/delete/{id}", handleAccountDelete)
	mux.HandleFunc("POST /accounts/transfer", handleTransfer)

	mux.HandleFunc("GET /hobby", handleHobby)
	mux.HandleFunc("POST /hobby/items/add", handleHobbyItemAdd)
	mux.HandleFunc("POST /hobby/items/rename/{id}", handleHobbyItemRename)
	mux.HandleFunc("POST /hobby/items/delete/{id}", handleHobbyItemDelete)
	mux.HandleFunc("POST /hobby/assign/{id}", handleHobbyAssign)

	mux.HandleFunc("GET /wishlist", handleWishlist)
	mux.HandleFunc("POST /wishlist/add", handleWishAdd)
	mux.HandleFunc("POST /wishlist/update/{id}", handleWishUpdate)
	mux.HandleFunc("POST /wishlist/delete/{id}", handleWishDelete)
	mux.HandleFunc("POST /wishlist/buy/{id}", handleWishBuy)

	mux.HandleFunc("GET /timetable", handleTimetable)
	mux.HandleFunc("POST /timetable/dump/add", handleDumpAdd)
	mux.HandleFunc("POST /timetable/dump/delete/{id}", handleDumpDelete)
	mux.HandleFunc("POST /timetable/today/set/{id}", handleTodaySet)
	mux.HandleFunc("POST /timetable/today/unset/{id}", handleTodayUnset)
	mux.HandleFunc("POST /timetable/today/toggle/{id}", handleTodayToggle)
	mux.HandleFunc("POST /timetable/schedule/{id}", handleSchedule)
	mux.HandleFunc("POST /timetable/unschedule/{id}", handleUnschedule)
	mux.HandleFunc("POST /timetable/edit/{id}", handleTaskEdit)
	mux.HandleFunc("POST /timetable/delete/{id}", handleTaskDelete)
	mux.HandleFunc("POST /timetable/categories/add", handleCategoryAdd)
	mux.HandleFunc("POST /timetable/categories/rename/{id}", handleCategoryRename)
	mux.HandleFunc("POST /timetable/categories/delete/{id}", handleCategoryDelete)

	mux.HandleFunc("POST /notes", handleNoteSave)

	static := http.FileServerFS(staticFS)
	mux.Handle("GET /static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		static.ServeHTTP(w, r)
	}))

	handler = requireAuth(mux)
}
