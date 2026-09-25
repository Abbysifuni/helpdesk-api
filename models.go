package main

import "time"

type User struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	Role         string    `json:"role"` // "customer" or "agent"
	PasswordHash string    `json:"-"`    // never sent to clients
	CreatedAt    time.Time `json:"created_at"`
}

type Ticket struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`   // open, in_progress, resolved, closed
	Priority    string    `json:"priority"` // low, medium, high
	CreatedBy   int64     `json:"created_by"`
	AssignedTo  *int64    `json:"assigned_to"` // nil = not assigned yet
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Comment struct {
	ID        int64     `json:"id"`
	TicketID  int64     `json:"ticket_id"`
	UserID    int64     `json:"user_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

var validStatus = map[string]bool{"open": true, "in_progress": true, "resolved": true, "closed": true}
var validPriority = map[string]bool{"low": true, "medium": true, "high": true}
