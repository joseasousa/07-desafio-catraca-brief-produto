package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type eventPanel struct {
	Confirmed, Pending, CheckIns, Seats, Revenue, Refunded int64
}

func (p eventPanel) RevenueLabel() string {
	return fmt.Sprintf("%d,%02d", p.Revenue/100, p.Revenue%100)
}

func (a *App) panel(ctx context.Context, eventID string) (eventPanel, error) {
	var panel eventPanel
	err := a.DB.QueryRowContext(ctx, `SELECT
		COUNT(CASE WHEN t.status='confirmed' THEN 1 END),
		COUNT(CASE WHEN t.status='pending' THEN 1 END),
		COUNT(CASE WHEN t.status='confirmed' AND t.checked_in=1 THEN 1 END),
		CASE WHEN e.status='cancelled' THEN 0 ELSE e.capacity-COUNT(CASE WHEN t.status IN ('pending','confirmed') THEN 1 END) END,
		COALESCE(SUM(CASE WHEN t.status='confirmed' THEN t.price_cents END),0),
		COUNT(CASE WHEN t.status='refunded' THEN 1 END)
		FROM events e LEFT JOIN tickets t ON t.event_id=e.id
		WHERE e.id=? GROUP BY e.id`, eventID).Scan(
		&panel.Confirmed, &panel.Pending, &panel.CheckIns, &panel.Seats, &panel.Revenue, &panel.Refunded,
	)
	return panel, err
}

func init() {
	routeRegistrars = append(routeRegistrars, (*App).registerOperations)
}

func (a *App) registerOperations(mux *http.ServeMux) {
	mux.HandleFunc("POST /organizador/eventos/{id}/check-in", a.checkIn)
	mux.HandleFunc("POST /organizador/eventos/{id}/cancelar", a.cancelEvent)
}

func (a *App) eventAction(w http.ResponseWriter, r *http.Request) (User, bool) {
	user, ok := a.requireRole(w, r, "organizer")
	if !ok {
		return User{}, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if !a.validCSRF(r) {
		http.Error(w, "Formulário inválido", http.StatusForbidden)
		return User{}, false
	}
	return user, true
}

const invalidCheckIn = "Ingresso inválido para este evento"

func (a *App) checkIn(w http.ResponseWriter, r *http.Request) {
	user, ok := a.eventAction(w, r)
	if !ok {
		return
	}
	ctx, id := r.Context(), r.PathValue("id")
	code := strings.TrimSpace(r.PostFormValue("code"))
	var message string
	var event Event
	err := a.write(ctx, func(conn *sql.Conn) error {
		var err error
		event, err = ownedEvent(ctx, conn, id, user.ID)
		if err != nil {
			return err
		}
		var ticketID, status string
		var used bool
		err = conn.QueryRowContext(ctx, "SELECT id,status,checked_in FROM tickets WHERE event_id=? AND code=?", id, code).Scan(&ticketID, &status, &used)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		switch {
		case errors.Is(err, sql.ErrNoRows):
			message = invalidCheckIn
		case event.Status == "cancelled":
			message = "Evento cancelado"
		case status != "confirmed":
			message = "Ingresso não confirmado"
		case used:
			message = "Ingresso já utilizado"
		default:
			message = "Entrada liberada"
			_, err = conn.ExecContext(ctx, "UPDATE tickets SET checked_in=1 WHERE id=?", ticketID)
			return err
		}
		return nil
	})
	if err != nil {
		eventFailure(w, err)
		return
	}
	a.renderEvent(w, r, http.StatusOK, eventPage{Event: event, Message: message})
}

func (a *App) cancelEvent(w http.ResponseWriter, r *http.Request) {
	user, ok := a.eventAction(w, r)
	if !ok {
		return
	}
	ctx, id := r.Context(), r.PathValue("id")
	err := a.write(ctx, func(conn *sql.Conn) error {
		event, err := ownedEvent(ctx, conn, id, user.ID)
		if err != nil {
			return err
		}
		if event.Status != "published" {
			return errEventLocked
		}
		if _, err := conn.ExecContext(ctx, "UPDATE events SET status='cancelled' WHERE id=?", id); err != nil {
			return err
		}
		_, err = conn.ExecContext(ctx, `UPDATE tickets SET status=CASE status
			WHEN 'pending' THEN 'cancelled' WHEN 'confirmed' THEN 'refunded' END
			WHERE event_id=? AND status IN ('pending','confirmed')`, id)
		return err
	})
	if err != nil {
		eventFailure(w, err)
		return
	}
	http.Redirect(w, r, "/organizador/eventos/"+id, http.StatusSeeOther)
}
