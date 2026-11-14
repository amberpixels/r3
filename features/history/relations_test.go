package history_test

import (
	"testing"

	"github.com/expectto/be"

	"github.com/amberpixels/r3/features/history"
)

type relPhoto struct {
	ID  int64  `r3:"id,pk"`
	URL string `r3:"url"`
}

type relTag struct {
	ID int64 `r3:"id,pk"`
}

type relShot struct {
	ID        int64  `r3:"id,pk"`
	OwnerID   int64  `r3:"owner_id"`
	Caption   string `r3:"caption"`
	CreatedAt int64  `r3:"created_at"`
}

type relOwner struct {
	ID      int64      `r3:"id,pk"`
	Name    string     `r3:"name"`
	Photos  []relPhoto `r3:"rel:many-to-many,join:owner_photos,fk:owner_id,ref:photo_id,order:sort_order"`
	Tags    []relTag   `r3:"rel:many-to-many,join:owner_tags,fk:owner_id,ref:tag_id"`
	Shots   []relShot  `r3:"rel:has-many,fk:owner_id,owned"`
	Loose   []relShot  `r3:"rel:has-many,fk:owner_id"`
	Aliases []string   `r3:"aliases"`
}

func changesByField(changes []history.FieldChange) map[string]history.FieldChange {
	out := make(map[string]history.FieldChange, len(changes))
	for _, c := range changes {
		out[c.Field] = c
	}
	return out
}

func TestDiff_RelationMembers(t *testing.T) {
	old := relOwner{
		ID: 1, Photos: []relPhoto{{ID: 1}, {ID: 2}}, Tags: []relTag{{ID: 1}, {ID: 2}},
		Shots:   []relShot{{ID: 10, OwnerID: 1, Caption: "a", CreatedAt: 5}},
		Loose:   []relShot{{ID: 1}},
		Aliases: []string{"x"},
	}
	cur := relOwner{
		ID: 1, Photos: []relPhoto{{ID: 2}, {ID: 1}}, Tags: []relTag{{ID: 2}, {ID: 1}},
		Shots:   []relShot{{ID: 99, OwnerID: 1, Caption: "a", CreatedAt: 7}},
		Loose:   []relShot{{ID: 2}},
		Aliases: []string{"y"},
	}

	changes := changesByField(history.Diff(old, cur))

	// Ordered M2M: a pure reorder is a change, recorded as PK lists.
	be.AssertThat(t, changes["photos"].OldValue, be.Eq(any([]any{int64(1), int64(2)})))
	be.AssertThat(t, changes["photos"].NewValue, be.Eq(any([]any{int64(2), int64(1)})))
	// Unordered M2M, owned children differing only by PK/FK/timestamps,
	// non-owned has-many and undeclared slices: nothing.
	be.AssertThat(t, len(changes), be.Eq(1))
}

func TestDiff_RelationNilIsNotLoaded(t *testing.T) {
	loaded := relOwner{ID: 1, Photos: []relPhoto{{ID: 1}}}
	unloaded := relOwner{ID: 1}

	be.AssertThat(t, len(history.Diff(loaded, unloaded)), be.Eq(0))
	be.AssertThat(t, len(history.Diff(unloaded, loaded)), be.Eq(0))

	cleared := changesByField(history.Diff(loaded, relOwner{ID: 1, Photos: []relPhoto{}}))
	be.AssertThat(t, cleared["photos"].NewValue, be.Eq(any([]any{})))
}

func TestDiffCreateDelete_Relations(t *testing.T) {
	entity := relOwner{ID: 1, Shots: []relShot{{ID: 3, OwnerID: 1, Caption: "a"}}}

	created := changesByField(history.DiffCreate(entity))
	be.AssertThat(t, created["shots"].NewValue, be.Eq(any([]any{map[string]any{"caption": "a"}})))
	_, hasPhotos := created["photos"]
	be.AssertThat(t, hasPhotos, be.False())

	deleted := changesByField(history.DiffDelete(entity))
	be.AssertThat(t, deleted["shots"].NewValue, be.Nil())
}
