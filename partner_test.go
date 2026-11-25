package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPartnerReadsWebPurchase(t *testing.T) {
	app, _ := purchaseTestApp(t)
	id := purchaseTestEvent(t, app, 1, "published")
	participant := "usr_participante"
	web, err := app.purchase(context.Background(), id, "participante@example.com", &participant, "4000000000000001")
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/partner/tickets/"+web.ID, nil)
	request.Header.Set("X-Api-Key", partnerKey)
	response := httptest.NewRecorder()
	app.routes().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
	var result partnerTicketResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	want := partnerTicketResponse{TicketID: web.ID, Code: web.Code, EventID: id, Status: "pending"}
	if result != want {
		t.Fatalf("ticket = %+v, want %+v", result, want)
	}
}
