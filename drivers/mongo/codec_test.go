package r3mongo_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/amberpixels/r3"
	r3mongo "github.com/amberpixels/r3/drivers/mongo"
)

// Meeting stores its StartedAt time.Time as a unix-seconds int via a value codec.
// The bson name ("started_at") and the r3-derived schema name agree, so the codec
// resolves on every path.
type Meeting struct {
	ID        bson.ObjectID `bson:"_id"`
	Title     string        `bson:"title"`
	StartedAt time.Time     `bson:"started_at" r3:"codec:unixtime"`
}

// TestMongoUnknownCodecPanics asserts that an unregistered codec name fails loudly
// at construction (like every other backend), rather than silently storing the
// un-encoded value. It needs no database: the panic is raised during reflection.
func TestMongoUnknownCodecPanics(t *testing.T) {
	type BadCodec struct {
		ID bson.ObjectID `bson:"_id"`
		At time.Time     `bson:"at"  r3:"codec:definitely_not_registered"`
	}
	defer func() {
		r := recover()
		require.NotNil(t, r, "an unregistered codec name must panic at construction")
		err, ok := r.(error)
		require.True(t, ok, "panic value must be an error")
		require.ErrorIs(t, err, r3.ErrUnknownCodec)
	}()
	// The collection is never dereferenced before the panic, so nil is fine.
	r3mongo.NewMongoCRUD[BadCodec, bson.ObjectID](nil)
}

