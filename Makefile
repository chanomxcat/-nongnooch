# All targets run in Docker. Never call go/npm on the host.
SHELL := bash
export MSYS_NO_PATHCONV := 1
COMPOSE  := docker compose -f deploy/compose.yaml --project-directory .
# Default is not ./... because docs/node_modules makes the package walk very slow.
PKG      ?= ./pkg/... ./services/...
RUN      ?=
REGISTRY ?= nongnooch
TAG      ?= dev

.PHONY: up down logs test test-int tidy build gen

up:
	$(COMPOSE) up -d --wait

down:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f --tail=100

test:
	$(COMPOSE) run --rm --no-deps gotest go test -race -count=1 -run "$(RUN)" $(PKG)

test-int:
	$(COMPOSE) run --rm gotest go test -race -count=1 -p 1 -tags=integration -run "$(RUN)" $(PKG)

tidy:
	$(COMPOSE) run --rm --no-deps gotest go mod tidy

gen:
	$(COMPOSE) run --rm --no-deps gotest sh deploy/gen-proto.sh

build:
	docker build -f deploy/Dockerfile.go --build-arg SVC=$(SVC) -t $(REGISTRY)/$(SVC):$(TAG) .

# ---- Frontend (Next.js) — all in node:20-bookworm via the `node` tools service ----
APP ?= web
WS   = $(if $(filter web admin gate,$(APP)),apps/$(APP),packages/$(APP))
NODE = $(COMPOSE) run --rm --no-deps node

.PHONY: fe-install fe-lint fe-test fe-build build-web e2e tokens

fe-install:
	$(NODE) npm ci --no-audit --no-fund

tokens:
	$(NODE) npm run build -w @nn/tokens

fe-lint:
	$(NODE) sh -c "npm run build -w @nn/tokens && npm run lint -w $(WS)"

fe-test:
	$(NODE) npm run test -w $(WS) -- $(RUN)

fe-build:
	$(NODE) sh -c "npm run build -w @nn/tokens && npm run build -w $(WS)"

build-web:
	docker build -f deploy/Dockerfile.web --build-arg APP=$(APP) -t $(REGISTRY)/$(APP):$(TAG) .

e2e:
	$(COMPOSE) --profile e2e run --rm -w /src/apps/$(APP) playwright npx playwright test $(SPEC)

# ---- Demo ----
.PHONY: seed demo-reset

# Service seeds (identity -> member -> commerce -> ticket -> notify -> content) + today's demo history
# (orders / tickets / scans via the real APIs). Runs on the compose network; idempotent.
seed:
	$(COMPOSE) run --rm --no-deps node node deploy/seed/seed.mjs $(SEED_ARGS)

# Wipes demo data (MariaDB + NATS volumes, mockext memory), rebuilds, starts and seeds.
# Tool caches (node_modules, Go modules) are kept.
demo-reset:
ifneq ($(CONFIRM),yes)
	@echo "Refusing: demo-reset deletes all demo data. Run: make demo-reset CONFIRM=yes"; exit 1
else
	$(COMPOSE) --profile e2e --profile tools down --remove-orphans
	-docker volume rm nongnooch_mariadb-data nongnooch_nats-data
	$(COMPOSE) up -d --build --wait
	$(MAKE) seed
endif
