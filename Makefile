# All targets run in Docker. Never call go/npm on the host.
SHELL := bash
export MSYS_NO_PATHCONV := 1
COMPOSE  := docker compose -f deploy/compose.yaml --project-directory .
# Default is not ./... because docs/node_modules makes the package walk very slow.
PKG      ?= ./pkg/... ./services/...
RUN      ?=
REGISTRY ?= nongnooch
TAG      ?= dev

.PHONY: up down logs test test-int tidy build

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

build:
	docker build -f deploy/Dockerfile.go --build-arg SVC=$(SVC) -t $(REGISTRY)/$(SVC):$(TAG) .
