# S4rCiv test suite — single reproducible entrypoint.
#
# A third party needs only Docker (+ Node for the browser E2E). The host toolchain
# is deliberately NOT relied on: Go 1.26, protoc and Atlas all run inside containers,
# so `git clone && make test` reproduces the full suite without a bespoke setup.
#
# Layers (test pyramid, bottom-heavy):
#   make unit         Go + Rust + web unit tests (fast, hermetic, no orchestration)
#   make cdc          contract checks: proto byte-drift guard + buf breaking
#   make integration  Go drivers vs a REAL migrated Postgres (template-DB-per-test)
#   make e2e          Playwright browser journeys vs the real, deterministically seeded stack
#   make test         everything above, then teardown
#
# Every DB-touching layer uses the real Postgres from compose.yaml — the same image,
# migrations and wiring as production — so tests never diverge from the real stack.

COMPOSE := docker compose -f compose.yaml -f compose.test.yaml
BASELINE_REF ?= origin/main

.PHONY: test unit cdc integration e2e \
        unit-go unit-rust unit-web \
        proto-drift buf-breaking seed up down clean help

help:
	@grep -E '^[a-zA-Z0-9_-]+:.*?##' $(MAKEFILE_LIST) | \
	  awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

## ---- aggregate ----------------------------------------------------------------

test: ## Run the whole suite (unit + cdc + integration + e2e), then teardown
	@$(MAKE) unit && $(MAKE) cdc && $(MAKE) integration && $(MAKE) e2e; \
	  s=$$?; $(MAKE) down; exit $$s

unit: unit-go unit-rust unit-web ## All language unit tests

## ---- unit ---------------------------------------------------------------------

unit-go: ## Go unit tests (domain/usecase/gateway/handler) — fakes only, no DB
	$(COMPOSE) build test-runner
	$(COMPOSE) run --rm test-runner go test ./...

unit-rust: ## Rust differ unit + in-process contract tests
	$(COMPOSE) run --rm --build differ-test

unit-web: ## SvelteKit component/unit tests (Vitest)
	$(COMPOSE) run --rm --build web-test

## ---- contract (CDC) -----------------------------------------------------------

cdc: proto-drift buf-breaking ## Contract checks across service boundaries

proto-drift: ## Fail if the two diff.proto trees diverge (api vs differ, synced manually)
	@./scripts/check-proto-drift.sh

buf-breaking: ## Fail on wire/source-incompatible proto changes vs $(BASELINE_REF)
	cd services/api && buf breaking --against '../../.git#ref=$(BASELINE_REF),subdir=services/api/proto'

## ---- integration (real Postgres) ----------------------------------------------

integration: ## Go drivers vs a real migrated Postgres; template-DB-per-test
	$(COMPOSE) build test-runner
	$(COMPOSE) run --rm test-runner \
	  go test -race -tags=integration -count=1 ./internal/driver/postgres/...

## ---- end-to-end (browser, real seeded stack) ----------------------------------

up: ## Bring the real stack up (db+migrate+api+web), health-gated
	$(COMPOSE) up -d --wait db migrate api web

seed: ## Deterministically seed the E2E database (fixed inputs → reproducible hashes)
	$(COMPOSE) run --rm seed

e2e: up seed ## Playwright browser journeys vs the running, seeded stack
	cd web && pnpm exec playwright test; \
	  s=$$?; $(MAKE) down; exit $$s

## ---- lifecycle ----------------------------------------------------------------

down: ## Stop the stack and remove volumes/orphans
	-$(COMPOSE) down -v --remove-orphans

clean: down ## Teardown + drop built test images
	-docker image rm s4rciv-test-runner s4rciv-differ-test s4rciv-web-test 2>/dev/null || true
