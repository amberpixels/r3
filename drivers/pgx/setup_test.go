package r3pgx_test

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // pgx driver for database/sql
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
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
	client, err := testcontainers.NewDockerClientWithOpts(ctx)
	if err != nil {
		return false
	}
	defer client.Close()

	// Try to ping Docker
	_, err = client.Ping(ctx)
	return err == nil
}

func setupPostgresContainer() (testcontainers.Container, *sql.DB, error) {
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
		WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(30 * time.Second),
	}

	// Start the container
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, nil, err
	}

	// Get the host and port of the container
	host, err := container.Host(ctx)
	if err != nil {
		return nil, nil, err
	}

	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		return nil, nil, err
	}

	// Build the PostgreSQL DSN (pgx prefers URL-style connection strings)
	dsn := fmt.Sprintf("postgres://test:test@%s/testdb?sslmode=disable", net.JoinHostPort(host, port.Port()))

	slog.Info("PostgreSQL DSN: ", "dsn", dsn)

	// Open a standard database/sql connection using pgx stdlib driver
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, nil, fmt.Errorf("failed to open sql.DB: %w", err)
	}

	// Verify connection
	if err := db.PingContext(ctx); err != nil {
		_ = container.Terminate(ctx)
		return nil, nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	return container, db, nil
}
