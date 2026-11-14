package r3sqlite3nocgo_test

import (
	"database/sql"
	"testing"

	r3sqlite3 "github.com/amberpixels/r3/drivers/sqlite3"
	"github.com/stretchr/testify/require"

	"github.com/amberpixels/r3/e2e/harness"
)

func TestSqlite3_JSONFields(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)

	harness.CreateProfiledPets(t, db, harness.ProfiledPetsDDLSQLite)
	harness.RunJSONFieldSuite(t, r3sqlite3.NewSqlite3CRUD[harness.ProfiledPet, int64](db), true)
}
