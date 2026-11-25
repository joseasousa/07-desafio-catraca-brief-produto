package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

const eventSchema = `
CREATE TABLE IF NOT EXISTS events (
	id TEXT PRIMARY KEY,
	owner_id TEXT NOT NULL REFERENCES users(id),
	name TEXT NOT NULL,
	starts_at INTEGER NOT NULL,
	location TEXT NOT NULL,
	capacity INTEGER NOT NULL CHECK(capacity >= 1),
	price_cents INTEGER NOT NULL CHECK(price_cents > 0),
	status TEXT NOT NULL CHECK(status IN ('draft','published','cancelled'))
);
CREATE TABLE IF NOT EXISTS tickets (
	id TEXT PRIMARY KEY,
	event_id TEXT NOT NULL REFERENCES events(id),
	channel TEXT NOT NULL CHECK(channel IN ('web','partner')),
	participant_id TEXT REFERENCES users(id),
	buyer_email TEXT NOT NULL,
	price_cents INTEGER NOT NULL CHECK(price_cents > 0),
	status TEXT NOT NULL CHECK(status IN ('pending','confirmed','declined','cancelled','refunded')),
	code TEXT NOT NULL UNIQUE,
	checked_in INTEGER NOT NULL DEFAULT 0 CHECK(checked_in IN (0,1)),
	gateway_approved INTEGER NOT NULL CHECK(gateway_approved IN (0,1)),
	gateway_due_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS tickets_event_status ON tickets(event_id, status);`

const eventColumns = `e.id,e.name,e.starts_at,e.location,e.capacity,e.price_cents,e.status,
	(SELECT COUNT(*) FROM tickets t WHERE t.event_id=e.id AND t.status IN ('pending','confirmed'))`

const eventDateLayout = "2006-01-02T15:04"

var (
	eventZone        *time.Location
	priceFormat      = regexp.MustCompile(`^[0-9]+(?:[,.][0-9]{1,2})?$`)
	errEventInvalid  = errors.New("Preencha os dados do evento com início no futuro, lotação inteira e preço positivo")
	errEventCapacity = errors.New("Lotação menor que as vagas ocupadas")
	errEventLocked   = errors.New("O evento não permite essa alteração")
)

type Event struct {
	ID, Name, Location, Status     string
	StartsAt                       int64
	Capacity, PriceCents, Occupied int64
}

func (e Event) AvailableSeats() int64 {
	if e.Status == "cancelled" {
		return 0
	}
	return e.Capacity - e.Occupied
}

func (e Event) StartInput() string {
	if e.StartsAt == 0 {
		return ""
	}
	return time.Unix(e.StartsAt, 0).In(eventZone).Format(eventDateLayout)
}

func (e Event) Price() string {
	if e.PriceCents == 0 {
		return ""
	}
	return fmt.Sprintf("%d,%02d", e.PriceCents/100, e.PriceCents%100)
}

func scanEvent(row interface{ Scan(...any) error }) (Event, error) {
	var event Event
	err := row.Scan(&event.ID, &event.Name, &event.StartsAt, &event.Location, &event.Capacity, &event.PriceCents, &event.Status, &event.Occupied)
	return event, err
}

type eventReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func ownedEvent(ctx context.Context, db eventReader, eventID, ownerID string) (Event, error) {
	return scanEvent(db.QueryRowContext(ctx, "SELECT "+eventColumns+" FROM events e WHERE e.id=? AND e.owner_id=?", eventID, ownerID))
}

func init() {
	zone, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		panic(err)
	}
	eventZone = zone
	migrations = append(migrations, eventSchema)
	routeRegistrars = append(routeRegistrars, (*App).registerEvents)
}

func (a *App) registerEvents(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", a.eventList)
	mux.HandleFunc("GET /organizador", a.eventList)
	mux.HandleFunc("GET /organizador/eventos/novo", a.eventForm)
	mux.HandleFunc("POST /organizador/eventos/novo", a.eventForm)
	mux.HandleFunc("GET /organizador/eventos/{id}", a.eventForm)
	mux.HandleFunc("POST /organizador/eventos/{id}", a.eventForm)
	mux.HandleFunc("POST /organizador/eventos/{id}/publicar", a.publishEvent)
}

