# Default recipe: format and auto-fix everything
default: fmt

# Vet — run the real `go vet` across all packages (immutable check).
# golangci-lint also runs govet, but CI invokes this as a standalone step so a
# vet regression is caught even without golangci-lint installed.
vet:
    go vet ./...

# Lint — check only, never modifies files (immutable).
# Runs the real `go vet` first (via the `vet` recipe), then golangci-lint
# (which also runs its own govet plus many other linters).
lint: lint-install vet
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

# Integration tests for CI: serialize packages with `-p 1` so the
# testcontainers-backed suites (pq/pgx/mysql/gorm/bun/gopg + the petstore
# Postgres) don't all spin up DB containers at once and exhaust the runner —
# the source of intermittent "failed to connect" flakes. Slower but deterministic.
test-integration:
    go test -p 1 ./...

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
