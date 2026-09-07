package r3mysql_test

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql" // MySQL driver for database/sql
	"github.com/testcontainers/testcontainers-go"

	"github.com/amberpixels/r3/e2e/harness"
)

func isDockerAvailable() bool { return harness.DockerAvailable() }

// setupMySQLContainer starts MySQL and opens it through go-sql-driver.
func setupMySQLContainer() (testcontainers.Container, *sql.DB, error) {
	ctx := context.Background()

	my, err := harness.StartMySQL(ctx)
	if err != nil {
		return nil, nil, err
	}

	db, err := sql.Open("mysql", my.DSN())
	if err != nil {
		my.Terminate()
		return nil, nil, fmt.Errorf("failed to open sql.DB: %w", err)
	}
	if err := harness.WaitReady(ctx, db.PingContext); err != nil {
		my.Terminate()
		return nil, nil, err
	}

	return my.Container, db, nil
}