const eventListBody = `
<h1>{{if .Data.Manage}}Meus eventos{{else}}Vitrine{{end}}</h1>
{{if .Data.Manage}}<a href="/organizador/eventos/novo">Criar evento</a>{{end}}
{{range .Data.Events}}<article>
<h2>{{if $.Data.Manage}}<a href="/organizador/eventos/{{.ID}}">{{.Name}}</a>{{else}}{{.Name}}{{end}}</h2>
<p>{{.Location}} · {{.StartInput}} · R$ {{.Price}}</p>
{{if not $.Data.Manage}}
<p>Vagas disponíveis: {{.AvailableSeats}}</p>
{{if le .AvailableSeats 0}}<p>Ingressos esgotados</p>
{{else if eq $.User.Role "participant"}}
<form method="post" action="/eventos/{{.ID}}/comprar">
<input type="hidden" name="csrf" value="{{$.CSRF}}">
<label>Cartão <input name="cardNumber" inputmode="numeric" autocomplete="off" pattern="[0-9]{16}" maxlength="16" required></label>
<button>Comprar ingresso</button>
</form>
{{else if not $.User.ID}}<p><a href="/entrar">Entre para comprar</a></p>{{end}}
{{end}}
</article>{{end}}`

func (a *App) eventList(w http.ResponseWriter, r *http.Request) {
	page := struct {
		Manage bool
		Events []Event
	}{Manage: r.URL.Path == "/organizador"}
	query := "SELECT " + eventColumns + " FROM events e WHERE e.status='published'"
	var args []any
	if page.Manage {
		user, ok := a.requireRole(w, r, "organizer")
		if !ok {
			return
		}
		query = "SELECT " + eventColumns + " FROM events e WHERE e.owner_id=?"
		args = append(args, user.ID)
	}
	rows, err := a.DB.QueryContext(r.Context(), query+" ORDER BY e.starts_at,e.id", args...)
	if err != nil {
		eventFailure(w, err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			eventFailure(w, err)
			return
		}
		page.Events = append(page.Events, event)
	}
	if err := rows.Err(); err != nil {
		eventFailure(w, err)
		return
	}
	a.render(w, r, http.StatusOK, "Eventos", eventListBody, page)
}

const eventFormBody = `
<h1>{{if .Data.Event.ID}}{{.Data.Event.Name}}{{else}}Criar evento{{end}}</h1>
{{if .Data.Error}}<p role="alert">{{.Data.Error}}</p>{{end}}
{{if ne .Data.Event.Status "cancelled"}}
<form method="post" action="{{if .Data.Event.ID}}/organizador/eventos/{{.Data.Event.ID}}{{else}}/organizador/eventos/novo{{end}}">
<input type="hidden" name="csrf" value="{{.CSRF}}">
<label>Nome <input name="name" value="{{.Data.Event.Name}}" required></label>
<label>Data e hora de início <input name="startsAt" type="datetime-local" value="{{.Data.Event.StartInput}}" required></label>
<label>Local <input name="location" value="{{.Data.Event.Location}}" required></label>
<label>Lotação <input name="capacity" type="number" min="1" step="1" value="{{if .Data.Event.Capacity}}{{.Data.Event.Capacity}}{{end}}" required></label>
<label>Preço em reais <input name="price" inputmode="decimal" value="{{.Data.Event.Price}}" required></label>
<button>{{if .Data.Event.ID}}Salvar alterações{{else}}Salvar rascunho{{end}}</button>
</form>
{{if eq .Data.Event.Status "draft"}}
<form method="post" action="/organizador/eventos/{{.Data.Event.ID}}/publicar">
<input type="hidden" name="csrf" value="{{.CSRF}}"><button>Publicar</button></form>
{{end}}
{{end}}
{{if .Data.Event.ID}}<p>{{if eq .Data.Event.Status "draft"}}Rascunho{{else if eq .Data.Event.Status "published"}}Publicado{{else}}Evento cancelado{{end}}</p>
{{if .Data.Message}}<p role="status">{{.Data.Message}}</p>{{end}}
{{if eq .Data.Event.Status "published"}}
<form method="post" action="/organizador/eventos/{{.Data.Event.ID}}/check-in">
<input type="hidden" name="csrf" value="{{.CSRF}}">
<label>Código do ingresso <input name="code" required></label><button>Fazer check-in</button>
</form>
<form method="post" action="/organizador/eventos/{{.Data.Event.ID}}/cancelar">
<input type="hidden" name="csrf" value="{{.CSRF}}"><button>Cancelar evento</button>
</form>
{{end}}
<section aria-labelledby="painel"><h2 id="painel">Painel</h2><dl>
<div><dt>Confirmados</dt><dd>{{.Data.Panel.Confirmed}}</dd></div>
<div><dt>Pendentes</dt><dd>{{.Data.Panel.Pending}}</dd></div>
<div><dt>Check-ins</dt><dd>{{.Data.Panel.CheckIns}}</dd></div>
<div><dt>Vagas disponíveis</dt><dd>{{.Data.Panel.Seats}}</dd></div>
<div><dt>Receita</dt><dd>R$ {{.Data.Panel.RevenueLabel}}</dd></div>
<div><dt>Estornados</dt><dd>{{.Data.Panel.Refunded}}</dd></div>
</dl></section>
{{end}}`

