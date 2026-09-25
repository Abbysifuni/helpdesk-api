package main

import (
	"database/sql"
	"errors"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/mattn/go-sqlite3"
)

// ---------- register ----------

func (a *App) register(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))

	if in.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if _, err := mail.ParseAddress(in.Email); err != nil {
		writeError(w, http.StatusBadRequest, "a valid email is required")
		return
	}
	if len(in.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	hash, err := hashPassword(in.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not hash password")
		return
	}
	role := "customer"
	if a.AgentEmail != "" && strings.EqualFold(in.Email, a.AgentEmail) {
		role = "agent"
	}

	res, err := a.DB.Exec(`INSERT INTO users (name, email, password_hash, role) VALUES (?, ?, ?, ?)`,
		in.Name, in.Email, hash, role)
	var sqlErr sqlite3.Error
	if errors.As(err, &sqlErr) && sqlErr.ExtendedCode == sqlite3.ErrConstraintUnique {
		writeError(w, http.StatusConflict, "email already registered")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create user")
		return
	}
	id, _ := res.LastInsertId()
	u, _ := a.getUser(id)
	writeJSON(w, http.StatusCreated, u)
}

// ---------- login ----------

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	u, err := a.getUserByEmail(strings.ToLower(strings.TrimSpace(in.Email)))
	if err != nil || !checkPassword(in.Password, u.PasswordHash) {
		writeError(w, http.StatusUnauthorized, "wrong email or password")
		return
	}
	token, err := a.makeToken(u)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": u})
}

func (a *App) me(w http.ResponseWriter, r *http.Request, u User) {
	writeJSON(w, http.StatusOK, u)
}

// ---------- JWT ----------

func (a *App) makeToken(u User) (string, error) {
	claims := jwt.MapClaims{
		"sub":  strconv.FormatInt(u.ID, 10),
		"role": u.Role,
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.Secret)
}

// authedHandler is a handler that also receives the logged-in user.
type authedHandler func(w http.ResponseWriter, r *http.Request, u User)

// auth is middleware: it checks the Bearer token, loads the user,
// and only then calls the real handler.
func (a *App) auth(next authedHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			writeError(w, http.StatusUnauthorized, "missing token")
			return
		}
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
			return a.Secret, nil
		}, jwt.WithValidMethods([]string{"HS256"}))
		if err != nil || !token.Valid {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}
		sub, _ := token.Claims.GetSubject()
		id, _ := strconv.ParseInt(sub, 10, 64)
		u, err := a.getUser(id)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "user not found")
			return
		}
		next(w, r, u)
	})
}

// ---------- user queries ----------

const userCols = `id, name, email, role, password_hash, created_at`

func scanUser(row *sql.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.PasswordHash, &u.CreatedAt)
	return u, err
}

func (a *App) getUser(id int64) (User, error) {
	return scanUser(a.DB.QueryRow(`SELECT `+userCols+` FROM users WHERE id = ?`, id))
}

func (a *App) getUserByEmail(email string) (User, error) {
	return scanUser(a.DB.QueryRow(`SELECT `+userCols+` FROM users WHERE email = ?`, email))
}
