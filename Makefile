.DEFAULT_GOAL := help

.PHONY: help
help: ## List the available targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F ':.*## ' '{printf "  %-10s %s\n", $$1, $$2}'

.PHONY: run
run: ## Run the server locally with in-memory statistics
	go run ./cmd/server

.PHONY: build
build: ## Build the server binary into bin/
	CGO_ENABLED=0 go build -trimpath -o bin/server ./cmd/server

.PHONY: test
test: ## Run the tests with the race detector
	go test -race ./...

.PHONY: cover
cover: ## Run the tests and print the coverage per function
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

.PHONY: lint
lint: ## Run the linters (requires golangci-lint v2)
	golangci-lint run

.PHONY: up
up: ## Start the server and Redis with Docker Compose
	docker compose up --build

.PHONY: down
down: ## Stop the Docker Compose stack
	docker compose down
