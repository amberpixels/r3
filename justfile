# All module directories, discovered from go.mod files (root "." sorts first)
modules := `find . -name 'go.mod' -not -path '*/vendor/*' -exec dirname {} \; | sort | tr '\n' ' '`

# Default recipe
default: tidy

# Tidy: format, vet, and tidy all modules
tidy:
    for dir in {{ modules }}; do \
        echo "=> tidy $dir"; \
        (cd $dir && go fmt ./... && go vet ./... && go mod tidy) || exit 1; \
    done

# Install golangci-lint if not already installed
lint-install:
    @which golangci-lint > /dev/null 2>&1 || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Lint the code using golangci-lint
lint: lint-install
    golangci-lint fmt
    golangci-lint run

# Test all modules
test:
    for dir in {{ modules }}; do \
        echo "=> test $dir"; \
        (cd $dir && go test ./...) || exit 1; \
    done

# Test only short tests (skip integration tests requiring Docker)
test-short:
    for dir in {{ modules }}; do \
        echo "=> test-short $dir"; \
        (cd $dir && go test -short ./...) || exit 1; \
    done

# Vet all modules
vet:
    for dir in {{ modules }}; do \
        echo "=> vet $dir"; \
        (cd $dir && go vet ./...) || exit 1; \
    done

# Run go mod tidy on all modules
mod-tidy:
    for dir in {{ modules }}; do \
        echo "=> mod tidy $dir"; \
        (cd $dir && go mod tidy) || exit 1; \
    done

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
