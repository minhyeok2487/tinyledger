// Package handler is the Vercel Go serverless entrypoint. All routes are
// rewritten to this single function (see vercel.json), which delegates to
// the same http.Handler used by the local dev server in main.go.
package handler

import (
	"net/http"

	"gagyebu/internal/app"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	app.NewHandler().ServeHTTP(w, r)
}
