// Package harness starts the throwaway database containers the r3 integration
// suite runs against, and locates the shared goose migrations. It exists so the
// container stack lives in this module only: importing an r3 driver must never
// drag testcontainers, moby or docker into a consumer's go.sum.
package harness

import (
	"context"
	"os"
	"path/filepath"
	"runtime"

	dockerclient "github.com/moby/moby/client"
	"github.com/testcontainers/testcontainers-go"
)

// Credentials every container in this package is started with. Throwaway
// containers on a random port, so they are fixed rather than generated.
const (
	User     = "test"
	Password = "test"
	Database = "testdb"
)

// DockerAvailable reports whether a Docker daemon is reachable, without
// panicking when it is not. Tests skip on false rather than fail.
func DockerAvailable() bool {
	// testcontainers panics rather than returning an error on some misconfigured
	// hosts, so the probe has to survive that too.
	defer func() { _ = recover() }()

	// OrbStack does not populate DOCKER_HOST, so point at its socket before
	// giving up on an otherwise working daemon.
	if os.Getenv("DOCKER_HOST") == "" {
		// Best effort: a daemon we cannot point at just means DockerAvailable is false.
		_ = os.Setenv("DOCKER_HOST", "unix:///Users/"+os.Getenv("USER")+"/.orbstack/run/docker.sock")
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

// repoRoot is the r3 checkout this module sits inside, resolved from this
// file's own compile-time path so no caller has to know its depth below it.
func repoRoot() string {
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	// self is <root>/e2e/harness/docker.go
	return filepath.Dir(filepath.Dir(filepath.Dir(self)))
}

// MigrationsDir is the goose migration set for the given flavor, as an absolute
// path. Flavor is "" or "postgres" for the default set, "mysql" or "sqlite" for
// the per-flavor variants.
func MigrationsDir(flavor string) string {
	dir := "migrations"
	switch flavor {
	case "mysql", "sqlite":
		dir += "_" + flavor
	}
	return filepath.Join(repoRoot(), "internal", "testing", dir)
}
