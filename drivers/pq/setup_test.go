package r3pq_test

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

	// Build the PostgreSQL DSN
	dsn := fmt.Sprintf("host=%s port=%s user=test password=test dbname=testdb sslmode=disable", host, port.Port())

	slog.Info("PostgreSQL DSN: ", "dsn", dsn)

	// Open a standard database/sql connection
	db, err := sql.Open("postgres", dsn)
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
