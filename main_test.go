package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestServer starts the API with a fresh in-memory database.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	db, err := openDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	app := &App{DB: db, Secret: []byte("test-secret"), AgentEmail: "agent@techtz.com"}
	srv := httptest.NewServer(app.routes())
	t.Cleanup(func() { srv.Close(); db.Close() })
	return srv
}

// call sends a JSON request and decodes the JSON response.
func call(t *testing.T, srv *httptest.Server, method, path, token string, body any) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, srv.URL+path, &buf)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

// signup registers a user and returns a login token.
func signup(t *testing.T, srv *httptest.Server, name, email string) string {
	t.Helper()
	call(t, srv, "POST", "/api/register", "", map[string]string{"name": name, "email": email, "password": "secret123"})
	code, out := call(t, srv, "POST", "/api/login", "", map[string]string{"email": email, "password": "secret123"})
	if code != http.StatusOK {
		t.Fatalf("login %s: got %d", email, code)
	}
	return out["token"].(string)
}

func TestRegisterValidation(t *testing.T) {
	srv := newTestServer(t)
	tests := []struct {
		name string
		body map[string]string
		want int
	}{
		{"ok", map[string]string{"name": "Amina", "email": "amina@techtz.com", "password": "secret123"}, 201},
		{"duplicate email", map[string]string{"name": "Amina", "email": "amina@techtz.com", "password": "secret123"}, 409},
		{"missing name", map[string]string{"email": "x@techtz.com", "password": "secret123"}, 400},
		{"bad email", map[string]string{"name": "X", "email": "not-an-email", "password": "secret123"}, 400},
		{"short password", map[string]string{"name": "X", "email": "y@techtz.com", "password": "123"}, 400},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, _ := call(t, srv, "POST", "/api/register", "", tc.body)
			if code != tc.want {
				t.Errorf("got %d, want %d", code, tc.want)
			}
		})
	}
}

func TestLoginAndAuth(t *testing.T) {
	srv := newTestServer(t)
	token := signup(t, srv, "Juma", "juma@techtz.com")

	if code, _ := call(t, srv, "POST", "/api/login", "", map[string]string{"email": "juma@techtz.com", "password": "wrong-pass"}); code != 401 {
		t.Errorf("wrong password: got %d, want 401", code)
	}
	if code, _ := call(t, srv, "GET", "/api/me", "", nil); code != 401 {
		t.Errorf("no token: got %d, want 401", code)
	}
	if code, _ := call(t, srv, "GET", "/api/me", "not-a-real-token", nil); code != 401 {
		t.Errorf("bad token: got %d, want 401", code)
	}
	code, me := call(t, srv, "GET", "/api/me", token, nil)
	if code != 200 || me["email"] != "juma@techtz.com" || me["role"] != "customer" {
		t.Errorf("me: got %d %v", code, me)
	}
	if _, leaked := me["password_hash"]; leaked {
		t.Error("password hash must never be returned")
	}
}

func TestTicketFlow(t *testing.T) {
	srv := newTestServer(t)
	amina := signup(t, srv, "Amina", "amina@techtz.com")
	juma := signup(t, srv, "Juma", "juma@techtz.com")
	agent := signup(t, srv, "Support", "agent@techtz.com")

	// customer creates a ticket
	code, tk := call(t, srv, "POST", "/api/tickets", amina, map[string]string{"title": "Cannot log in", "priority": "high"})
	if code != 201 || tk["status"] != "open" || tk["priority"] != "high" {
		t.Fatalf("create: got %d %v", code, tk)
	}
	path := "/api/tickets/1"

	if code, _ := call(t, srv, "POST", "/api/tickets", amina, map[string]string{"title": ""}); code != 400 {
		t.Errorf("empty title: got %d, want 400", code)
	}
	if code, _ := call(t, srv, "GET", path, juma, nil); code != 404 {
		t.Errorf("other customer: got %d, want 404", code)
	}
	if code, _ := call(t, srv, "GET", path, agent, nil); code != 200 {
		t.Errorf("agent view: got %d, want 200", code)
	}
	if code, _ := call(t, srv, "PATCH", path, amina, map[string]string{"priority": "low"}); code != 403 {
		t.Errorf("customer priority: got %d, want 403", code)
	}
	if code, _ := call(t, srv, "DELETE", path, amina, nil); code != 403 {
		t.Errorf("customer delete: got %d, want 403", code)
	}

	// agent takes the ticket
	code, tk = call(t, srv, "PATCH", path, agent, map[string]any{"status": "in_progress", "assigned_to": 3})
	if code != 200 || tk["status"] != "in_progress" || tk["assigned_to"].(float64) != 3 {
		t.Errorf("agent update: got %d %v", code, tk)
	}

	// comments
	if code, _ := call(t, srv, "POST", path+"/comments", agent, map[string]string{"body": "Try resetting your password"}); code != 201 {
		t.Errorf("comment: got %d, want 201", code)
	}
	if code, _ := call(t, srv, "POST", path+"/comments", juma, map[string]string{"body": "hi"}); code != 404 {
		t.Errorf("comment on other's ticket: got %d, want 404", code)
	}

	// customer closes it
	if code, tk := call(t, srv, "PATCH", path, amina, map[string]string{"status": "closed"}); code != 200 || tk["status"] != "closed" {
		t.Errorf("customer close: got %d %v", code, tk)
	}

	// agent deletes it
	if code, _ := call(t, srv, "DELETE", path, agent, nil); code != 204 {
		t.Errorf("agent delete: got %d, want 204", code)
	}
	if code, _ := call(t, srv, "GET", path, agent, nil); code != 404 {
		t.Errorf("after delete: got %d, want 404", code)
	}
}

func TestListFilters(t *testing.T) {
	srv := newTestServer(t)
	amina := signup(t, srv, "Amina", "amina@techtz.com")
	juma := signup(t, srv, "Juma", "juma@techtz.com")
	agent := signup(t, srv, "Support", "agent@techtz.com")

	call(t, srv, "POST", "/api/tickets", amina, map[string]string{"title": "A", "priority": "high"})
	call(t, srv, "POST", "/api/tickets", amina, map[string]string{"title": "B", "priority": "low"})
	call(t, srv, "POST", "/api/tickets", juma, map[string]string{"title": "C", "priority": "high"})

	count := func(token, query string) int {
		req, _ := http.NewRequest("GET", srv.URL+"/api/tickets"+query, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var list []Ticket
		json.NewDecoder(res.Body).Decode(&list)
		return len(list)
	}
	checks := []struct {
		who, token, query string
		want              int
	}{
		{"amina all", amina, "", 2},
		{"juma all", juma, "", 1},
		{"agent all", agent, "", 3},
		{"agent high", agent, "?priority=high", 2},
		{"amina open+low", amina, "?status=open&priority=low", 1},
	}
	for _, c := range checks {
		if got := count(c.token, c.query); got != c.want {
			t.Errorf("%s: got %d tickets, want %d", c.who, got, c.want)
		}
	}
}
