# All module directories, discovered from go.mod files (root "." sorts first)
modules := `find . -name 'go.mod' -not -path '*/vendor/*' -exec dirname {} \; | sort | tr '\n' ' '`

# Helper script for running a command across all modules with pretty output
run := "bash scripts/foreach-module.sh"

# Default recipe
default: tidy

# Tidy: format, vet, and tidy all modules
tidy:
    @{{ run }} "tidy" "fmt + vet + mod tidy across all modules" {{ modules }} -- bash -c 'go fmt ./... && go vet ./... && go mod tidy'

# Install golangci-lint if not already installed
lint-install:
    @which golangci-lint > /dev/null 2>&1 || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Lint the code using golangci-lint
lint: lint-install
    @{{ run }} "lint" "golangci-lint across all modules" {{ modules }} -- bash -c 'golangci-lint fmt && golangci-lint run'

# Test all modules
test:
    @{{ run }} "test" "go test across all modules" {{ modules }} -- go test ./...

# Test only short tests (skip integration tests requiring Docker)
test-short:
    @{{ run }} "test-short" "go test -short across all modules" {{ modules }} -- go test -short ./...

# Vet all modules
vet:
    @{{ run }} "vet" "go vet across all modules" {{ modules }} -- go vet ./...

# Run go mod tidy on all modules
mod-tidy:
    @{{ run }} "mod-tidy" "go mod tidy across all modules" {{ modules }} -- go mod tidy

# Generate go.work for local development (not committed to git)
go-work:
    go work init
    for dir in {{ modules }}; do \
        go work use $dir; \
    done
    @echo "go.work generated — your IDE can now resolve all modules locally"

# Run the pet store example (starts PostgreSQL in Docker + the Go server)
example:
    docker start petstore-pg 2>/dev/null || \
        docker run -d --name petstore-pg -p 5432:5432 \
            -e POSTGRES_USER=petstore -e POSTGRES_PASSWORD=petstore -e POSTGRES_DB=petstore \
            postgres:18-alpine
    @echo "Waiting for PostgreSQL..."
    @until docker exec petstore-pg pg_isready -U petstore -q 2>/dev/null; do sleep 0.2; done
    go run ./examples/02petstore/cmd
