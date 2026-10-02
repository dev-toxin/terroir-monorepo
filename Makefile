# Terroir — one-command developer workflow. Requires only Go (>= 1.26). Docker is optional.
.DEFAULT_GOAL := help
BACKEND := backend
DEV_DB  ?= $(CURDIR)/tmp/terroir-dev.db
ADDR    ?= :8080

# Dev defaults: SQLite file, demo data, mock LLM, no network.
DEV_ENV = STORAGE=sqlite SQLITE_PATH=$(DEV_DB) AUTO_SEED=true LLM_PROVIDER=mock BACKEND_HTTP_ADDR=$(ADDR) APP_ENV=local

.PHONY: help dev seed reset-dev test test-pg cover lint build up down clean

help: ## Show targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  %-10s %s\n", $$1, $$2}'

dev: ## Run API on SQLite with demo data and mock LLM (no Docker)
	@mkdir -p $(dir $(DEV_DB))
	cd $(BACKEND) && $(DEV_ENV) go run ./cmd/api

seed: ## Seed demo household into the dev SQLite DB (idempotent)
	@mkdir -p $(dir $(DEV_DB))
	cd $(BACKEND) && $(DEV_ENV) go run ./cmd/api -seed

reset-dev: ## Delete the dev SQLite DB (next `make dev` re-creates and re-seeds it)
	rm -f $(DEV_DB) $(DEV_DB)-wal $(DEV_DB)-shm

test: ## Run all tests with the race detector
	cd $(BACKEND) && go test -race ./...

test-pg: ## Run tests incl. PostgreSQL integration (needs `make up`)
	cd $(BACKEND) && TERROIR_PG_TEST_URL=$${TERROIR_PG_TEST_URL:-postgres://terroir_app:change_me_local_password@localhost:5432/terroir?sslmode=disable} go test -race ./...

cover: ## Coverage for domain/service/pairing/llm
	cd $(BACKEND) && go test -cover ./internal/domain/... ./internal/service/... ./internal/pairing/... ./internal/llm/... ./internal/storage/...

lint: ## gofmt check + go vet (+ staticcheck if installed)
	@cd $(BACKEND) && out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi
	cd $(BACKEND) && go vet ./...
	@if command -v staticcheck >/dev/null 2>&1; then cd $(BACKEND) && staticcheck ./...; else echo "staticcheck not installed — skipped"; fi

build: ## Build the API binary into bin/terroir-api
	cd $(BACKEND) && CGO_ENABLED=0 go build -o ../bin/terroir-api ./cmd/api

up: ## Start PostgreSQL + API in Docker
	docker compose up -d --build

down: ## Stop Docker services
	docker compose down

clean: ## Remove build artifacts
	rm -rf bin tmp
