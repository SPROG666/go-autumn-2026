include .env.example
-include .env
export

.PHONY: generate run build test vet migrate migrate-down migrate-status check-database-url docker-build docker-run

docker-build:
	docker build -t trip-service:lab01 .

docker-run:
	python3 deploy/run-container.py

build:
	mkdir -p bin
	go build -o bin/trip-service ./cmd/trip-service

test:
	go test -race ./...

vet:
	go vet ./...

run:
	go run ./cmd/trip-service

generate:
	mkdir -p api
	go tool oapi-codegen -config oapi-codegen.yaml contracts/openapi/trip-service.openapi.yaml

check-database-url:
	@test -n "$$DATABASE_URL" || { echo "DATABASE_URL is required; set it in .env or pass it to make" >&2; exit 1; }

migrate: check-database-url
	@GOOSE_DRIVER=postgres GOOSE_DBSTRING="$$DATABASE_URL" go tool goose -dir migrations up

migrate-down: check-database-url
	@GOOSE_DRIVER=postgres GOOSE_DBSTRING="$$DATABASE_URL" go tool goose -dir migrations down

migrate-status: check-database-url
	@GOOSE_DRIVER=postgres GOOSE_DBSTRING="$$DATABASE_URL" go tool goose -dir migrations status
