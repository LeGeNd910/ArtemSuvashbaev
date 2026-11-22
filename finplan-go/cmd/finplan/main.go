package main

import (
	"log"
	"net/http"
	"time"

	"finplan/internal/storage"
	"finplan/internal/webapp"
)

func main() {
	store, err := storage.Open("finplan.db")
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	if err := store.Prepare(); err != nil {
		log.Fatal(err)
	}

	app, err := webapp.New(store)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", app.Dashboard)
	mux.HandleFunc("GET /about", app.About)
	mux.HandleFunc("GET /ping", app.Ping)
	mux.HandleFunc("GET /expenses/new", app.NewExpense)
	mux.HandleFunc("POST /expenses", app.CreateExpense)
	mux.HandleFunc("GET /expenses/{id}/edit", app.EditExpense)
	mux.HandleFunc("POST /expenses/{id}/update", app.UpdateExpense)
	mux.HandleFunc("POST /expenses/{id}/delete", app.DeleteExpense)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	server := &http.Server{
		Addr:              ":8080",
		Handler:           app.RequestLog(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Println("ФинПлан запущен: http://localhost:8080")
	log.Fatal(server.ListenAndServe())
}
