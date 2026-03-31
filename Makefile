APP_NAME := server

.PHONY: help run build test migrate-up migrate-down sqlc-generate compose-up compose-down

help:
	@printf "run build test migrate-up migrate-down sqlc-generate compose-up compose-down\n"

run:
	go run ./cmd/server

build:
	mkdir -p bin
	go build -o ./bin/$(APP_NAME) ./cmd/server

test:
	go test ./...

migrate-up:
	go run ./cmd/server migrate up

migrate-down:
	go run ./cmd/server migrate down

sqlc-generate:
	docker run --rm -v "$(PWD):/src" -w /src sqlc/sqlc:1.29.0 generate

compose-up:
	docker compose up --build

compose-down:
	docker compose down -v

