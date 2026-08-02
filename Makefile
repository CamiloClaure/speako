.DEFAULT_GOAL := help
SHELL := /bin/bash
API := api
WEB := web
DB ?= $(PWD)/api/speako.dev.db

.PHONY: help test arch lint run seed build e2e web-dev clean

help: ## Show available targets
	@grep -E '^[a-z-]+:.*?## ' $(MAKEFILE_LIST) | awk -F':.*?## ' '{printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

test: ## Unit + contract tests (Go)
	cd $(API) && go test ./...

arch: ## Enforce the dependency rule - must pass before any task is "done"
	cd $(API) && go test -run TestDependencyRule -v ./...

lint: ## go vet + gofmt check
	cd $(API) && go vet ./... && test -z "$$(gofmt -l . | tee /dev/stderr)"

run: ## Run the API on :8080
	cd $(API) && SPEAKO_DB_PATH=$(DB) go run ./cmd/api

seed: ## Reset the dev database with teacher, availability and products
	rm -f $(DB) $(DB)-wal $(DB)-shm
	cd $(API) && SPEAKO_DB_PATH=$(DB) go run ./cmd/speako-cli migrate
	cd $(API) && SPEAKO_DB_PATH=$(DB) go run ./cmd/speako-cli seed

build: ## Static API binary + web bundle
	cd $(API) && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ../bin/speako ./cmd/api
	cd $(WEB) && npm ci && npm run build

web-dev: ## Vite dev server on :5173
	cd $(WEB) && npm run dev

e2e: ## Playwright end-to-end against a live stack
	cd $(WEB) && npm run e2e

clean: ## Remove build artifacts and the dev database
	rm -rf bin $(WEB)/dist $(DB) $(DB)-wal $(DB)-shm
