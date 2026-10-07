.PHONY: build run test fmt vet tidy compose-up compose-down

build:
	go build -o bin/coterie ./apps/server

run:
	go run ./apps/server

test:
	go test ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

tidy:
	go mod tidy

compose-up:
	docker compose -f deployments/docker-compose.yml up -d

compose-down:
	docker compose -f deployments/docker-compose.yml down
