package r3gorm_test

import (
	"context"
	"fmt"

	"github.com/testcontainers/testcontainers-go"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/amberpixels/r3/e2e/harness"
)

func isDockerAvailable() bool { return harness.DockerAvailable() }

// setupPostgresContainer starts Postgres and opens it through gorm.
func setupPostgresContainer() (testcontainers.Container, *gorm.DB, error) {
	ctx := context.Background()

	pg, err := harness.StartPostgres(ctx)
	if err != nil {
		return nil, nil, err
	}

	db, err := gorm.Open(postgres.Open(pg.KeywordDSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		pg.Terminate()
		return nil, nil, fmt.Errorf("failed to open gorm.DB: %w", err)
	}

	return pg.Container, db, nil
}
