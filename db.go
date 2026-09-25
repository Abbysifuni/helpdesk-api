package main

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3" // registers the "sqlite3" driver
)

const schema = `
CREATE TABLE IF NOT EXISTS users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    name          TEXT NOT NULL,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'customer',
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tickets (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'open',
    priority    TEXT NOT NULL DEFAULT 'medium',
    created_by  INTEGER NOT NULL REFERENCES users(id),
    assigned_to INTEGER REFERENCES users(id),
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS comments (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    ticket_id  INTEGER NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    user_id    INTEGER NOT NULL REFERENCES users(id),
    body       TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);`

// openDB opens the SQLite file and creates the tables if needed.
func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite likes one writer at a time
	if err := db.Ping(); err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	return db, nil
}
