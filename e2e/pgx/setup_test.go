package r3pgx_test

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib" // pgx driver for database/sql
	"github.com/testcontainers/testcontainers-go"

	"github.com/amberpixels/r3/e2e/harness"
)

func isDockerAvailable() bool { return harness.DockerAvailable() }

// setupPostgresContainer starts Postgres and opens it through pgx's stdlib
// driver, which prefers the URL-style connection string.
func setupPostgresContainer() (testcontainers.Container, *sql.DB, error) {
	ctx := context.Background()

	pg, err := harness.StartPostgres(ctx)
	if err != nil {
		return nil, nil, err
	}

	db, err := sql.Open("pgx", pg.URLDSN())
	if err != nil {
		pg.Terminate()
		return nil, nil, fmt.Errorf("failed to open sql.DB: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		pg.Terminate()
		return nil, nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	return pg.Container, db, nil
}
