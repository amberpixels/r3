package harness

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// StartMongo boots mongo:7 and returns a connected database plus a cleanup func
// that disconnects the client and terminates the container. The "Waiting for
// connections" log is the canonical readiness signal: the port opens before
// Mongo accepts connections, so waiting on the port alone is flaky.
func StartMongo(ctx context.Context) (*mongo.Database, func(), error) {
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
	terminate := func() { _ = container.Terminate(context.Background()) }

	host, port, err := hostPort(ctx, container, "27017")
	if err != nil {
		terminate()
		return nil, nil, err
	}

	client, err := mongo.Connect(options.Client().ApplyURI("mongodb://" + net.JoinHostPort(host, port)))
	if err != nil {
		terminate()
		return nil, nil, fmt.Errorf("connect to mongo: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		terminate()
		return nil, nil, fmt.Errorf("ping mongo: %w", err)
	}

	cleanup := func() {
		_ = client.Disconnect(context.Background())
		terminate()
	}
	return client.Database(Database), cleanup, nil
}
