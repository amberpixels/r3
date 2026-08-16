package enginemongo_test

import (
	"errors"
	"testing"

	"github.com/expectto/be"

	"github.com/amberpixels/r3"
	enginemongo "github.com/amberpixels/r3/engine/mongo"
)

// run names its fields the way a Mongo model does: through bson tags, which
// r3.SchemaOf never reads. The engine's own meta is therefore the only thing
// that can tell a real field from a typo.
type run struct {
	ID      string `bson:"_id"          r3:"pk"`
	Name    string `bson:"name"`
	Streams string `bson:"streams_json"`
}

// aliasedID keeps its identity under a name that is not _id, which the stateless
// BSON dialect cannot recognize.
type aliasedID struct {
	Code string `bson:"code" r3:"pk"`
	Blob string `bson:"blob"`
}

func TestResolveProjection(t *testing.T) {
	meta := enginemongo.GetStructMeta[run]()

	t.Run("a known excluded field passes through", func(t *testing.T) {
		got, err := meta.ResolveProjection(r3.Query{ExcludeFields: r3.Exclude("streams_json")})
		be.NoError(t, err)
		be.AssertThat(t, r3.FieldsToStrings(got.ExcludeFields), be.Eq([]string{"streams_json"}))
	})

	t.Run("an unknown excluded field is a typo, not a no-op", func(t *testing.T) {
		_, err := meta.ResolveProjection(r3.Query{ExcludeFields: r3.Exclude("streams")})
		be.AssertThat(t, errors.Is(err, r3.ErrUnknownField), be.True())
	})

	t.Run("both forms at once conflict", func(t *testing.T) {
		_, err := meta.ResolveProjection(r3.Query{
			Fields:        r3.Include("name"),
			ExcludeFields: r3.Exclude("streams_json"),
		})
		be.AssertThat(t, errors.Is(err, r3.ErrProjectionConflict), be.True())
	})

	t.Run("no projection is left alone", func(t *testing.T) {
		got, err := meta.ResolveProjection(r3.Query{})
		be.NoError(t, err)
		be.AssertThat(t, len(got.ExcludeFields), be.Eq(0))
	})
}

// An entity handed back without its identity cannot be patched, deleted, or
// linked - and the identity is not always called _id.
func TestResolveProjection_KeepsAliasedIdentity(t *testing.T) {
	meta := enginemongo.GetStructMeta[aliasedID]()
	be.RequireThat(t, meta.IDField, be.Eq("code"))

	got, err := meta.ResolveProjection(r3.Query{ExcludeFields: r3.Exclude("code", "blob")})
	be.NoError(t, err)
	be.AssertThat(t, r3.FieldsToStrings(got.ExcludeFields), be.Eq([]string{"blob"}))
}
