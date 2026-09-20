package r3_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/amberpixels/r3"
)

type kv struct {
	Key   string
	Value string
}

// bareCommander implements r3.Commander but neither Upserter nor BulkPatcher, so
// the capability helpers must return the "not supported" sentinels for it.
type bareCommander struct{}

func (bareCommander) Create(context.Context, kv) (kv, error)           { return kv{}, nil }
func (bareCommander) Update(context.Context, kv) (kv, error)           { return kv{}, nil }
func (bareCommander) Patch(context.Context, kv, r3.Fields) (kv, error) { return kv{}, nil }
func (bareCommander) Delete(context.Context, string) error             { return nil }

// upserter adds the Upserter/BulkPatcher capabilities, recording what it saw.
type capCommander struct {
	bareCommander

	gotSpec    r3.UpsertSpec
	gotFilters r3.Filters
	gotFields  r3.Fields
}

func (c *capCommander) Upsert(_ context.Context, entity kv, opts ...r3.UpsertOption) (kv, error) {
	c.gotSpec = r3.NewUpsertSpec(opts...)
	return entity, nil
}

func (c *capCommander) PatchWhere(
	_ context.Context, filters r3.Filters, _ kv, fields r3.Fields,
) (int64, error) {
	c.gotFilters = filters
	c.gotFields = fields
	return 7, nil
}

func TestUpsertOf_NotSupported(t *testing.T) {
	_, err := r3.UpsertOf[kv, string](context.Background(), bareCommander{}, kv{Key: "k"})
	require.ErrorIs(t, err, r3.ErrUpsertNotSupported)
}

func TestPatchWhereOf_NotSupported(t *testing.T) {
	_, err := r3.PatchWhereOf[kv, string](
		context.Background(), bareCommander{}, r3.Filters{r3.Eq("value", "x")}, kv{}, nil,
	)
	require.ErrorIs(t, err, r3.ErrBulkPatchNotSupported)
}

func TestUpsertOf_ForwardsAndResolvesOptions(t *testing.T) {
	c := &capCommander{}
	got, err := r3.UpsertOf[kv, string](
		context.Background(), c, kv{Key: "k", Value: "v"},
		r3.OnConflict("key"),
		r3.UpdateOnConflict(r3.NewFieldSpec("value")),
	)
	require.NoError(t, err)
	require.Equal(t, "v", got.Value)
	require.Equal(t, []string{"key"}, c.gotSpec.ConflictColumns)
	require.Equal(t, []string{"value"}, r3.FieldsToStrings(c.gotSpec.UpdateFields))
}

func TestPatchWhereOf_Forwards(t *testing.T) {
	c := &capCommander{}
	n, err := r3.PatchWhereOf[kv, string](
		context.Background(), c,
		r3.Filters{r3.Eq("value", "old")}, kv{Value: "new"},
		r3.Fields{r3.NewFieldSpec("value")},
	)
	require.NoError(t, err)
	require.Equal(t, int64(7), n)
	require.Len(t, c.gotFilters, 1)
	require.Equal(t, []string{"value"}, r3.FieldsToStrings(c.gotFields))
}

func TestNewUpsertSpec_Defaults(t *testing.T) {
	spec := r3.NewUpsertSpec()
	require.Nil(t, spec.ConflictColumns, "default conflict target is empty (means PK)")
	require.Nil(t, spec.UpdateFields, "default update set is empty (means all mutable)")
}

// The overlap rule lives in core so all four backends mean the same thing by a
// malformed spec, rather than each applying whichever assignment lands last.
func TestUpsertSpecValidate(t *testing.T) {
	hits := r3.NewFieldSpec("hits")
	seen := r3.NewFieldSpec("last_seen")

	t.Run("a column in both sets is rejected", func(t *testing.T) {
		spec := r3.NewUpsertSpec(r3.UpdateOnConflict(hits), r3.IncrementOnConflict(hits))
		require.ErrorIs(t, spec.Validate(), r3.ErrUpsertIncrementConflict)
	})

	t.Run("disjoint sets are fine", func(t *testing.T) {
		spec := r3.NewUpsertSpec(r3.UpdateOnConflict(seen), r3.IncrementOnConflict(hits))
		require.NoError(t, spec.Validate())
	})

	t.Run("either set alone is fine", func(t *testing.T) {
		require.NoError(t, r3.NewUpsertSpec(r3.IncrementOnConflict(hits)).Validate())
		require.NoError(t, r3.NewUpsertSpec(r3.UpdateOnConflict(hits)).Validate())
		require.NoError(t, r3.NewUpsertSpec().Validate())
	})
}
