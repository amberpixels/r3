package r3bun_test

import (
	"testing"

	r3bun "github.com/amberpixels/r3/drivers/bun"

	"github.com/amberpixels/r3/e2e/harness"
)

func TestJSONFields(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	if !isDockerAvailable() {
		t.Skip("Docker not available")
	}

	container, db, sqlDB, err := setupPostgresContainer()
	if err != nil {
		t.Skipf("Failed to set up the database container: %v", err)
	}
	defer func() {
		_ = db.Close()
		_ = container.Terminate(t.Context())
	}()

	harness.CreateProfiledPets(t, sqlDB, harness.ProfiledPetsDDLPostgres)
	harness.RunJSONFieldSuite(t, r3bun.NewBunCRUD[harness.ProfiledPet, int64](db), false)
}
