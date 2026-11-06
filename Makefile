include .env.example
-include .env
export

GOLANGCI_LINT ?= $(shell go env GOPATH)/bin/golangci-lint

.PHONY: generate run build test test-unit test-integration lint vet migrate migrate-down migrate-status check-database-url docker-build docker-run

docker-build:
	docker build -t trip-service:lab02 .

docker-run:
	python3 deploy/run-container.py

build:
	mkdir -p bin
	go build -o bin/trip-service ./cmd/service

test:
	go test -race -tags=integration -count=2 -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -n 1

test-unit:
	go test -race -count=2 -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -n 1

test-integration:
	go test -race -tags=integration -count=2 ./internal/repository/trip

lint:
	$(GOLANGCI_LINT) run ./...

vet:
	go vet ./...

run:
	go run ./cmd/service

generate:
	mkdir -p api
	go tool oapi-codegen -config oapi-codegen.yaml contracts/openapi/trip-service.openapi.yaml
	go generate ./internal/...

check-database-url:
	@test -n "$$DATABASE_URL" || { echo "DATABASE_URL is required; set it in .env or pass it to make" >&2; exit 1; }

migrate: check-database-url
	@GOOSE_DRIVER=postgres GOOSE_DBSTRING="$$DATABASE_URL" go tool goose -dir migrations up

migrate-down: check-database-url
	@GOOSE_DRIVER=postgres GOOSE_DBSTRING="$$DATABASE_URL" go tool goose -dir migrations down

migrate-status: check-database-url
	@GOOSE_DRIVER=postgres GOOSE_DBSTRING="$$DATABASE_URL" go tool goose -dir migrations status
