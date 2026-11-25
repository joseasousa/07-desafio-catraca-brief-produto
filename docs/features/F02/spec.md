# F02 — Spec

Evento com dono, status e valores monetários em centavos. O PRD define os critérios A02 X12.

Use `events` com ID textual, ID do usuário dono, início UTC, local, capacidade inteira, preço em centavos e status `draft`, `published` ou `cancelled`. Exibir formulários em português e interpretar a data local em America/Sao_Paulo. Conservar o caminho da página de gestão ao publicar.

A leitura de ocupação é compartilhada com compras e painel. Definir a tabela de ingressos nesta wave para consultar ocupação durante edição antes de existir o checkout. O registro inclui canal, participante opcional, e-mail do comprador, preço, status, código único, check-in, resultado e prazo do gateway. Validar a lotação e persistir a edição sob o mesmo bloqueio de escrita.
