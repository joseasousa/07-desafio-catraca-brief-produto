package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"html/template"
	"log"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const authSchema = `
CREATE TABLE IF NOT EXISTS users (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	email TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	role TEXT NOT NULL CHECK(role IN ('organizer','participant'))
);
CREATE TABLE IF NOT EXISTS sessions (
	token TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	expires_at INTEGER NOT NULL
);`

const demoPassword = "CatracaDemo123!"
const sessionCookie = "catraca_session"
const csrfCookie = "catraca_csrf"

type User struct {
	ID, Name, Email, Role string
}

func randomID(prefix string) string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		panic(err)
	}
	return prefix + hex.EncodeToString(value)
}

func seedUsers(ctx context.Context, conn *sql.Conn) error {
	accounts := []User{
		{"usr_org_a", "Organizador A", "org-a@example.com", "organizer"},
		{"usr_org_b", "Organizador B", "org-b@example.com", "organizer"},
		{"usr_participante", "Participante", "participante@example.com", "participant"},
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(demoPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	for _, user := range accounts {
		_, err := conn.ExecContext(ctx, `INSERT INTO users (id,name,email,password_hash,role)
			VALUES (?,?,?,?,?) ON CONFLICT(id) DO NOTHING`, user.ID, user.Name, user.Email, string(hash), user.Role)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *App) currentUser(r *http.Request) (User, bool) {
	var user User
	token, err := r.Cookie(sessionCookie)
	if err != nil {
		return user, false
	}
	err = a.DB.QueryRowContext(r.Context(), `SELECT users.id,users.name,users.email,users.role
		FROM sessions JOIN users ON users.id=sessions.user_id
		WHERE sessions.token=? AND expires_at>?`, token.Value, time.Now().Unix()).Scan(&user.ID, &user.Name, &user.Email, &user.Role)
	return user, err == nil
}

func (a *App) requireRole(w http.ResponseWriter, r *http.Request, role string) (User, bool) {
	user, ok := a.currentUser(r)
	if !ok {
		http.Redirect(w, r, "/entrar", http.StatusSeeOther)
		return User{}, false
	}
	if user.Role != role {
		http.Error(w, "Acesso negado", http.StatusForbidden)
		return User{}, false
	}
	return user, true
}

func (a *App) csrf(r *http.Request) string {
	token, err := r.Cookie(csrfCookie)
	if err != nil || len(token.Value) != 32 {
		return ""
	}
	return token.Value
}

func (a *App) validCSRF(r *http.Request) bool {
	token := a.csrf(r)
	return token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(r.PostFormValue("csrf"))) == 1
}

func setCookie(w http.ResponseWriter, r *http.Request, name, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: value, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode,
		Secure: r.TLS != nil, MaxAge: maxAge,
	})
}

func (a *App) registerAuth(mux *http.ServeMux) {
	mux.HandleFunc("GET /entrar", a.login)
	mux.HandleFunc("POST /entrar", a.login)
	mux.HandleFunc("GET /cadastro", a.register)
	mux.HandleFunc("POST /cadastro", a.register)
	mux.HandleFunc("POST /sair", a.logout)
}

const accountForm = `
<h1>{{.Data.Title}}</h1>
{{if .Data.Error}}<p role="alert">{{.Data.Error}}</p>{{end}}
<form method="post">
	<input type="hidden" name="csrf" value="{{.CSRF}}">
	{{if .Data.Register}}<label>Nome <input name="name" required></label>{{end}}
	<label>E-mail <input name="email" type="email" autocomplete="email" required></label>
	<label>Senha <input name="password" type="password" autocomplete="{{if .Data.Register}}new-password{{else}}current-password{{end}}" required></label>
	<button>{{.Data.Title}}</button>
</form>`

type accountPage struct {
	Title, Error string
	Register     bool
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	page := accountPage{Title: "Entrar"}
	if r.Method == http.MethodGet {
		a.render(w, r, http.StatusOK, page.Title, accountForm, page)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if !a.validCSRF(r) {
		http.Error(w, "Formulário inválido", http.StatusForbidden)
		return
	}
	var userID, role, hash string
	email := strings.ToLower(strings.TrimSpace(r.PostFormValue("email")))
	err := a.DB.QueryRowContext(r.Context(), "SELECT id,role,password_hash FROM users WHERE email=?", email).Scan(&userID, &role, &hash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Erro ao entrar", http.StatusInternalServerError)
		return
	}
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(r.PostFormValue("password"))) != nil {
		page.Error = "E-mail ou senha inválidos"
		a.render(w, r, http.StatusUnauthorized, page.Title, accountForm, page)
		return
	}
	a.startSession(w, r, userID, role)
}

