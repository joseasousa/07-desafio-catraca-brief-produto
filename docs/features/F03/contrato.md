# F03 — Contrato

Execute os testes Go com banco SQLite temporário e gateway simulado. Antes de existir uma API parceira, exercite a interface compartilhada de compras diretamente, por chamadas concorrentes a partir de goroutines. A verificação HTTP fica para o fluxo integrado na main.

- F03-1 → X23. Evento inexistente e rascunho recusam compra. Uma compra em publicado retorna pendente com código, ocupa uma vaga e preserva o preço vigente mesmo após edição.
- F03-2 → X23. Duas rajadas de 30 chamadas sincronizadas em eventos novos de cinco lugares retornam 5 compras pendentes e 25 recusas por falta de vaga em cada rodada.
- F03-3 → X23. Alterar lotação para menos que a ocupação falha. Recusa do gateway devolve a vaga. Confirmação preserva a ocupação.
- F03-4 → X23. Validar finais rápidos e lentos e reabrir o banco com uma compra pendente já vencida. O processo retoma e resolve a compra.
