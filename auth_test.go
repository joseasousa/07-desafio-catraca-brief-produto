package main

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestAccountsAndSeed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.sqlite")
	app, err := openApp(path)
	if err != nil {
		t.Fatal(err)
	}
	app.DB.Close()
	app, err = openApp(path)
	if err != nil {
		t.Fatal(err)
	}
	defer app.DB.Close()

	var count int
	if err := app.DB.QueryRow("SELECT count(*) FROM users").Scan(&count); err != nil || count != 3 {
		t.Fatalf("seed after restart: count=%d err=%v", count, err)
	}
	const baseURL = "http://catraca.test"
	handler := app.routes()
	for _, account := range []struct{ Email, Password, Target string }{
		{"org-a@example.com", demoPassword, "/organizador"},
		{"org-b@example.com", demoPassword, "/organizador"},
		{"participante@example.com", demoPassword, "/"},
		{"nova@example.com", "senha válida", "/"},
	} {
		jar, err := cookiejar.New(nil)
		if err != nil {
			t.Fatal(err)
		}
		client := &http.Client{Transport: handlerTransport{handler}, Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}}
		route := "/entrar"
		if account.Email == "nova@example.com" {
			route = "/cadastro"
		}
		token := pageToken(t, client, baseURL+route)
		form := url.Values{"name": {"Nova"}, "email": {account.Email}, "password": {account.Password}}
		postAccount(t, client, baseURL+route, form, http.StatusForbidden, "")
		form.Set("csrf", token)
		postAccount(t, client, baseURL+route, form, http.StatusSeeOther, account.Target)
		token = pageToken(t, client, baseURL+"/")
		postAccount(t, client, baseURL+"/sair", url.Values{"csrf": {token}}, http.StatusSeeOther, "/")
	}
	var role string
	if err := app.DB.QueryRow("SELECT role FROM users WHERE email='nova@example.com'").Scan(&role); err != nil || role != "participant" {
		t.Fatalf("registration: role=%q err=%v", role, err)
	}
	if err := app.DB.QueryRow("SELECT count(*) FROM sessions").Scan(&count); err != nil || count != 0 {
		t.Fatalf("after logout: count=%d err=%v", count, err)
	}
}

type handlerTransport struct {
	handler http.Handler
}

func (transport handlerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response := httptest.NewRecorder()
	transport.handler.ServeHTTP(response, request)
	return response.Result(), nil
}

var csrfField = regexp.MustCompile(`name="csrf" value="([a-f0-9]+)"`)

func pageToken(t *testing.T, client *http.Client, target string) string {
	t.Helper()
	response, err := client.Get(target)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	page, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	match := csrfField.FindSubmatch(page)
	if response.StatusCode != http.StatusOK || len(match) != 2 {
		t.Fatalf("GET %s: status=%d missing CSRF field", target, response.StatusCode)
	}
	return string(match[1])
}

func postAccount(t *testing.T, client *http.Client, target string, values url.Values, status int, location string) {
	t.Helper()
	response, err := client.PostForm(target, values)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != status || !strings.EqualFold(response.Header.Get("Location"), location) {
		t.Fatalf("POST %s: status=%d location=%q", target, response.StatusCode, response.Header.Get("Location"))
	}
}
