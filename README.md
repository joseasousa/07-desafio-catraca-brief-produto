# Catraca

MVP local de ingressos para eventos pequenos.

## Como rodar

Pré-requisitos: Git, Docker com Compose, Make e um shell em Linux ou macOS.

```sh
make up
make gates
make e2e
make down
```

`make e2e` sobe uma instância isolada com a seed, executa os cenários no Chromium e remove o ambiente de teste.

## Endereços

- Produto ($WEB): http://localhost:8080
- API de parceiros ($API): http://localhost:8080 (rotas em `/api/partner`)

## Credenciais

| Conta | E-mail | Senha |
|---|---|---|
| Organizador A | org-a@example.com | CatracaDemo123! |
| Organizador B | org-b@example.com | CatracaDemo123! |
| Participante | participante@example.com | CatracaDemo123! |

Chave de demonstração para `X-Api-Key`: `catraca-local-partner-key`.

## Artefatos

- [PRD](docs/PRD.md)
- Specs, planos, contratos e relatórios por feature
  - [F01 Contas e ambiente](docs/features/F01)
  - [F02 Eventos](docs/features/F02)
  - [F03 Compras e gateway](docs/features/F03)
  - [F04 Checkout web](docs/features/F04)
  - [F05 API de parceiros](docs/features/F05)
  - [F06 Operação](docs/features/F06)
- [Estado](estado.json)
- [Instruções para agentes](AGENTS.md)
- Wave 4 em paralelo: [F04 Checkout web #2](https://github.com/joseasousa/07-desafio-catraca-brief-produto/pull/2) e [F05 API de parceiros #1](https://github.com/joseasousa/07-desafio-catraca-brief-produto/pull/1).

## Decisões

Go e HTML renderizado no servidor com HTMX mantêm a aplicação em um serviço. SQLite persiste o estado em volume Docker. Uma transação de escrita com `BEGIN IMMEDIATE` serializa a contagem de ocupação e a reserva da vaga, assim como edição de lotação e cancelamento. O gateway simulado registra resultado e prazo na compra. Um processo em segundo plano resolve ingressos pendentes em eventos publicados, inclusive após reiniciar. [Spec de compras](docs/features/F03/spec.md)

Cada implementador entrega o SHA e o contrato a uma sessão separada, com contexto novo. O avaliador testa a versão indicada e registra um relatório novo para cada tentativa.
