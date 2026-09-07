package r3bun_test

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq" // PostgreSQL driver for database/sql
	"github.com/testcontainers/testcontainers-go"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/amberpixels/r3/e2e/harness"
)

func isDockerAvailable() bool { return harness.DockerAvailable() }

// setupPostgresContainer starts Postgres and wraps the database/sql handle with
// Bun. The raw handle comes back too, since goose migrates through it.
func setupPostgresContainer() (testcontainers.Container, *bun.DB, *sql.DB, error) {
	ctx := context.Background()

	pg, err := harness.StartPostgres(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	sqlDB, err := sql.Open("postgres", pg.KeywordDSN())
	if err != nil {
		pg.Terminate()
		return nil, nil, nil, fmt.Errorf("failed to open sql.DB: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		pg.Terminate()
		return nil, nil, nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	return pg.Container, bun.NewDB(sqlDB, pgdialect.New()), sqlDB, nil
}
