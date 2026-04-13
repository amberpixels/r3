package r3gopg_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/go-pg/pg/v10"
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

func setupPostgresContainer() (testcontainers.Container, *pg.DB, error) {
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

	// Build the go-pg options
	addr := fmt.Sprintf("%s:%s", host, port.Port())

	slog.Info("PostgreSQL address: ", "addr", addr)

	// Connect using go-pg
	db := pg.Connect(&pg.Options{
		Addr:     addr,
		User:     "test",
		Password: "test",
		Database: "testdb",
	})

	// Verify connection
	if err := db.Ping(ctx); err != nil {
		_ = container.Terminate(ctx)
		return nil, nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	return container, db, nil
}
