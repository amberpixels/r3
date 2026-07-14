package r3bun_test

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver for database/sql
	dockerclient "github.com/moby/moby/client"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// isDockerAvailable checks if Docker is available without panicking.
func isDockerAvailable() bool {
	defer func() {
		// Catch any panic from testcontainers
		recover()
	}()

	// For OrbStack users, ensure DOCKER_HOST is set correctly
	if os.Getenv("DOCKER_HOST") == "" {
		// Try OrbStack socket path first
		orbstackSocket := "unix:///Users/" + os.Getenv("USER") + "/.orbstack/run/docker.sock"
		os.Setenv("DOCKER_HOST", orbstackSocket)
	}

	ctx := context.Background()
	dc, err := testcontainers.NewDockerClientWithOpts(ctx)
	if err != nil {
		return false
	}
	defer dc.Close()

	// Try to ping Docker
	_, err = dc.Ping(ctx, dockerclient.PingOptions{})
	return err == nil
}

// setupPostgresContainer creates a PostgreSQL container and returns Bun DB + raw sql.DB for migrations.
func setupPostgresContainer() (testcontainers.Container, *bun.DB, *sql.DB, error) {
	ctx := context.Background()

	// Create PostgreSQL container request
	req := testcontainers.ContainerRequest{
		Image:        "postgres:18-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "testdb",
		},
		// The Postgres entrypoint boots a throwaway server for initdb before the
		// real one, and Docker's port proxy accepts connections before the DB is
		// up, so wait for the second "ready" log line, not just the open port.
		WaitingFor: wait.ForAll(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
			wait.ForListeningPort("5432/tcp"),
		).WithDeadline(60 * time.Second),
	}

	// Start the container
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, nil, nil, err
	}

	// Get the host and port of the container
	host, err := container.Host(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		return nil, nil, nil, err
	}

	// Build the PostgreSQL DSN
	dsn := fmt.Sprintf("host=%s port=%s user=test password=test dbname=testdb sslmode=disable", host, port.Port())

	slog.Info("PostgreSQL DSN: ", "dsn", dsn)

	// Open a standard database/sql connection (Bun wraps this)
	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, nil, nil, fmt.Errorf("failed to open sql.DB: %w", err)
	}

	// Verify connection
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = container.Terminate(ctx)
		return nil, nil, nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	// Wrap with Bun using PostgreSQL dialect
	db := bun.NewDB(sqlDB, pgdialect.New())

	return container, db, sqlDB, nil
}
