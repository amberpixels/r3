package r3mysql_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql" // MySQL driver for database/sql
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

func setupMySQLContainer() (testcontainers.Container, *sql.DB, error) {
	ctx := context.Background()

	// Create MySQL container request
	req := testcontainers.ContainerRequest{
		Image:        "mysql:8",
		ExposedPorts: []string{"3306/tcp"},
		Env: map[string]string{
			"MYSQL_ROOT_PASSWORD": "test",
			"MYSQL_USER":          "test",
			"MYSQL_PASSWORD":      "test",
			"MYSQL_DATABASE":      "testdb",
		},
		WaitingFor: wait.ForListeningPort("3306/tcp").WithStartupTimeout(60 * time.Second),
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

	port, err := container.MappedPort(ctx, "3306")
	if err != nil {
		return nil, nil, err
	}

	// Build the MySQL DSN (go-sql-driver/mysql format)
	dsn := fmt.Sprintf("test:test@tcp(%s:%s)/testdb?parseTime=true&multiStatements=true", host, port.Port())

	slog.Info("MySQL DSN: ", "dsn", dsn)

	// Open a standard database/sql connection
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, nil, fmt.Errorf("failed to open sql.DB: %w", err)
	}

	// MySQL may take a moment to accept connections after port is open.
	// Retry ping with backoff.
	for range 30 {
		if err := db.PingContext(ctx); err == nil {
			return container, db, nil
		}
		time.Sleep(1 * time.Second)
	}

	_ = container.Terminate(ctx)
	return nil, nil, errors.New("failed to ping MySQL after retries")
}
