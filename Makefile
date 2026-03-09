SHELL := /bin/sh

GO ?= go
NPM ?= npm
UI_DIR ?= ui
DOCKER_IMAGE ?= api-gateway:dev
COMPOSE ?= docker compose

.PHONY: help run test ui-install ui-dev ui-build build docker-build up down logs clean

help:
	@printf "Available targets:\n"
	@printf "  make ui-install   Install UI dependencies\n"
	@printf "  make ui-dev       Run Svelte UI dev server\n"
	@printf "  make ui-build     Build Svelte UI for production\n"
	@printf "  make run          Run API gateway\n"
	@printf "  make test         Run Go tests\n"
	@printf "  make build        Build UI + Go binary\n"
	@printf "  make docker-build Build Docker image\n"
	@printf "  make up           Start gateway + redis (docker compose)\n"
	@printf "  make down         Stop docker compose stack\n"
	@printf "  make logs         Tail docker compose logs\n"
	@printf "  make clean        Remove build artifacts\n"

ui-install:
	$(NPM) --prefix $(UI_DIR) install

ui-dev:
	$(NPM) --prefix $(UI_DIR) run dev

ui-build:
	$(NPM) --prefix $(UI_DIR) run build

run:
	$(GO) run ./cmd/app

test:
	$(GO) test ./...

build: ui-build
	mkdir -p bin
	$(GO) build -o ./bin/api-gateway ./cmd/app

docker-build:
	docker build -t $(DOCKER_IMAGE) .

up:
	$(COMPOSE) up --build

down:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f

clean:
	rm -rf ./bin ./ui/dist
