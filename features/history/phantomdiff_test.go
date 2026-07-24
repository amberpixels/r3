package history_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/expectto/be"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	r3gorm "github.com/amberpixels/r3/drivers/gorm"
	"github.com/amberpixels/r3/features/history"
)

// Ticket carries a system-managed created_at, the shape that surfaced the
// phantom diff: a partial model built from a request DTO does not carry it.
type Ticket struct {
	ID        int64  `r3:"id,pk"`
	Title     string `r3:"title"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// A driver-backed history test: the phantom diff only appears against a backend
// that shapes its writes, which no in-memory mock does.
func setupTicketRepo(t *testing.T) (*history.CRUD[Ticket, int64], *memoryChangeRecordCRUD) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	be.NoError(t, err)
	be.NoError(t, db.AutoMigrate(&Ticket{}))

	store := newMemoryChangeRecordCRUD()
	repo := history.WithHistory[Ticket, int64](
		r3gorm.NewGormCRUD[Ticket, int64](db), store,
		history.WithIDFunc[Ticket, int64](func(tk Ticket) int64 { return tk.ID }),
	)
	return repo, store
}

// H11 end to end: updating from a partial model must not record a created_at
// change. The driver omits created_at from the write, so the row keeps it; when
// Update returned the caller's input, history diffed a zero created_at against
// the stored one and filed a phantom "created_at: <real> -> 0001-01-01" on every
// such save.
func TestCRUD_UpdateFromPartialModelRecordsNoPhantomDiff(t *testing.T) {
	repo, store := setupTicketRepo(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, Ticket{Title: "a"})
	be.NoError(t, err)
	be.RequireThat(t, created.CreatedAt.IsZero(), be.False())

	// The partial model a form save builds: everything the request carried, and
	// nothing it didn't.
	updated, err := repo.Update(ctx, Ticket{ID: created.ID, Title: "b"})
	be.NoError(t, err)
	be.AssertThat(t, updated.Title, be.Eq("b"))
	be.AssertThat(t, updated.CreatedAt.IsZero(), be.False())

	records, count, err := listForRecord(ctx, store, "tickets", strconv.FormatInt(created.ID, 10))
	be.NoError(t, err)
	be.RequireThat(t, count, be.Eq(int64(2)))

	changed := make([]string, 0, len(records[1].Changes.Val))
	for _, c := range records[1].Changes.Val {
		changed = append(changed, c.Field)
	}
	be.AssertThat(t, changed, be.Not(be.ContainElement("created_at")))
	be.AssertThat(t, changed, be.ContainElement("title"))
}
