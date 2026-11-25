package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path/filepath"
	"testing"
	"time"
)

func purchaseTestApp(t *testing.T) (*App, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.sqlite")
	app, err := openApp(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.DB.Close() })
	return app, path
}

func purchaseTestEvent(t *testing.T, a *App, capacity int, status string) string {
	t.Helper()
	ctx := context.Background()
	id := randomID("evt_")
	err := a.write(ctx, func(conn *sql.Conn) error {
		_, err := conn.ExecContext(ctx, `INSERT INTO events
			(id,owner_id,name,starts_at,location,capacity,price_cents,status)
			VALUES (?,'usr_org_a','Jazz',?,'Porão',?,10000,?)`, id, time.Now().Add(48*time.Hour).Unix(), capacity, status)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestConcurrentReservations(t *testing.T) {
	app, _ := purchaseTestApp(t)
	for round := 0; round < 2; round++ {
		id := purchaseTestEvent(t, app, 5, "published")
		start := make(chan struct{})
		results := make(chan error, 30)
		for index := 0; index < 30; index++ {
			go func(index int) {
				<-start
				ticket, err := app.purchase(context.Background(), id, fmt.Sprintf("buyer%d@example.com", index), nil, "4000000000000001")
				if err == nil && (ticket.Status != "pending" || ticket.Code == "") {
					err = fmt.Errorf("unexpected ticket: %+v", ticket)
				}
				results <- err
			}(index)
		}
		close(start)
		accepted, soldOut := 0, 0
		for index := 0; index < 30; index++ {
			switch err := <-results; err {
			case nil:
				accepted++
			case errSoldOut:
				soldOut++
			default:
				t.Fatalf("round %d: %v", round+1, err)
			}
		}
		if accepted != 5 || soldOut != 25 {
			t.Fatalf("round %d: %d accepted, %d sold out", round+1, accepted, soldOut)
		}
		t.Logf("round %d: %d pending, %d sold out", round+1, accepted, soldOut)
	}
}

func TestPurchaseEventAndGateway(t *testing.T) {
	app, path := purchaseTestApp(t)
	ctx := context.Background()
	const email = "buyer@example.com"
	const fastApprove = "4000000000000001"
	draft := purchaseTestEvent(t, app, 2, "draft")
	for _, eventID := range []string{"missing", draft, purchaseTestEvent(t, app, 2, "cancelled")} {
		if _, err := app.purchase(ctx, eventID, email, nil, fastApprove); !errors.Is(err, errEventUnavailable) {
			t.Fatalf("unavailable event: %v", err)
		}
	}
	if _, err := app.purchase(ctx, draft, email, nil, "123"); !errors.Is(err, errPurchaseInvalid) {
		t.Fatalf("invalid card: %v", err)
	}
	id := purchaseTestEvent(t, app, 2, "published")
	first, err := app.purchase(ctx, id, email, nil, fastApprove)
	if err != nil || first.Status != "pending" || first.PriceCents != 10000 || first.Code == "" {
		t.Fatalf("first purchase: %+v, %v", first, err)
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: handlerTransport{app.routes()}, Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	const base = "http://catraca.test"
	token := pageToken(t, client, base+"/entrar")
	postAccount(t, client, base+"/entrar", url.Values{"email": {"org-a@example.com"}, "password": {demoPassword}, "csrf": {token}}, 303, "/organizador")
	target := base + "/organizador/eventos/" + id
	edit := url.Values{"name": {"Jazz"}, "location": {"Porão"}, "startsAt": {time.Now().In(eventZone).Add(48 * time.Hour).Format(eventDateLayout)}, "capacity": {"2"}, "price": {"150,00"}, "csrf": {pageToken(t, client, target)}}
	postAccount(t, client, target, edit, 303, "/organizador/eventos/"+id)
	second, err := app.purchase(ctx, id, email, nil, "4000000000000004")
	if err != nil || second.PriceCents != 15000 || second.Code == first.Code {
		t.Fatalf("second purchase: %+v, %v", second, err)
	}
	edit.Set("capacity", "1")
	postAccount(t, client, target, edit, 422, "")
	if _, err := app.purchase(ctx, id, email, nil, fastApprove); !errors.Is(err, errSoldOut) {
		t.Fatalf("full event: %v", err)
	}

	if err := app.resolvePayments(ctx, time.Now().Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	checkTicket(t, app, first.ID, "confirmed", 10000)
	checkTicket(t, app, second.ID, "pending", 15000)
	if err := app.DB.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := openApp(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.DB.Close()
	if err := reopened.resolvePayments(ctx, time.Now().Add(45*time.Second)); err != nil {
		t.Fatal(err)
	}
	checkTicket(t, reopened, second.ID, "declined", 15000)
	event, err := ownedEvent(ctx, reopened.DB, id, "usr_org_a")
	if err != nil || event.AvailableSeats() != 1 {
		t.Fatalf("availability after decline: %+v, %v", event, err)
	}
}

func checkTicket(t *testing.T, app *App, id, status string, price int64) {
	t.Helper()
	ticket, err := app.ticketByID(context.Background(), id)
	if err != nil || ticket.Status != status || ticket.PriceCents != price {
		t.Fatalf("ticket: %+v, %v; expected %s at %d cents", ticket, err, status, price)
	}
}

func TestPaymentSimulation(t *testing.T) {
	for _, card := range []struct {
		suffix   string
		approved bool
		delay    time.Duration
	}{
		{"0001", true, 3 * time.Second},
		{"0002", false, 3 * time.Second},
		{"0003", true, 35 * time.Second},
		{"0004", false, 35 * time.Second},
		{"9999", false, 3 * time.Second},
	} {
		approved, delay := payment("400000000000" + card.suffix)
		if approved != card.approved || delay != card.delay {
			t.Errorf("card ending %s: approved=%v delay=%s", card.suffix, approved, delay)
		}
	}
}