type eventPage struct {
	Event          Event
	Panel          eventPanel
	Error, Message string
}

func (a *App) renderEvent(w http.ResponseWriter, r *http.Request, status int, page eventPage) {
	if page.Event.ID != "" {
		panel, err := a.panel(r.Context(), page.Event.ID)
		if err != nil {
			eventFailure(w, err)
			return
		}
		page.Panel = panel
	}
	a.render(w, r, status, "Evento", eventFormBody, page)
}

func parseEvent(r *http.Request, event Event) (Event, error) {
	event.Name = strings.TrimSpace(r.PostFormValue("name"))
	event.Location = strings.TrimSpace(r.PostFormValue("location"))
	start, startErr := time.ParseInLocation(eventDateLayout, r.PostFormValue("startsAt"), eventZone)
	capacity, capErr := strconv.ParseInt(r.PostFormValue("capacity"), 10, 64)
	price := strings.TrimSpace(r.PostFormValue("price"))
	event.StartsAt, event.Capacity = start.Unix(), capacity
	if startErr != nil {
		event.StartsAt = 0
	}
	if !priceFormat.MatchString(price) {
		return event, errEventInvalid
	}
	whole, fraction, _ := strings.Cut(strings.ReplaceAll(price, ",", "."), ".")
	cents, priceErr := strconv.ParseInt(whole+(fraction + "00")[:2], 10, 64)
	event.PriceCents = cents
	if event.Name == "" || event.Location == "" || startErr != nil || !start.After(time.Now()) || capErr != nil || capacity < 1 || priceErr != nil || cents < 1 {
		return event, errEventInvalid
	}
	return event, nil
}

func (a *App) eventForm(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireRole(w, r, "organizer")
	if !ok {
		return
	}
	ctx, id := r.Context(), r.PathValue("id")
	page := eventPage{}
	if id != "" {
		event, err := ownedEvent(ctx, a.DB, id, user.ID)
		if err != nil {
			eventFailure(w, err)
			return
		}
		page.Event = event
	}
	if r.Method == http.MethodGet {
		a.renderEvent(w, r, http.StatusOK, page)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if !a.validCSRF(r) {
		http.Error(w, "Formulário inválido", http.StatusForbidden)
		return
	}
	event, err := parseEvent(r, page.Event)
	if err == nil {
		err = a.write(ctx, func(conn *sql.Conn) error {
			if !time.Unix(event.StartsAt, 0).After(time.Now()) {
				return errEventInvalid
			}
			if id == "" {
				id = randomID("evt_")
				_, err := conn.ExecContext(ctx, `INSERT INTO events (id,owner_id,name,starts_at,location,capacity,price_cents,status) VALUES (?,?,?,?,?,?,?,'draft')`,
					id, user.ID, event.Name, event.StartsAt, event.Location, event.Capacity, event.PriceCents)
				return err
			}
			current, err := ownedEvent(ctx, conn, id, user.ID)
			if err != nil {
				return err
			}
			if current.Status == "cancelled" {
				return errEventLocked
			}
			if event.Capacity < current.Occupied {
				return errEventCapacity
			}
			_, err = conn.ExecContext(ctx, `UPDATE events SET name=?,starts_at=?,location=?,capacity=?,price_cents=? WHERE id=?`,
				event.Name, event.StartsAt, event.Location, event.Capacity, event.PriceCents, id)
			return err
		})
	}
	switch err {
	case nil:
		http.Redirect(w, r, "/organizador/eventos/"+id, http.StatusSeeOther)
	case errEventInvalid, errEventCapacity:
		page.Event, page.Error = event, err.Error()
		a.renderEvent(w, r, http.StatusUnprocessableEntity, page)
	default:
		eventFailure(w, err)
	}
}

func (a *App) publishEvent(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireRole(w, r, "organizer")
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if !a.validCSRF(r) {
		http.Error(w, "Formulário inválido", http.StatusForbidden)
		return
	}
	ctx, id := r.Context(), r.PathValue("id")
	err := a.write(ctx, func(conn *sql.Conn) error {
		event, err := ownedEvent(ctx, conn, id, user.ID)
		if err != nil {
			return err
		}
		if event.Status != "draft" {
			return errEventLocked
		}
		_, err = conn.ExecContext(ctx, "UPDATE events SET status='published' WHERE id=?", id)
		return err
	})
	if err != nil {
		eventFailure(w, err)
		return
	}
	http.Redirect(w, r, "/organizador/eventos/"+id, http.StatusSeeOther)
}

func eventFailure(w http.ResponseWriter, err error) {
	switch err {
	case sql.ErrNoRows:
		http.Error(w, "Evento não encontrado", http.StatusNotFound)
	case errEventLocked:
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		http.Error(w, "Erro ao acessar evento", http.StatusInternalServerError)
	}
}
