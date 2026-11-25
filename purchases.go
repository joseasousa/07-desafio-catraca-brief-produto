package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/mail"
	"regexp"
	"time"
)

var (
	errPurchaseInvalid  = errors.New("invalid_request")
	errEventUnavailable = errors.New("event_not_available")
	errSoldOut          = errors.New("sold_out")
	cardFormat          = regexp.MustCompile(`^[0-9]{16}$`)
)

type Ticket struct {
	ID, EventID, EventName, Code, Status string
	PriceCents                           int64
	CheckedIn                            bool
}

const ticketColumns = `t.id,t.event_id,e.name,t.code,t.status,t.price_cents,t.checked_in`

func scanTicket(row interface{ Scan(...any) error }) (Ticket, error) {
	var ticket Ticket
	err := row.Scan(&ticket.ID, &ticket.EventID, &ticket.EventName, &ticket.Code, &ticket.Status, &ticket.PriceCents, &ticket.CheckedIn)
	return ticket, err
}

func payment(card string) (bool, time.Duration) {
	switch card[12:] {
	case "0001":
		return true, 3 * time.Second
	case "0003":
		return true, 35 * time.Second
	case "0004":
		return false, 35 * time.Second
	default:
		return false, 3 * time.Second
	}
}

func (a *App) purchase(ctx context.Context, eventID, email string, participantID *string, card string) (Ticket, error) {
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || !cardFormat.MatchString(card) {
		return Ticket{}, errPurchaseInvalid
	}
	approved, delay := payment(card)
	ticket := Ticket{ID: randomID("tkt_"), EventID: eventID, Code: randomID(""), Status: "pending"}
	channel := "partner"
	if participantID != nil {
		channel = "web"
	}
	err = a.write(ctx, func(conn *sql.Conn) error {
		event, err := scanEvent(conn.QueryRowContext(ctx, "SELECT "+eventColumns+" FROM events e WHERE e.id=? AND e.status='published'", eventID))
		if errors.Is(err, sql.ErrNoRows) {
			return errEventUnavailable
		}
		if err != nil {
			return err
		}
		if event.AvailableSeats() <= 0 {
			return errSoldOut
		}
		ticket.EventName, ticket.PriceCents = event.Name, event.PriceCents
		_, err = conn.ExecContext(ctx, `INSERT INTO tickets
			(id,event_id,channel,participant_id,buyer_email,price_cents,status,code,gateway_approved,gateway_due_at)
			VALUES (?,?,?,?,?,?,?,?,?,?)`,
			ticket.ID, eventID, channel, participantID, email, ticket.PriceCents, ticket.Status, ticket.Code, approved, time.Now().Add(delay).UnixMilli())
		return err
	})
	if err != nil {
		return Ticket{}, err
	}
	return ticket, nil
}

func (a *App) ticketByID(ctx context.Context, id string) (Ticket, error) {
	return scanTicket(a.DB.QueryRowContext(ctx, "SELECT "+ticketColumns+" FROM tickets t JOIN events e ON e.id=t.event_id WHERE t.id=?", id))
}

func (a *App) participantTickets(ctx context.Context, participantID string) ([]Ticket, error) {
	rows, err := a.DB.QueryContext(ctx, "SELECT "+ticketColumns+" FROM tickets t JOIN events e ON e.id=t.event_id WHERE t.participant_id=? ORDER BY t.gateway_due_at DESC,t.id", participantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tickets []Ticket
	for rows.Next() {
		ticket, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, ticket)
	}
	return tickets, rows.Err()
}

func init() {
	startupHooks = append(startupHooks, func(ctx context.Context, a *App) {
		go a.runGateway(ctx)
	})
	migrations = append(migrations, `CREATE INDEX IF NOT EXISTS tickets_gateway_pending ON tickets(gateway_due_at) WHERE status='pending';`)
}

func (a *App) runGateway(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := a.resolvePayments(ctx, time.Now()); err != nil && ctx.Err() == nil {
			log.Printf("gateway: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *App) resolvePayments(ctx context.Context, now time.Time) error {
	const due = `status='pending' AND gateway_due_at<=? AND event_id IN (SELECT id FROM events WHERE status='published')`
	var exists bool
	if err := a.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM tickets WHERE `+due+`)`, now.UnixMilli()).Scan(&exists); err != nil || !exists {
		return err
	}
	return a.write(ctx, func(conn *sql.Conn) error {
		_, err := conn.ExecContext(ctx, `UPDATE tickets SET status=CASE gateway_approved WHEN 1 THEN 'confirmed' ELSE 'declined' END WHERE `+due, now.UnixMilli())
		return err
	})
}
