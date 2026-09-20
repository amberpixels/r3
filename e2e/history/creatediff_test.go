package history_test

import (
	"context"
	"strconv"
	"testing"

	r3gorm "github.com/amberpixels/r3/drivers/gorm"
	"github.com/amberpixels/r3/features/history"
	"github.com/expectto/be"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Invoice carries a readonly number the driver omits from the insert so the DB
// default fills it, the shape that surfaced the create-diff bug.
type Invoice struct {
	ID     int64  `r3:"id,pk"`
	Amount int    `r3:"amount"`
	Number string `r3:"number,readonly" gorm:"default:'AUTO-1'"`
}

func setupInvoiceRepo(t *testing.T) (*history.CRUD[Invoice, int64], *changeRecordStore) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	be.NoError(t, err)
	be.NoError(t, db.AutoMigrate(&Invoice{}))

	store := newChangeRecordStore()
	repo := history.WithHistory[Invoice, int64](
		r3gorm.NewGormCRUD[Invoice, int64](db), store,
		history.WithIDFunc[Invoice, int64](func(inv Invoice) int64 { return inv.ID }),
	)
	return repo, store
}

// history diffs whatever Create returns. While that was the caller's input, a
// column the driver omitted so the DB could fill it was filed at its input value
// (here, a number the caller never got to choose), and the audit trail recorded
// something that was never stored.
func TestCRUD_CreateRecordsPersistedValueNotTheInput(t *testing.T) {
	repo, store := setupInvoiceRepo(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, Invoice{Amount: 42, Number: "ignored"})
	be.NoError(t, err)
	be.RequireThat(t, created.Number, be.Eq("AUTO-1"))

	records := store.forRecord("invoices", strconv.FormatInt(created.ID, 10))
	be.RequireThat(t, len(records), be.Eq(1))

	var number string
	for _, c := range records[0].Changes.Val {
		if c.Field == "number" {
			number, _ = c.NewValue.(string)
		}
	}
	be.AssertThat(t, number, be.Eq("AUTO-1"))
}
