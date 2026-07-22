SHELL := /bin/sh

.PHONY: help up down logs rebuild test test-race fmt vet demo idempotency-check reset-db

help:
	@echo "make up                 Start PostgreSQL and the app"
	@echo "make down               Stop containers"
	@echo "make logs               Follow app logs"
	@echo "make rebuild            Rebuild and restart"
	@echo "make test               Run unit tests"
	@echo "make test-race          Run tests with race detector"
	@echo "make fmt                 Format Go source"
	@echo "make vet                 Run go vet"
	@echo "make demo                Run curl smoke demo"
	@echo "make idempotency-check   Reproduce lost-response retry"
	@echo "make reset-db            Delete local database volume"

up:
	docker compose up --build -d
	@echo "Open http://localhost:8080"

down:
	docker compose down

logs:
	docker compose logs -f app

rebuild:
	docker compose up --build -d

test:
	go test ./...

test-race:
	go test -race ./...

fmt:
	gofmt -w $$(find . -name '*.go' -type f)

vet:
	go vet ./...

demo:
	./scripts/demo.sh

idempotency-check:
	./scripts/idempotency-check.sh

reset-db:
	docker compose down -v
