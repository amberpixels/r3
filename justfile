# All module directories (order matters: dependencies first)
modules := ". ./dialects/bson ./dialects/json ./dialects/sql ./mongobase ./sqlbase ./drivers/bun ./drivers/gopg ./drivers/gorm ./drivers/mongo ./drivers/mysql ./drivers/pgx ./drivers/pq ./drivers/sqlite3 ./examples"

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

# Run the pet store example (starts PostgreSQL in Docker + the Go server)
example:
    docker start petstore-pg 2>/dev/null || \
        docker run -d --name petstore-pg -p 5432:5432 \
            -e POSTGRES_USER=petstore -e POSTGRES_PASSWORD=petstore -e POSTGRES_DB=petstore \
            postgres:18-alpine
    @echo "Waiting for PostgreSQL..."
    @until docker exec petstore-pg pg_isready -U petstore -q 2>/dev/null; do sleep 0.2; done
    go run ./examples/02petstore/cmd
