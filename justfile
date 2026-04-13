# Default recipe
default: tidy

# Tidy: format, vet, and tidy
tidy:
    go fmt ./...
    go vet ./...
    go mod tidy

# Install golangci-lint if not already installed
lint-install:
    @which golangci-lint > /dev/null 2>&1 || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Lint the code using golangci-lint
lint: lint-install
    golangci-lint fmt
    golangci-lint run

# Test all modules
test:
    go test ./...

# Test only short tests (skip integration tests requiring Docker)
test-short:
    go test -short ./...

# Vet all modules
vet:
    go vet ./...

# Run go mod tidy
mod-tidy:
    go mod tidy

# Run the pet store example (starts PostgreSQL in Docker + the Go server)
example:
    docker start petstore-pg 2>/dev/null || \
        docker run -d --name petstore-pg -p 5432:5432 \
            -e POSTGRES_USER=petstore -e POSTGRES_PASSWORD=petstore -e POSTGRES_DB=petstore \
            postgres:18-alpine
    @echo "Waiting for PostgreSQL..."
    @until docker exec petstore-pg pg_isready -U petstore -q 2>/dev/null; do sleep 0.2; done
    go run ./examples/02petstore/cmd
