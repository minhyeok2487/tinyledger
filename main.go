package main

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"os"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

var tpl *template.Template

func main() {
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

	addr := ":8080"
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}
	log.Println("가계부 서버 시작:", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
