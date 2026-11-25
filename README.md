# Catraca

MVP local de ingressos para eventos pequenos.

## Como rodar

Pré-requisitos: Git, Docker com Compose e um shell em Linux ou macOS.

```sh
./scripts/up
./scripts/gates
./scripts/down
```

## Endereços

- Produto: http://localhost:8080
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
- [Specs, planos e contratos](docs/features)
- [Estado](estado.json)
- [Instruções para agentes](AGENTS.md)

## Decisões

Go e HTML renderizado no servidor com HTMX mantêm a aplicação em um serviço. SQLite persiste o estado em volume Docker. Os detalhes de concorrência e do gateway estão na [spec de compras](docs/features/F03/spec.md).
