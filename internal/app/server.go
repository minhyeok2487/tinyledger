package app

import (
	"embed"
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

	var err error
	tpl, err = template.New("").Funcs(template.FuncMap{
		"comma":       comma,
		"icon":        iconFor,
		"lines":       noteLines,
		"notePreview": notePreview,
	}).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", handleDashboard)
	mux.HandleFunc("POST /add", handleAdd)
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

	mux.HandleFunc("POST /notes", handleNoteSave)

	mux.Handle("GET /static/", http.FileServerFS(staticFS))

	handler = mux
}
