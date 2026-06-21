.PHONY: run dev build test test-integration lint format mockery \
	migrate-up migrate-down migrate-create seed up down logs

run:
	go run ./cmd/api

dev:
	air

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o ./bin/app ./cmd/api

test:
	go test -race ./...

test-integration:
	go test -tags=integration ./test/integration/...

lint:
	golangci-lint run ./...

format:
	go fmt ./...
	goimports -w .

mockery:
	mockery

migrate-up:
	go run ./cmd/api -migrate up

migrate-down:
	go run ./cmd/api -migrate down

migrate-create:
	@test -n "$(NAME)" || (echo "NAME is required: make migrate-create NAME=add_x" && exit 1)
	migrate create -ext sql -dir db/migrations -seq $(NAME)

seed:
	docker compose exec -T db psql -U wst -d wst < db/seeds/0001_sample.sql

up:
	docker compose up --build -d

down:
	docker compose down -v

logs:
	docker compose logs -f
