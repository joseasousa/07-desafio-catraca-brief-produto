package main

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "modernc.org/sqlite"
)

type App struct {
	DB *sql.DB
}

var routeRegistrars []func(*App, *http.ServeMux)
var startupHooks []func(context.Context, *App)
var migrations []string
var urlRoot = url.URL{Path: "/"}

//go:embed assets/htmx.min.js
var htmxJS []byte

func openApp(path string) (*App, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	app := &App{DB: db}
	if err := app.initialize(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return app, nil
}

func (a *App) write(ctx context.Context, operation func(*sql.Conn) error) error {
	conn, err := a.DB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		}
	}()
	if err := operation(conn); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

func (a *App) initialize(ctx context.Context) error {
	return a.write(ctx, func(conn *sql.Conn) error {
		if _, err := conn.ExecContext(ctx, authSchema); err != nil {
			return err
		}
		for _, migration := range migrations {
			if _, err := conn.ExecContext(ctx, migration); err != nil {
				return err
			}
		}
		return seedUsers(ctx, conn)
	})
}

func (a *App) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		if err := a.DB.PingContext(r.Context()); err != nil {
			http.Error(w, "Banco indisponível", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /assets/htmx.min.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = w.Write(htmxJS)
	})
	a.registerAuth(mux)
	for _, register := range routeRegistrars {
		register(a, mux)
	}
	if handler, pattern := mux.Handler(&http.Request{Method: "GET", URL: &urlRoot}); pattern == "" || handler == nil {
		mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
			a.render(w, r, http.StatusOK, "Catraca", "<h1>Catraca</h1>", nil)
		})
	}
	organizerURL := url.URL{Path: "/organizador"}
	if _, pattern := mux.Handler(&http.Request{Method: "GET", URL: &organizerURL}); pattern == "" {
		mux.HandleFunc("GET /organizador", func(w http.ResponseWriter, r *http.Request) {
			if _, ok := a.requireRole(w, r, "organizer"); !ok {
				return
			}
			a.render(w, r, http.StatusOK, "Meus eventos", "<h1>Meus eventos</h1>", nil)
		})
	}
	return mux
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	path := os.Getenv("CATRACA_DB")
	if path == "" {
		path = "catraca.sqlite"
	}
	app, err := openApp(path)
	if err != nil {
		log.Fatal(err)
	}
	defer app.DB.Close()
	for _, start := range startupHooks {
		start(ctx, app)
	}
	server := &http.Server{Addr: ":8080", Handler: app.routes(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Print("Catraca em :8080")
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
