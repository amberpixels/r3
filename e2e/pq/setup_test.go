package r3pq_test

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq" // PostgreSQL driver for database/sql
	"github.com/testcontainers/testcontainers-go"

	"github.com/amberpixels/r3/e2e/harness"
)

func isDockerAvailable() bool { return harness.DockerAvailable() }

// setupPostgresContainer starts Postgres and opens it through lib/pq.
func setupPostgresContainer() (testcontainers.Container, *sql.DB, error) {
	ctx := context.Background()

	pg, err := harness.StartPostgres(ctx)
	if err != nil {
		return nil, nil, err
	}

	db, err := sql.Open("postgres", pg.KeywordDSN())
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
