package r3mongo_test

import (
	"testing"

	"github.com/amberpixels/r3"
	r3mongo "github.com/amberpixels/r3/drivers/mongo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/amberpixels/r3/e2e/harness"
)

// mongoPet keeps its profile in an r3.JSONColumn, which the bson codec stores
// as a {val: {...}} subdocument; JSON fields address keys inside it.
type mongoPet struct {
	ID      bson.ObjectID                     `bson:"_id"`
	Name    string                            `bson:"name"`
	Profile r3.JSONColumn[harness.PetProfile] `bson:"profile"`
}

func TestMongoJSONFields(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	if !isDockerAvailable() {
		t.Skip("Docker not available - Mongo integration test requires Docker")
	}

	ctx := t.Context()
	db, cleanup, err := setupMongoContainer(ctx)
	if err != nil {
		t.Skipf("Failed to set up Mongo container: %v", err)
	}
	defer cleanup()

	repo := r3mongo.NewMongoCRUD[mongoPet, bson.ObjectID](db.Collection("profiled_pets"))
	for _, p := range []mongoPet{
		{Name: "Rex", Profile: r3.NewJSONColumn(harness.PetProfile{
			Breed: "beagle", Age: 9, Vaccinations: map[string]int{"count": 3},
		})},
		{Name: "Bo", Profile: r3.NewJSONColumn(harness.PetProfile{Breed: "pug", Age: 10})},
		{Name: "Max", Profile: r3.NewJSONColumn(harness.PetProfile{Breed: "beagle", Age: 2})},
		{Name: "Nox", Profile: r3.NewJSONColumn(harness.PetProfile{})},
	} {
		p.ID = bson.NewObjectID()
		_, err := repo.Create(ctx, p)
		require.NoError(t, err)
	}
	names := func(q r3.Query) []string {
		pets, _, err := repo.List(ctx, q)
		require.NoError(t, err)
		out := make([]string, len(pets))
		for i, p := range pets {
			out[i] = p.Name
		}
		return out
	}

	breed := r3.JSONField("profile", "breed")
	age := r3.JSONField("profile", "age").AsNumber()

	assert.Equal(t, []string{"Max", "Rex"}, names(r3.Query{
		Filters: r3.Filters{r3.F(breed, "beagle")},
		Sorts:   r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("name"))},
	}))
	assert.Equal(t, []string{"Bo", "Rex"}, names(r3.Query{
		Filters: r3.Filters{r3.Fop(age, r3.OperatorGt, "5")},
		Sorts:   r3.Sorts{r3.NewSortDescSpec(age)},
	}))
	assert.Equal(t, []string{"Rex"}, names(r3.Query{
		Filters: r3.Filters{r3.F(r3.JSONField("profile", "vaccinations", "count").AsNumber(), 3)},
	}))

	_, err = repo.Aggregate(ctx, r3.Query{
		GroupBy:    r3.Fields{breed},
		Aggregates: r3.Aggregates{r3.AggCount("n")},
	})
	require.ErrorIs(t, err, r3.ErrJSONFieldNotSupported)
}
