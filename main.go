package main

import (
	"log"
	"net/http"
	"os"

	"gagyebu/internal/app"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"
)

func main() {
	addr := ":8080"
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}
	log.Println("가계부 서버 시작:", addr)
	log.Fatal(http.ListenAndServe(addr, app.NewHandler()))
}
