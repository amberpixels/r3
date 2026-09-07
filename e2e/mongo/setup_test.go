package r3mongo_test

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/amberpixels/r3/e2e/harness"
)

func isDockerAvailable() bool { return harness.DockerAvailable() }

// setupMongoContainer starts MongoDB and returns a fresh database plus the
// cleanup that disconnects the client and stops the container.
func setupMongoContainer(ctx context.Context) (*mongo.Database, func(), error) {
	return harness.StartMongo(ctx)
}
