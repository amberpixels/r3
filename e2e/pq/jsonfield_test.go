package r3pq_test

import (
	"testing"

	r3pq "github.com/amberpixels/r3/drivers/pq"

	"github.com/amberpixels/r3/e2e/harness"
)

func TestJSONFields(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	if !isDockerAvailable() {
		t.Skip("Docker not available")
	}

	container, db, err := setupPostgresContainer()
	if err != nil {
		t.Skipf("Failed to set up the database container: %v", err)
	}
	defer func() {
		_ = db.Close()
		_ = container.Terminate(t.Context())
	}()

	harness.CreateProfiledPets(t, db, harness.ProfiledPetsDDLPostgres)
	harness.RunJSONFieldSuite(t, r3pq.NewPqCRUD[harness.ProfiledPet, int64](db), true)
}
