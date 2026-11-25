# Catraca

Use `make up`, `make down`, `make gates` e `make e2e`. Amplie a cobertura a cada wave. Use `make e2e` para fluxos expostos pela interface ou API e `make gates` para contratos de serviço, como o da F03 antes de existirem os canais de compra. Leia o PRD em `docs/PRD.md` e consulte `estado.json` antes de iniciar uma feature. Cada diretório em `docs/features` contém spec, plano, contrato e avaliações.

Implemente na ordem das waves. Entregue o SHA e o contrato a um avaliador com contexto separado. Preserve cada relatório commitado. Só avance a wave após aprovação e integração. Nunca guarde o cartão. Toda mutação do domínio usa a transação de escrita compartilhada.
