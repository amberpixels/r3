package r3gorm_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	dockerclient "github.com/moby/moby/client"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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

func setupPostgresContainer() (testcontainers.Container, *gorm.DB, error) {
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

	// Connect to the PostgreSQL database
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info), // Enable verbose logging
	})
	if err != nil {
		_ = container.Terminate(ctx)

		return nil, nil, err
	}

	return container, db, nil
}
