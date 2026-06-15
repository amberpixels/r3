package r3mongo_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	dockerclient "github.com/moby/moby/client"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// isDockerAvailable checks if Docker (or OrbStack) is reachable without panicking.
func isDockerAvailable() bool {
	defer func() { recover() }()

	if os.Getenv("DOCKER_HOST") == "" {
		orbstackSocket := "unix:///Users/" + os.Getenv("USER") + "/.orbstack/run/docker.sock"
		os.Setenv("DOCKER_HOST", orbstackSocket)
	}

	ctx := context.Background()
	dc, err := testcontainers.NewDockerClientWithOpts(ctx)
	if err != nil {
		return false
	}
	defer dc.Close()

	_, err = dc.Ping(ctx, dockerclient.PingOptions{})
	return err == nil
}

// setupMongoContainer starts a MongoDB container and returns a connected client,
// a fresh database, and a cleanup func. The "Waiting for connections" log is the
// canonical readiness signal — the port opens before Mongo accepts connections,
// so waiting on the port alone is flaky.
func setupMongoContainer(ctx context.Context) (*mongo.Database, func(), error) {
	req := testcontainers.ContainerRequest{
		Image:        "mongo:7",
		ExposedPorts: []string{"27017/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("27017/tcp"),
			wait.ForLog("Waiting for connections"),
		).WithDeadline(90 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("start mongo container: %w", err)
	}

	cleanup := func() { _ = container.Terminate(context.Background()) }

	host, err := container.Host(ctx)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	port, err := container.MappedPort(ctx, "27017")
	if err != nil {
		cleanup()
		return nil, nil, err
	}

	uri := "mongodb://" + net.JoinHostPort(host, port.Port())
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("connect to mongo: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("ping mongo: %w", err)
	}

	closeAll := func() {
		_ = client.Disconnect(context.Background())
		cleanup()
	}
	return client.Database("testdb"), closeAll, nil
}
