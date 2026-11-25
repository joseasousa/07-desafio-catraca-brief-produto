# F05 — Contrato

Subir com `./scripts/up`. Usar a chave do README, dois eventos criados pela F02 (um rascunho e um publicado) e um ingresso de participante criado pelo serviço F03. Usar curl ou testes Go HTTP com estas mesmas entradas.

- F05-1 → X25. GET /api/partner/events lista apenas o evento publicado com id, name, startsAt, priceCents e availableSeats. Uma compra pendente reduz as vagas na listagem.
- F05-2 → X35. Para cada rota, chamar com chave ausente e com chave inválida. Em ambos os casos esperar 401 e unauthorized, inclusive se a compra tiver corpo inválido.
- F05-3 → X35. Com a chave correta, enviar e-mail inválido, campo ausente e cartão inválido. Cada um responde 422 invalid_request antes de resolver o evento. Corpo válido em evento inexistente ou rascunho responde 404 event_not_available. Evento publicado sem vagas responde 409 sold_out. Compra válida com vaga retorna 202 com ticketId, code e status pending.
- F05-4 → X35. Consultar o ingresso recém-criado e um do canal web. Conferir ticketId, code, eventId, status e checkedIn. ID inexistente retorna 404 ticket_not_found.