func (a *App) register(w http.ResponseWriter, r *http.Request) {
	page := accountPage{Title: "Criar conta", Register: true}
	if r.Method == http.MethodGet {
		a.render(w, r, http.StatusOK, page.Title, accountForm, page)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if !a.validCSRF(r) {
		http.Error(w, "Formulário inválido", http.StatusForbidden)
		return
	}
	name := strings.TrimSpace(r.PostFormValue("name"))
	email := strings.ToLower(strings.TrimSpace(r.PostFormValue("email")))
	password := r.PostFormValue("password")
	address, err := mail.ParseAddress(email)
	if name == "" || err != nil || address.Address != email || len(password) == 0 || len(password) > 72 {
		page.Error = "Preencha nome, e-mail e senha válidos"
		a.render(w, r, http.StatusUnprocessableEntity, page.Title, accountForm, page)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Erro ao criar conta", http.StatusInternalServerError)
		return
	}
	id := randomID("usr_")
	result, err := a.DB.ExecContext(r.Context(), `INSERT INTO users (id,name,email,password_hash,role)
		VALUES (?,?,?,?,'participant') ON CONFLICT(email) DO NOTHING`, id, name, email, string(hash))
	if err != nil {
		http.Error(w, "Erro ao criar conta", http.StatusInternalServerError)
		return
	}
	if inserted, err := result.RowsAffected(); err != nil || inserted != 1 {
		page.Error = "E-mail já cadastrado"
		a.render(w, r, http.StatusConflict, page.Title, accountForm, page)
		return
	}
	a.startSession(w, r, id, "participant")
}

func (a *App) startSession(w http.ResponseWriter, r *http.Request, userID, role string) {
	const ttl = 24 * time.Hour
	token := randomID("")
	err := a.write(r.Context(), func(conn *sql.Conn) error {
		if old, err := r.Cookie(sessionCookie); err == nil {
			if _, err := conn.ExecContext(r.Context(), "DELETE FROM sessions WHERE token=?", old.Value); err != nil {
				return err
			}
		}
		_, err := conn.ExecContext(r.Context(), "INSERT INTO sessions VALUES (?,?,?)", token, userID, time.Now().Add(ttl).Unix())
		return err
	})
	if err != nil {
		http.Error(w, "Erro ao iniciar sessão", http.StatusInternalServerError)
		return
	}
	setCookie(w, r, sessionCookie, token, int(ttl.Seconds()))
	setCookie(w, r, csrfCookie, randomID(""), 0)
	target := "/"
	if role == "organizer" {
		target = "/organizador"
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if !a.validCSRF(r) {
		http.Error(w, "Formulário inválido", http.StatusForbidden)
		return
	}
	if token, err := r.Cookie(sessionCookie); err == nil {
		if _, err := a.DB.ExecContext(r.Context(), "DELETE FROM sessions WHERE token=?", token.Value); err != nil {
			http.Error(w, "Erro ao encerrar sessão", http.StatusInternalServerError)
			return
		}
	}
	setCookie(w, r, sessionCookie, "", -1)
	setCookie(w, r, csrfCookie, "", -1)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

const pageShell = `<!DOCTYPE html>
<html lang="pt-BR">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>{{.Title}} · Catraca</title>
<script defer src="/assets/htmx.min.js"></script>
<style>body{max-width:56rem;margin:2rem auto;padding:0 1rem;font:1rem system-ui,sans-serif}nav{display:flex;gap:1rem;align-items:center;flex-wrap:wrap}label{display:block;margin:.75rem 0}input,button{font:inherit}form.inline{display:inline}</style></head>
<body>
<nav aria-label="Principal"><a href="/">Vitrine</a>
{{if eq .User.Role "organizer"}}<a href="/organizador">Meus eventos</a>{{end}}
{{if eq .User.Role "participant"}}<a href="/meus-ingressos">Meus ingressos</a>{{end}}
{{if .User.ID}}<span>{{.User.Name}}</span><form class="inline" method="post" action="/sair"><input type="hidden" name="csrf" value="{{.CSRF}}"><button>Sair</button></form>
{{else}}<a href="/entrar">Entrar</a><a href="/cadastro">Criar conta</a>{{end}}</nav>
<main>{{template "content" .}}</main>
</body></html>`

func (a *App) render(w http.ResponseWriter, r *http.Request, status int, title, body string, data any) {
	token := a.csrf(r)
	if token == "" {
		token = randomID("")
		setCookie(w, r, csrfCookie, token, 0)
	}
	user, _ := a.currentUser(r)
	view := struct {
		Title string
		CSRF  string
		User  User
		Data  any
	}{title, token, user, data}
	tmpl, err := template.New("page").Parse(pageShell + `{{define "content"}}` + body + `{{end}}`)
	var rendered bytes.Buffer
	if err == nil {
		err = tmpl.Execute(&rendered, view)
	}
	if err != nil {
		log.Printf("render %q: %v", title, err)
		http.Error(w, "Erro ao exibir página", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(rendered.Bytes())
}
