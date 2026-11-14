package enginefile_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amberpixels/r3"
	enginefile "github.com/amberpixels/r3/engine/file"
)

type petProfile struct {
	Breed        string         `json:"breed,omitempty"`
	Age          int            `json:"age,omitempty"`
	Vaccinations map[string]int `json:"vaccinations,omitempty"`
}

type profiledPet struct {
	ID      int                       `json:"id"      r3:"id,pk"`
	Name    string                    `json:"name"`
	Profile r3.JSONColumn[petProfile] `json:"profile"`
}

func TestJSONField_FilterSortGroup(t *testing.T) {
	repo := newJSONRepo[profiledPet, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	for _, p := range []profiledPet{
		{Name: "Rex", Profile: r3.NewJSONColumn(petProfile{Breed: "beagle", Age: 9, Vaccinations: map[string]int{"count": 3}})},
		{Name: "Bo", Profile: r3.NewJSONColumn(petProfile{Breed: "pug", Age: 10})},
		{Name: "Max", Profile: r3.NewJSONColumn(petProfile{Breed: "beagle", Age: 2})},
		{Name: "Nox", Profile: r3.NewJSONColumn(petProfile{})},
	} {
		_, err := repo.Create(ctx, p)
		require.NoError(t, err)
	}
	names := func(pets []profiledPet) []string {
		out := make([]string, len(pets))
		for i, p := range pets {
			out[i] = p.Name
		}
		return out
	}

	breed := r3.JSONField("profile", "breed")
	age := r3.JSONField("profile", "age").AsNumber()

	got, _, err := repo.List(ctx, r3.Query{
		Filters: r3.Filters{r3.F(breed, "beagle")},
		Sorts:   r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("name"))},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"Max", "Rex"}, names(got))

	// Numeric: 10 sorts after 9 (text order would put "10" first).
	got, _, err = repo.List(ctx, r3.Query{
		Filters: r3.Filters{r3.Fop(age, r3.OperatorGt, 5)},
		Sorts:   r3.Sorts{r3.NewSortDescSpec(age)},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"Bo", "Rex"}, names(got))

	got, _, err = repo.List(ctx, r3.Query{
		Filters: r3.Filters{r3.Fop(r3.JSONField("profile", "vaccinations", "count").AsNumber(), r3.OperatorEq, 3)},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"Rex"}, names(got))

	// A missing key is NULL: Eq never matches it, Eq(nil) does.
	got, _, err = repo.List(ctx, r3.Query{Filters: r3.Filters{r3.F(breed, nil)}})
	require.NoError(t, err)
	assert.Equal(t, []string{"Nox"}, names(got))

	_, _, err = repo.List(ctx, r3.Query{Filters: r3.Filters{r3.F(breed, 3)}})
	require.ErrorIs(t, err, r3.ErrJSONFieldValueType)

	rows, err := repo.Aggregate(ctx, r3.Query{
		Filters:    r3.Filters{r3.Fop(breed, r3.OperatorExists, true)},
		GroupBy:    r3.Fields{breed},
		Aggregates: r3.Aggregates{r3.AggCount("n")},
		Sorts:      r3.Sorts{r3.NewSortAscSpec(breed)},
	})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "beagle", rows[0][breed.String()])
	assert.EqualValues(t, 2, rows[0]["n"])
}
