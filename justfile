# Default recipe: format and auto-fix everything
default: fmt

# Lint — check only, never modifies files (immutable).
# golangci-lint runs govet (enable-all), so no separate vet step is needed.
lint: lint-install
    golangci-lint run

# Format & auto-fix — fixes everything that can be fixed (mutable)
fmt: lint-install
    golangci-lint fmt
    golangci-lint run --fix
    go mod tidy

# Install golangci-lint if not already installed
lint-install:
    @which golangci-lint > /dev/null 2>&1 || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Test all modules (requires Docker for integration tests)
test:
    go test ./...

# Test only short tests (skip integration tests requiring Docker)
test-short:
    go test -short ./...

# Run the pet store example (starts PostgreSQL in Docker + the Go server)
example:
    docker start petstore-pg 2>/dev/null || \
        docker run -d --name petstore-pg -p 5432:5432 \
            -e POSTGRES_USER=petstore -e POSTGRES_PASSWORD=petstore -e POSTGRES_DB=petstore \
            postgres:18-alpine
    @echo "Waiting for PostgreSQL..."
    @until docker exec petstore-pg pg_isready -U petstore -q 2>/dev/null; do sleep 0.2; done
    go run ./examples/02petstore/cmd
