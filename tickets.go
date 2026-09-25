package main

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

const ticketCols = `id, title, description, status, priority, created_by, assigned_to, created_at, updated_at`

type scanner interface{ Scan(dest ...any) error }

func scanTicket(s scanner) (Ticket, error) {
	var t Ticket
	err := s.Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.Priority,
		&t.CreatedBy, &t.AssignedTo, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

// findTicket loads a ticket the user is allowed to see.
// Customers only see their own tickets; agents see everything.
func (a *App) findTicket(w http.ResponseWriter, r *http.Request, u User) (Ticket, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ticket id")
		return Ticket{}, false
	}
	t, err := scanTicket(a.DB.QueryRow(`SELECT `+ticketCols+` FROM tickets WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) || (err == nil && u.Role != "agent" && t.CreatedBy != u.ID) {
		writeError(w, http.StatusNotFound, "ticket not found")
		return Ticket{}, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return Ticket{}, false
	}
	return t, true
}

// POST /api/tickets
func (a *App) createTicket(w http.ResponseWriter, r *http.Request, u User) {
	var in struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Priority    string `json:"priority"`
	}
	if err := readJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	if in.Priority == "" {
		in.Priority = "medium"
	}
	if !validPriority[in.Priority] {
		writeError(w, http.StatusBadRequest, "priority must be low, medium or high")
		return
	}
	res, err := a.DB.Exec(`INSERT INTO tickets (title, description, priority, created_by) VALUES (?, ?, ?, ?)`,
		in.Title, in.Description, in.Priority, u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create ticket")
		return
	}
	id, _ := res.LastInsertId()
	t, _ := scanTicket(a.DB.QueryRow(`SELECT `+ticketCols+` FROM tickets WHERE id = ?`, id))
	writeJSON(w, http.StatusCreated, t)
}

// GET /api/tickets?status=open&priority=high
func (a *App) listTickets(w http.ResponseWriter, r *http.Request, u User) {
	query := `SELECT ` + ticketCols + ` FROM tickets WHERE 1=1`
	var args []any
	if u.Role != "agent" {
		query += ` AND created_by = ?`
		args = append(args, u.ID)
	}
	if s := r.URL.Query().Get("status"); s != "" {
		query += ` AND status = ?`
		args = append(args, s)
	}
	if p := r.URL.Query().Get("priority"); p != "" {
		query += ` AND priority = ?`
		args = append(args, p)
	}
	query += ` ORDER BY id DESC`

	rows, err := a.DB.Query(query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	tickets := []Ticket{} // empty list, not null
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
		tickets = append(tickets, t)
	}
	writeJSON(w, http.StatusOK, tickets)
}

// GET /api/tickets/{id}
func (a *App) getTicket(w http.ResponseWriter, r *http.Request, u User) {
	if t, ok := a.findTicket(w, r, u); ok {
		writeJSON(w, http.StatusOK, t)
	}
}

// PATCH /api/tickets/{id}
func (a *App) updateTicket(w http.ResponseWriter, r *http.Request, u User) {
	t, ok := a.findTicket(w, r, u)
	if !ok {
		return
	}
	var in struct { // pointers: nil means "not sent"
		Status     *string `json:"status"`
		Priority   *string `json:"priority"`
		AssignedTo *int64  `json:"assigned_to"`
	}
	if err := readJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	isAgent := u.Role == "agent"

	if in.Status != nil {
		if !validStatus[*in.Status] {
			writeError(w, http.StatusBadRequest, "status must be open, in_progress, resolved or closed")
			return
		}
		if !isAgent && *in.Status != "closed" {
			writeError(w, http.StatusForbidden, "customers can only close their tickets")
			return
		}
		t.Status = *in.Status
	}
	if in.Priority != nil {
		if !isAgent {
			writeError(w, http.StatusForbidden, "only agents can change priority")
			return
		}
		if !validPriority[*in.Priority] {
			writeError(w, http.StatusBadRequest, "priority must be low, medium or high")
			return
		}
		t.Priority = *in.Priority
	}
	if in.AssignedTo != nil {
		if !isAgent {
			writeError(w, http.StatusForbidden, "only agents can assign tickets")
			return
		}
		agent, err := a.getUser(*in.AssignedTo)
		if err != nil || agent.Role != "agent" {
			writeError(w, http.StatusBadRequest, "assigned_to must be an agent")
			return
		}
		t.AssignedTo = in.AssignedTo
	}

	_, err := a.DB.Exec(`UPDATE tickets SET status = ?, priority = ?, assigned_to = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		t.Status, t.Priority, t.AssignedTo, t.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not update ticket")
		return
	}
	t, _ = scanTicket(a.DB.QueryRow(`SELECT `+ticketCols+` FROM tickets WHERE id = ?`, t.ID))
	writeJSON(w, http.StatusOK, t)
}

// DELETE /api/tickets/{id}  (agents only)
func (a *App) deleteTicket(w http.ResponseWriter, r *http.Request, u User) {
	t, ok := a.findTicket(w, r, u)
	if !ok {
		return
	}
	if u.Role != "agent" {
		writeError(w, http.StatusForbidden, "only agents can delete tickets")
		return
	}
	if _, err := a.DB.Exec(`DELETE FROM tickets WHERE id = ?`, t.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete ticket")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/tickets/{id}/comments
func (a *App) addComment(w http.ResponseWriter, r *http.Request, u User) {
	t, ok := a.findTicket(w, r, u)
	if !ok {
		return
	}
	var in struct {
		Body string `json:"body"`
	}
	if err := readJSON(r, &in); err != nil || strings.TrimSpace(in.Body) == "" {
		writeError(w, http.StatusBadRequest, "body is required")
		return
	}
	res, err := a.DB.Exec(`INSERT INTO comments (ticket_id, user_id, body) VALUES (?, ?, ?)`, t.ID, u.ID, in.Body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not add comment")
		return
	}
	id, _ := res.LastInsertId()
	var c Comment
	a.DB.QueryRow(`SELECT id, ticket_id, user_id, body, created_at FROM comments WHERE id = ?`, id).
		Scan(&c.ID, &c.TicketID, &c.UserID, &c.Body, &c.CreatedAt)
	writeJSON(w, http.StatusCreated, c)
}

// GET /api/tickets/{id}/comments
func (a *App) listComments(w http.ResponseWriter, r *http.Request, u User) {
	t, ok := a.findTicket(w, r, u)
	if !ok {
		return
	}
	rows, err := a.DB.Query(`SELECT id, ticket_id, user_id, body, created_at FROM comments WHERE ticket_id = ? ORDER BY id`, t.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()
	comments := []Comment{}
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.TicketID, &c.UserID, &c.Body, &c.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
		comments = append(comments, c)
	}
	writeJSON(w, http.StatusOK, comments)
}
