package main

import (
	"errors"
	"net/http"
)

func init() {
	routeRegistrars = append(routeRegistrars, (*App).registerCheckout)
}

func (a *App) registerCheckout(mux *http.ServeMux) {
	mux.HandleFunc("POST /eventos/{id}/comprar", a.checkout)
	mux.HandleFunc("GET /meus-ingressos", a.myTickets)
}

func (a *App) checkout(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireRole(w, r, "participant")
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if !a.validCSRF(r) {
		http.Error(w, "Formulário inválido", http.StatusForbidden)
		return
	}
	_, err := a.purchase(r.Context(), r.PathValue("id"), user.Email, &user.ID, r.PostFormValue("cardNumber"))
	switch {
	case err == nil:
		http.Redirect(w, r, "/meus-ingressos", http.StatusSeeOther)
	case errors.Is(err, errPurchaseInvalid):
		a.checkoutError(w, r, http.StatusUnprocessableEntity, "Informe um cartão com 16 dígitos")
	case errors.Is(err, errEventUnavailable):
		a.checkoutError(w, r, http.StatusNotFound, "Evento indisponível")
	case errors.Is(err, errSoldOut):
		a.checkoutError(w, r, http.StatusConflict, "Ingressos esgotados")
	default:
		a.checkoutError(w, r, http.StatusInternalServerError, "Erro ao comprar ingresso")
	}
}

func (a *App) checkoutError(w http.ResponseWriter, r *http.Request, status int, message string) {
	a.render(w, r, status, "Compra", `<h1>Compra de ingresso</h1><p role="alert">{{.Data}}</p><a href="/">Voltar à vitrine</a>`, message)
}

const myTicketsBody = `
<h1>Meus ingressos</h1>
{{range .Data}}<article>
<h2>{{.EventName}}</h2>
<p>Status: {{.StatusLabel}}</p>
<p>Código: <code>{{.Code}}</code></p>
</article>{{else}}<p>Você ainda não comprou ingressos.</p>{{end}}`

func (t Ticket) StatusLabel() string {
	switch t.Status {
	case "pending":
		return "pendente"
	case "confirmed":
		return "confirmado"
	case "declined":
		return "recusado"
	case "cancelled":
		return "cancelado"
	case "refunded":
		return "estornado"
	default:
		return t.Status
	}
}

func (a *App) myTickets(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireRole(w, r, "participant")
	if !ok {
		return
	}
	tickets, err := a.participantTickets(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "Erro ao listar ingressos", http.StatusInternalServerError)
		return
	}
	a.render(w, r, http.StatusOK, "Meus ingressos", myTicketsBody, tickets)
}
