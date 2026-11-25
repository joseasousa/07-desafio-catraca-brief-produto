.DEFAULT_GOAL := up

.PHONY: up down gates
up:
	docker compose up --build --detach --wait

down:
	docker compose down --remove-orphans

gates:
	@set -eu; \
	project="catraca-gates-$$$$"; \
	compose() { docker compose -p "$$project" -f compose.yaml -f compose.gates.yaml "$$@"; }; \
	trap 'compose down --volumes --remove-orphans' EXIT; \
	trap 'exit 130' INT; \
	trap 'exit 143' TERM; \
	compose run --rm --build go-gates