func TestMongoValueCodecs(t *testing.T) {
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

	// Constructing a codec'd repo must no longer panic (RequireCodecSupport is gone).
	repo := r3mongo.NewMongoCRUD[Meeting, bson.ObjectID](db.Collection("meetings"))

	base := time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)
	seed := map[string]Meeting{}
	for _, m := range []Meeting{
		{Title: "kickoff", StartedAt: base},
		{Title: "standup", StartedAt: base.Add(24 * time.Hour)},
		{Title: "review", StartedAt: base.Add(48 * time.Hour)},
	} {
		got, err := repo.Create(ctx, m)
		require.NoError(t, err, "create %s", m.Title)
		seed[got.Title] = got
	}

	t.Run("write encodes to int, read decodes to time.Time", func(t *testing.T) {
		got, err := repo.Get(ctx, seed["kickoff"].ID)
		require.NoError(t, err)
		assert.True(t, base.Equal(got.StartedAt), "round-trip time must match, got %v", got.StartedAt)

		// The stored column is a raw int64 (unix seconds), not a BSON date.
		var stored bson.M
		require.NoError(
			t,
			repo.Collection().FindOne(ctx, bson.D{{Key: "_id", Value: seed["kickoff"].ID}}).Decode(&stored),
		)
		raw, ok := stored["started_at"]
		require.True(t, ok, "started_at must be present")
		asInt, ok := raw.(int64)
		require.True(t, ok, "started_at must be stored as int64, got %T", raw)
		assert.Equal(t, base.Unix(), asInt)
	})

	t.Run("filter by a time bound compares against the int column", func(t *testing.T) {
		// StartedAt > base means standup and review (base+24h, base+48h).
		got, _, err := repo.List(ctx, r3.Query{
			Filters: r3.Filters{r3.Gt("started_at", base)},
		})
		require.NoError(t, err)
		assert.Len(t, got, 2)

		between, _, err := repo.List(ctx, r3.Query{
			Filters: r3.Filters{r3.Between("started_at", base, base.Add(24*time.Hour))},
		})
		require.NoError(t, err)
		assert.Len(t, between, 2, "kickoff and standup are within [base, base+24h]")
	})

	t.Run("sort and List decode all codec'd rows", func(t *testing.T) {
		got, _, err := repo.List(ctx, r3.Query{
			Sorts: r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("started_at"))},
		})
		require.NoError(t, err)
		require.Len(t, got, 3)
		assert.True(t, base.Equal(got[0].StartedAt))
		assert.True(t, base.Add(48*time.Hour).Equal(got[2].StartedAt))
	})

	t.Run("Patch on a codec'd field round-trips", func(t *testing.T) {
		newStart := base.Add(72 * time.Hour)
		patched, err := repo.Patch(ctx,
			Meeting{ID: seed["review"].ID, StartedAt: newStart},
			r3.Fields{r3.NewFieldSpec("started_at")},
		)
		require.NoError(t, err)
		assert.True(t, newStart.Equal(patched.StartedAt))

		got, err := repo.Get(ctx, seed["review"].ID)
		require.NoError(t, err)
		assert.True(t, newStart.Equal(got.StartedAt))
	})

	t.Run("cursor pagination encodes the codec'd keyset key", func(t *testing.T) {
		// After started_at=base, ascending, limit 1 -> standup (base+24h). The
		// cursor key is a time.Time that must be encoded to the stored int.
		token, err := r3.EncodeCursor(r3.CursorValues{"started_at": base})
		require.NoError(t, err)
		page, _, err := repo.List(ctx, r3.Query{
			Sorts:  r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("started_at"))},
			Cursor: r3.NewCursorAfter(token, 1),
		})
		require.NoError(t, err)
		require.Len(t, page, 1)
		assert.Equal(t, "standup", page[0].Title)
	})

	t.Run("zero time encodes to null and reads back as the zero value", func(t *testing.T) {
		created, err := repo.Create(ctx, Meeting{Title: "unscheduled"})
		require.NoError(t, err)
		assert.True(t, created.StartedAt.IsZero())

		// The stored column is null (the codec's NULL contract for a zero time).
		var stored bson.M
		require.NoError(t, repo.Collection().FindOne(ctx, bson.D{{Key: "_id", Value: created.ID}}).Decode(&stored))
		assert.Nil(t, stored["started_at"], "zero time must store as null")

		got, err := repo.Get(ctx, created.ID)
		require.NoError(t, err)
		assert.True(t, got.StartedAt.IsZero(), "null must read back as the zero time")
	})

	t.Run("aggregate MAX over a codec'd field decodes to time.Time", func(t *testing.T) {
		rows, err := repo.Aggregate(ctx, r3.Query{
			Aggregates: r3.Aggregates{r3.AggMax("started_at", "latest")},
		})
		require.NoError(t, err)
		require.Len(t, rows, 1)
		latest, ok := rows[0].Time("latest")
		require.True(t, ok, "MAX(started_at) must decode to a time.Time")
		// review was patched to base+72h, the latest.
		assert.True(t, base.Add(72*time.Hour).Equal(latest), "got %v", latest)
	})

	t.Run("relation aggregate encodes a codec'd owner filter", func(t *testing.T) {
		// A has-many owner whose owner filter references a codec'd field: the bound
		// must be encoded to the stored int, else it matches no owner.
		type Shift struct {
			ID        bson.ObjectID `bson:"_id"`
			StartedAt time.Time     `bson:"started_at" r3:"codec:unixtime"`
		}
		type ShiftLog struct {
			ID      bson.ObjectID `bson:"_id"`
			ShiftID bson.ObjectID `bson:"shift_id"`
		}
		shiftRepo := r3mongo.NewMongoCRUD[Shift, bson.ObjectID](db.Collection("shifts"),
			r3.WithRelations(r3.HasManyRelation("logs", "shift_logs", "shift_id")))
		logRepo := r3mongo.NewMongoCRUD[ShiftLog, bson.ObjectID](db.Collection("shift_logs"))

		early, err := shiftRepo.Create(ctx, Shift{StartedAt: base})
		require.NoError(t, err)
		late, err := shiftRepo.Create(ctx, Shift{StartedAt: base.Add(48 * time.Hour)})
		require.NoError(t, err)

		// early gets 1 log, late gets 2.
		for _, sid := range []bson.ObjectID{early.ID, late.ID, late.ID} {
			_, err := logRepo.Create(ctx, ShiftLog{ShiftID: sid})
			require.NoError(t, err)
		}

		// Owner filter on the codec'd field: only the late shift's logs (2).
		rows, err := shiftRepo.AggregateThroughRelation(ctx, "logs", r3.Query{
			Filters:    r3.Filters{r3.Gt("started_at", base.Add(24*time.Hour))},
			Aggregates: r3.Aggregates{r3.AggCount("n")},
		})
		require.NoError(t, err)
		require.Len(t, rows, 1)
		n, _ := rows[0].Int64("n")
		assert.Equal(t, int64(2), n, "only the late shift's logs, via an encoded owner filter")
	})
}
