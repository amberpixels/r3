package harness

import (
	"context"
	"database/sql"
	"testing"

	"github.com/amberpixels/r3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PetProfile is the JSON document stored in ProfiledPet.Profile.
type PetProfile struct {
	Breed        string         `json:"breed,omitempty"`
	Age          int            `json:"age,omitempty"`
	Vaccinations map[string]int `json:"vaccinations,omitempty"`
}

// ProfiledPet is the entity the JSON-field suite runs on: a pet whose profile
// is one JSON column. Tags cover the database/sql drivers (db), gorm, bun and go-pg.
type ProfiledPet struct {
	ID      int64                     `bun:"id,pk,autoincrement" db:"id,pk"   gorm:"primaryKey" pg:"id,pk"`
	Name    string                    `bun:"name"                db:"name"`
	Profile r3.JSONColumn[PetProfile] `bun:"profile"             db:"profile"`

	tableName struct{} `pg:"profiled_pets"` //nolint:unused // read by go-pg's tag parser
}

// TableName pins gorm to the table the DDL below creates.
func (ProfiledPet) TableName() string { return ProfiledPetsTable }

// ProfiledPetsTable is the table the suite creates and drops.
const ProfiledPetsTable = "profiled_pets"

// Per-flavor DDL for ProfiledPetsTable. Postgres uses jsonb and MySQL JSON so
// the native JSON types are exercised; SQLite stores the JSON as TEXT.
const (
	ProfiledPetsDDLSQLite = `CREATE TABLE profiled_pets (
		id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, profile TEXT)`
	ProfiledPetsDDLPostgres = `CREATE TABLE profiled_pets (
		id bigserial PRIMARY KEY, name text NOT NULL, profile jsonb)`
	ProfiledPetsDDLMySQL = `CREATE TABLE profiled_pets (
		id BIGINT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(64) NOT NULL, profile JSON)`
)

// CreateProfiledPets (re)creates ProfiledPetsTable with ddl and drops it when
// the test ends.
func CreateProfiledPets(t *testing.T, db *sql.DB, ddl string) {
	t.Helper()
	CreateProfiledPetsWith(t, func(q string) error {
		_, err := db.ExecContext(context.Background(), q)
		return err
	}, ddl)
}

// CreateProfiledPetsWith is [CreateProfiledPets] for a driver without a
// *sql.DB (go-pg): exec runs one statement.
func CreateProfiledPetsWith(t *testing.T, exec func(query string) error, ddl string) {
	t.Helper()
	_ = exec("DROP TABLE IF EXISTS " + ProfiledPetsTable)
	require.NoError(t, exec(ddl))
	t.Cleanup(func() { _ = exec("DROP TABLE IF EXISTS " + ProfiledPetsTable) })
}

// JSONFieldRepo is what the suite needs from a repository.
type JSONFieldRepo interface {
	r3.CRUD[ProfiledPet, int64]
	r3.Aggregator
}

// RunJSONFieldSuite seeds ProfiledPetsTable through repo and checks filters,
// sorts and group-by on keys inside the profile column: a string key, a
// numeric key, a nested key and a missing key, plus a bad key segment that
// must fail before any SQL runs.
//
// schemaChecked says the driver validates queries against the entity's schema,
// so a JSON field on a non-JSON column is rejected too; bun and go-pg do not.
func RunJSONFieldSuite(t *testing.T, repo JSONFieldRepo, schemaChecked bool) {
	t.Helper()
	ctx := t.Context()

	for _, p := range []ProfiledPet{
		{Name: "Rex", Profile: r3.NewJSONColumn(PetProfile{
			Breed: "beagle", Age: 9, Vaccinations: map[string]int{"count": 3},
		})},
		{Name: "Bo", Profile: r3.NewJSONColumn(PetProfile{Breed: "pug", Age: 10})},
		{Name: "Max", Profile: r3.NewJSONColumn(PetProfile{Breed: "beagle", Age: 2})},
		{Name: "Nox", Profile: r3.NewJSONColumn(PetProfile{})},
	} {
		_, err := repo.Create(ctx, p)
		require.NoError(t, err)
	}

	breed := r3.JSONField("profile", "breed")
	age := r3.JSONField("profile", "age").AsNumber()
	byName := r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("name"))}

	list := func(t *testing.T, q r3.Query) []string {
		t.Helper()
		pets, _, err := repo.List(ctx, q)
		require.NoError(t, err)
		names := make([]string, len(pets))
		for i, p := range pets {
			names[i] = p.Name
		}
		return names
	}

	t.Run("string key", func(t *testing.T) {
		assert.Equal(t, []string{"Max", "Rex"}, list(t, r3.Query{
			Filters: r3.Filters{r3.F(breed, "beagle")}, Sorts: byName,
		}))
		assert.Equal(t, []string{"Bo", "Max", "Rex"}, list(t, r3.Query{
			Filters: r3.Filters{r3.Fop(breed, r3.OperatorIn, []string{"beagle", "pug"})}, Sorts: byName,
		}))
	})

	t.Run("numeric key compares and sorts as a number", func(t *testing.T) {
		// Text order would put "10" before "9".
		assert.Equal(t, []string{"Bo", "Rex"}, list(t, r3.Query{
			Filters: r3.Filters{r3.Fop(age, r3.OperatorGt, 5)},
			Sorts:   r3.Sorts{r3.NewSortDescSpec(age)},
		}))
		assert.Equal(t, []string{"Max", "Rex", "Bo"}, list(t, r3.Query{
			Filters: r3.Filters{r3.Fop(age, r3.OperatorExists, true)},
			Sorts:   r3.Sorts{r3.NewSortAscSpec(age)},
		}))
	})

	t.Run("nested key", func(t *testing.T) {
		assert.Equal(t, []string{"Rex"}, list(t, r3.Query{
			Filters: r3.Filters{r3.F(r3.JSONField("profile", "vaccinations", "count").AsNumber(), 3)},
		}))
	})

	t.Run("missing key is NULL", func(t *testing.T) {
		assert.Equal(t, []string{"Nox"}, list(t, r3.Query{Filters: r3.Filters{r3.F(breed, nil)}}))
		assert.Equal(t, []string{"Bo", "Max", "Rex"}, list(t, r3.Query{
			Filters: r3.Filters{r3.Fop(breed, r3.OperatorNe, "poodle")}, Sorts: byName,
		}), "Ne does not match a missing key either")
	})

	t.Run("group by a key", func(t *testing.T) {
		rows, err := repo.Aggregate(ctx, r3.Query{
			Filters:    r3.Filters{r3.Fop(breed, r3.OperatorExists, true)},
			GroupBy:    r3.Fields{breed},
			Aggregates: r3.Aggregates{r3.AggCount("n")},
			Sorts:      r3.Sorts{r3.NewSortAscSpec(breed)},
		})
		require.NoError(t, err)
		require.Len(t, rows, 2)
		got, _ := rows[0].String(breed.String())
		assert.Equal(t, "beagle", got)
		n, _ := rows[0].Int64("n")
		assert.Equal(t, int64(2), n)
	})

	t.Run("rejected before SQL", func(t *testing.T) {
		_, _, err := repo.List(ctx, r3.Query{Filters: r3.Filters{r3.F(r3.JSONField("profile", "x;y"), "v")}})
		require.ErrorIs(t, err, r3.ErrInvalidJSONField)
		_, _, err = repo.List(ctx, r3.Query{Filters: r3.Filters{r3.F(r3.JSONField("profile", "a b"), "v")}})
		require.ErrorIs(t, err, r3.ErrInvalidJSONField)
		_, _, err = repo.List(ctx, r3.Query{Filters: r3.Filters{r3.Fop(breed, r3.OperatorGt, 5)}})
		require.ErrorIs(t, err, r3.ErrJSONFieldValueType)
		if schemaChecked {
			_, _, err = repo.List(ctx, r3.Query{Filters: r3.Filters{r3.F(r3.JSONField("name", "x"), "v")}})
			require.ErrorIs(t, err, r3.ErrJSONFieldNotJSONColumn)
		}
	})
}
