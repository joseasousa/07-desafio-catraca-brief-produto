package main

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"
)

const partnerKey = "catraca-local-partner-key"

func init() {
	routeRegistrars = append(routeRegistrars, (*App).registerPartner)
}

func (a *App) registerPartner(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/partner/events", authorizePartner(a.partnerEvents))
	mux.HandleFunc("POST /api/partner/events/{eventId}/purchases", authorizePartner(a.partnerPurchase))
	mux.HandleFunc("GET /api/partner/tickets/{ticketId}", authorizePartner(a.partnerTicket))
}

func authorizePartner(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Api-Key")), []byte(partnerKey)) != 1 {
			partnerError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r)
	}
}

func partnerJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write partner response: %v", err)
	}
}

func partnerError(w http.ResponseWriter, status int, code string) {
	partnerJSON(w, status, map[string]string{"error": code})
}

type partnerEvent struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	StartsAt       string `json:"startsAt"`
	PriceCents     int64  `json:"priceCents"`
	AvailableSeats int64  `json:"availableSeats"`
}

func (a *App) partnerEvents(w http.ResponseWriter, r *http.Request) {
	rows, err := a.DB.QueryContext(r.Context(), "SELECT "+eventColumns+" FROM events e WHERE e.status='published' ORDER BY e.starts_at,e.id")
	if err != nil {
		partnerError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	defer rows.Close()
	events := make([]partnerEvent, 0)
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			partnerError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		events = append(events, partnerEvent{
			ID: event.ID, Name: event.Name, StartsAt: time.Unix(event.StartsAt, 0).In(eventZone).Format(time.RFC3339),
			PriceCents: event.PriceCents, AvailableSeats: event.AvailableSeats(),
		})
	}
	if err := rows.Err(); err != nil {
		partnerError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	partnerJSON(w, http.StatusOK, events)
}

type partnerPurchaseResponse struct {
	TicketID string `json:"ticketId"`
	Code     string `json:"code"`
	Status   string `json:"status"`
}

func (a *App) partnerPurchase(w http.ResponseWriter, r *http.Request) {
	var input struct {
		BuyerEmail string `json:"buyerEmail"`
		CardNumber string `json:"cardNumber"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	decoder := json.NewDecoder(r.Body)
	if decoder.Decode(&input) != nil || decoder.Decode(new(json.RawMessage)) != io.EOF {
		partnerError(w, http.StatusUnprocessableEntity, "invalid_request")
		return
	}
	ticket, err := a.purchase(r.Context(), r.PathValue("eventId"), input.BuyerEmail, nil, input.CardNumber)
	switch {
	case err == nil:
		partnerJSON(w, http.StatusAccepted, partnerPurchaseResponse{TicketID: ticket.ID, Code: ticket.Code, Status: ticket.Status})
	case errors.Is(err, errPurchaseInvalid):
		partnerError(w, http.StatusUnprocessableEntity, "invalid_request")
	case errors.Is(err, errEventUnavailable):
		partnerError(w, http.StatusNotFound, "event_not_available")
	case errors.Is(err, errSoldOut):
		partnerError(w, http.StatusConflict, "sold_out")
	default:
		partnerError(w, http.StatusInternalServerError, "internal_error")
	}
}

type partnerTicketResponse struct {
	TicketID  string `json:"ticketId"`
	Code      string `json:"code"`
	EventID   string `json:"eventId"`
	Status    string `json:"status"`
	CheckedIn bool   `json:"checkedIn"`
}

func (a *App) partnerTicket(w http.ResponseWriter, r *http.Request) {
	ticket, err := a.ticketByID(r.Context(), r.PathValue("ticketId"))
	switch {
	case err == nil:
		partnerJSON(w, http.StatusOK, partnerTicketResponse{TicketID: ticket.ID, Code: ticket.Code, EventID: ticket.EventID, Status: ticket.Status, CheckedIn: ticket.CheckedIn})
	case errors.Is(err, sql.ErrNoRows):
		partnerError(w, http.StatusNotFound, "ticket_not_found")
	default:
		partnerError(w, http.StatusInternalServerError, "internal_error")
	}
}
