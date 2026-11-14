package r3gorm_test

import (
	"testing"

	r3gorm "github.com/amberpixels/r3/drivers/gorm"
	"github.com/stretchr/testify/require"

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
	defer func() { _ = container.Terminate(t.Context()) }()

	sqlDB, err := db.DB()
	require.NoError(t, err)
	harness.CreateProfiledPets(t, sqlDB, harness.ProfiledPetsDDLPostgres)
	harness.RunJSONFieldSuite(t, r3gorm.NewGormCRUD[harness.ProfiledPet, int64](db), true)
}
