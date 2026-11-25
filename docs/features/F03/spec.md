# F03 — Spec

BEGIN IMMEDIATE protege a reserva. Resultado e prazo do gateway persistem no ingresso. O PRD define os critérios X23.

A compra recebe evento, e-mail, participante opcional e cartão. Devolve ID e código do ingresso pendente ou erro tipado de validação, evento indisponível ou esgotado. A mesma interface serve os dois canais. A consulta por ID atende parceiros. A consulta filtrada pelo participante atende Meus ingressos.

O gateway usa 3 segundos para cartões rápidos e 35 segundos para lentos. Finais 0001 e 0003 aprovam. O processo consulta vencimentos a cada 100 ms e só atualiza ingressos pendentes de eventos publicados. Persistir somente resultado e prazo. Usar relógio configurável em testes.
