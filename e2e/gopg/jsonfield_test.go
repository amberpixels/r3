package r3gopg_test

import (
	"testing"

	r3gopg "github.com/amberpixels/r3/drivers/gopg"

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

	harness.CreateProfiledPetsWith(t, func(q string) error {
		_, err := db.Exec(q)
		return err
	}, harness.ProfiledPetsDDLPostgres)
	harness.RunJSONFieldSuite(t, r3gopg.NewGoPgCRUD[harness.ProfiledPet, int64](db), false)
}
