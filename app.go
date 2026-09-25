package main

import (
	"database/sql"
	"log"
	"net/http"
	"time"
)

// App holds everything our handlers need.
type App struct {
	DB         *sql.DB
	Secret     []byte
	AgentEmail string
}

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", a.health)
	mux.HandleFunc("POST /api/register", a.register)
	mux.HandleFunc("POST /api/login", a.login)

	mux.Handle("GET /api/me", a.auth(a.me))
	mux.Handle("POST /api/tickets", a.auth(a.createTicket))
	mux.Handle("GET /api/tickets", a.auth(a.listTickets))
	mux.Handle("GET /api/tickets/{id}", a.auth(a.getTicket))
	mux.Handle("PATCH /api/tickets/{id}", a.auth(a.updateTicket))
	mux.Handle("DELETE /api/tickets/{id}", a.auth(a.deleteTicket))
	mux.Handle("POST /api/tickets/{id}/comments", a.auth(a.addComment))
	mux.Handle("GET /api/tickets/{id}/comments", a.auth(a.listComments))

	return logRequests(mux)
}

func (a *App) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// logRequests prints one line for every request.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s (%s)", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}
