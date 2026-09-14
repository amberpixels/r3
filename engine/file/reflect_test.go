package enginefile_test

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amberpixels/r3"
	enginefile "github.com/amberpixels/r3/engine/file"
)

// docCoords is a nested value, not an entity of its own.
type docCoords struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// docPlace is the file-backed shape the type-based gate used to hide. The codec
// encodes whole entities, so these fields were always written to disk; the gate
// only kept them out of the meta, which is what decides filtering, patching and
// projection. Extra is the documented driver.Valuer escape hatch, which this
// engine's local copy of the gate dropped as well.
type docPlace struct {
	ID     int                     `json:"id"     r3:"id,pk"`
	Name   string                  `json:"name"`
	Tags   []string                `json:"tags"`
	Labels map[string]string       `json:"labels"`
	Coords docCoords               `json:"coords"`
	Extra  r3.JSONColumn[[]string] `json:"extra"`
	Items  []docCoords             `json:"items"  r3:"rel:has-many,fk:place_id"`
}

// TestStructMeta_UntaggedValueFieldsAreStored pins issue #22 for the file engine: a
// value slice, map or nested struct is a native JSON/YAML value, so it belongs
// to the meta; only a declared relation stays out.
func TestStructMeta_UntaggedValueFieldsAreStored(t *testing.T) {
	meta := enginefile.GetStructMeta[docPlace]()

	for _, name := range []string{"id", "name", "tags", "labels", "coords", "extra"} {
		assert.True(t, slices.Contains(meta.Fields, name),
			"field %q missing from meta.Fields %v", name, meta.Fields)
	}
	assert.False(t, slices.Contains(meta.Fields, "items"),
		"a declared relation was stored as a field: %v", meta.Fields)
}

func seedPlaces(t *testing.T) *enginefile.BaseCRUD[docPlace, int] {
	t.Helper()
	repo := newJSONRepo[docPlace, int](t, enginefile.IncrementIDGen[int]())
	_, err := repo.Create(context.Background(), docPlace{
		Name:   "Kyiv",
		Tags:   []string{"capital", "river"},
		Labels: map[string]string{"tier": "a"},
		Coords: docCoords{Lat: 50.45, Lon: 30.52},
	})
	require.NoError(t, err)
	return repo
}

// TestProjection_ValueSliceIsProjectable closes the leak: a projection that drops
// a value slice used to reject the name as unknown and hand the field back in
// full, which is the opposite of what the other engines do with the same query.
func TestProjection_ValueSliceIsProjectable(t *testing.T) {
	ctx := context.Background()
	repo := seedPlaces(t)

	got, _, err := repo.List(ctx, r3.Query{ExcludeFields: r3.Exclude("tags")})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Nil(t, got[0].Tags, "excluded value slice was handed back anyway")
	assert.Equal(t, "Kyiv", got[0].Name)

	kept, _, err := repo.List(ctx, r3.Query{Fields: r3.Include("tags")})
	require.NoError(t, err)
	require.Len(t, kept, 1)
	assert.Equal(t, []string{"capital", "river"}, kept[0].Tags)
	assert.Empty(t, kept[0].Name, "an unnamed field survived the projection")
}

// TestPatch_ValueSlice is the other behaviour the meta gates: the field can now
// be named in a patch instead of being rejected as unknown.
func TestPatch_ValueSlice(t *testing.T) {
	ctx := context.Background()
	repo := seedPlaces(t)

	patched, err := repo.Patch(ctx, docPlace{ID: 1, Tags: []string{"port"}}, r3.Fields{r3.NewFieldSpec("tags")})
	require.NoError(t, err)
	assert.Equal(t, []string{"port"}, patched.Tags)
	assert.Equal(t, "Kyiv", patched.Name, "patch touched a field it was not given")

	got, err := repo.Get(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, []string{"port"}, got.Tags)
	assert.Equal(t, map[string]string{"tier": "a"}, got.Labels)
}

// AuditBase is embedded rather than named, which encoding/json flattens into the
// record. A flat field name cannot describe that, so it stays out of the meta.
type AuditBase struct {
	CreatedBy string `json:"created_by"`
}

// Code is an embedded scalar, which has always been an ordinary field.
type Code string

type embeddingPlace struct {
	AuditBase
	Code

	ID   int    `json:"id"   r3:"id,pk"`
	Name string `json:"name"`
}

func TestStructMeta_EmbeddedStructIsLeftToTheEncoder(t *testing.T) {
	meta := enginefile.GetStructMeta[embeddingPlace]()

	assert.False(t, slices.Contains(meta.Fields, "audit_base"),
		"an embedded struct was given a flat field name: %v", meta.Fields)
	assert.True(t, slices.Contains(meta.Fields, "code"),
		"an embedded scalar is an ordinary field: %v", meta.Fields)
}
