# Variables
GOLANGCI_LINT := $(shell which golangci-lint)

# Default target
all: tidy

# Tidy: format and vet the code
tidy:
	@go fmt $$(go list ./...)
	@go vet $$(go list ./...)
	@go mod tidy

# Install golangci-lint only if it's not already installed
lint-install:
	@if ! [ -x "$(GOLANGCI_LINT)" ]; then \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi

# Lint the code using golangci-lint
# todo reuse var if possible
lint: lint-install
	$(shell which golangci-lint) fmt
	$(shell which golangci-lint) run

test:
	go test ./...

# Run the pet store example (starts PostgreSQL in Docker + the Go server)
example:
	@docker start petstore-pg 2>/dev/null || \
		docker run -d --name petstore-pg -p 5432:5432 \
			-e POSTGRES_USER=petstore -e POSTGRES_PASSWORD=petstore -e POSTGRES_DB=petstore \
			postgres:18-alpine
	@echo "Waiting for PostgreSQL..."
	@until docker exec petstore-pg pg_isready -U petstore -q 2>/dev/null; do sleep 0.2; done
	@go run ./examples/02petstore/cmd

# Phony targets
.PHONY: all tidy lint-install lint test example
