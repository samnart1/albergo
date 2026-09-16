DATABASE_URL ?= postgres://albergo:albergo@localhost:5432/albergo?sslmode=disable
STAFF_API_KEY ?= dev-staff-key

.PHONY: up down run test test-int lint fmt

up:
	docker compose -f deploy/docker-compose.yml up -d postgres

down:
	docker compose -f deploy/docker-compose.yml down -v

run:
	DATABASE_URL="$(DATABASE_URL)" STAFF_API_KEY="$(STAFF_API_KEY)" APP_ENV=development LOG_LEVEL=debug go run ./cmd/api

test:
	go test -race -count=1 ./...

test-int:
	go test -race -count=1 -tags=integration ./test/...

fmt:
	gofmt -1 -w .

lint:
	golangci-lint run

seed:
	DATABASE_URL="$(DATABASE_URL)" go run ./cmd/seed/

worker:
	DATABASE_URL="$(DATABASE_URL)" STAFF_API_KEY="$(STAFF_API_KEY)" APP_ENV=development LOG_LEVEL=debug go run ./cmd/worker
