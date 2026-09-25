# Helpdesk API (Go + SQLite + JWT)

A small, real-world ticket system API built step by step in the
**TechTZ Academy** YouTube series *“Build a Helpdesk API with Go”*.

▶ **Watch the 8-part series:** https://www.youtube.com/playlist?list=PLN0_voBC3mdc

- Customers register, log in and open support tickets
- Support agents see every ticket, change status and priority, assign tickets and delete them
- Everyone can comment on the tickets they can see
- Data is stored in **SQLite**, passwords are hashed with **PBKDF2-SHA256**, logins use **JWT**

## Requirements

- Go 1.24 or newer
- A C compiler for the SQLite driver (cgo): on Ubuntu/WSL run `sudo apt install build-essential`

## Run it

```bash
git clone https://github.com/Abbysifuni/helpdesk-api.git && cd helpdesk-api
go mod download
AGENT_EMAIL=agent@techtz.com JWT_SECRET=change-me go run .
# helpdesk API listening on :8080
```

| Variable      | Default                    | Meaning                                     |
|---------------|----------------------------|---------------------------------------------|
| `ADDR`        | `:8080`                    | Address to listen on                        |
| `DB_PATH`     | `helpdesk.db`              | SQLite database file                        |
| `JWT_SECRET`  | `change-me-in-production`  | Secret used to sign tokens — **change it!** |
| `AGENT_EMAIL` | *(empty)*                  | The user who registers with this email becomes a support agent |

## Endpoints

| Method | Path                              | Auth | Who            | What it does |
|--------|-----------------------------------|------|----------------|--------------|
| GET    | `/health`                         | –    | anyone         | Health check |
| POST   | `/api/register`                   | –    | anyone         | Create an account `{name, email, password}` |
| POST   | `/api/login`                      | –    | anyone         | Get a token `{email, password}` → `{token, user}` |
| GET    | `/api/me`                         | ✔    | any user       | The logged-in user |
| POST   | `/api/tickets`                    | ✔    | any user       | Open a ticket `{title, description, priority}` |
| GET    | `/api/tickets`                    | ✔    | any user       | List tickets (customers: own; agents: all). Filters: `?status=` `?priority=` |
| GET    | `/api/tickets/{id}`               | ✔    | owner / agent  | One ticket |
| PATCH  | `/api/tickets/{id}`               | ✔    | owner / agent  | Update `{status, priority, assigned_to}` (customers may only close) |
| DELETE | `/api/tickets/{id}`               | ✔    | agent          | Delete a ticket |
| POST   | `/api/tickets/{id}/comments`      | ✔    | owner / agent  | Add a comment `{body}` |
| GET    | `/api/tickets/{id}/comments`      | ✔    | owner / agent  | List comments |

- **status**: `open`, `in_progress`, `resolved`, `closed`
- **priority**: `low`, `medium`, `high`
- Errors look like `{"error": "message"}` with the right HTTP status (400, 401, 403, 404, 409).

## Try it with curl

```bash
# 1. register and log in
curl -X POST localhost:8080/api/register -d '{"name":"Amina","email":"amina@techtz.com","password":"secret123"}'
TOKEN=$(curl -s -X POST localhost:8080/api/login \
  -d '{"email":"amina@techtz.com","password":"secret123"}' | jq -r .token)

# 2. open a ticket
curl -X POST localhost:8080/api/tickets -H "Authorization: Bearer $TOKEN" \
  -d '{"title":"Cannot log in","description":"Reset email never arrives","priority":"high"}'

# 3. list my tickets
curl localhost:8080/api/tickets -H "Authorization: Bearer $TOKEN"

# 4. as the agent: take the ticket
curl -X PATCH localhost:8080/api/tickets/1 -H "Authorization: Bearer $AGENT_TOKEN" \
  -d '{"status":"in_progress","assigned_to":2}'

# 5. comment
curl -X POST localhost:8080/api/tickets/1/comments -H "Authorization: Bearer $TOKEN" \
  -d '{"body":"Thanks for the help!"}'
```

## Tests

```bash
go test -v ./...
```

Tests use an in-memory SQLite database and cover validation, login, JWT,
permissions (customer vs agent), the full ticket flow and list filters.

## Docker

```bash
docker build -t helpdesk .
docker run -p 8080:8080 -v helpdesk-data:/data \
  -e JWT_SECRET=change-me -e AGENT_EMAIL=agent@techtz.com helpdesk
```

## Project layout

```
main.go        start-up: config, database, server
app.go         App struct, routes, logging middleware
db.go          SQLite connection and tables
models.go      User, Ticket, Comment
respond.go     JSON helpers
password.go    password hashing (PBKDF2-SHA256)
auth.go        register, login, JWT and the auth middleware
tickets.go     tickets and comments
main_test.go   tests
Dockerfile     container build
```

## Ideas to extend it

- Pagination for `GET /api/tickets`
- Email notifications when a ticket changes
- A Vue or React front end (see our *Learn Go in 30 Days*, Day 29)
- Switch to PostgreSQL for large teams

License: MIT — free to learn from and use.
