include .env
export

DB_URL=postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)

.PHONY: dev run build test test-race migrate-up migrate-down migrate-create migrate-reset docker-up docker-down

dev:
	@air

run:
	@go run ./cmd/server

build:
	@go build -o bin/server ./cmd/server

test:
	@go test -v ./...

test-race:
	@go test -v -race ./...

migrate-up:
	@golang-migrate -path migrations -database "$(DB_URL)" up || migrate -path migrations -database "$(DB_URL)" up

migrate-down:
	@golang-migrate -path migrations -database "$(DB_URL)" down 1 || migrate -path migrations -database "$(DB_URL)" down 1

migrate-reset:
	@golang-migrate -path migrations -database "$(DB_URL)" drop -f || migrate -path migrations -database "$(DB_URL)" drop -f
	@make migrate-up

migrate-create:
	@read -p "Enter migration name: " name; \
	migrate create -ext sql -dir migrations -seq $$name

docker-up:
	@docker-compose up -d --build

docker-down:
	@docker-compose down -v
