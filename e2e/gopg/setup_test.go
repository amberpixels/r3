package r3gopg_test

import (
	"context"
	"fmt"

	"github.com/go-pg/pg/v10"
	"github.com/testcontainers/testcontainers-go"

	"github.com/amberpixels/r3/e2e/harness"
)

func isDockerAvailable() bool { return harness.DockerAvailable() }

// setupPostgresContainer starts Postgres and connects with go-pg, which takes a
// host:port address rather than a DSN.
func setupPostgresContainer() (testcontainers.Container, *pg.DB, error) {
	ctx := context.Background()

	pgc, err := harness.StartPostgres(ctx)
	if err != nil {
		return nil, nil, err
	}

	db := pg.Connect(&pg.Options{
		Addr:     pgc.Addr(),
		User:     harness.User,
		Password: harness.Password,
		Database: harness.Database,
	})
	if err := db.Ping(ctx); err != nil {
		pgc.Terminate()
		return nil, nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	return pgc.Container, db, nil
}
