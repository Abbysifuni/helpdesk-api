// Helpdesk API — a small ticket system built with Go, SQLite and JWT.
package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	addr := getenv("ADDR", ":8080")
	dbPath := getenv("DB_PATH", "helpdesk.db")
	secret := getenv("JWT_SECRET", "change-me-in-production")

	db, err := openDB(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	app := &App{
		DB:         db,
		Secret:     []byte(secret),
		AgentEmail: os.Getenv("AGENT_EMAIL"), // this user becomes a support agent
	}

	log.Println("helpdesk API listening on", addr)
	log.Fatal(http.ListenAndServe(addr, app.routes()))
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
