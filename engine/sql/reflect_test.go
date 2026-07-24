package enginesql_test

import (
	"testing"
	"time"

	"github.com/expectto/be"

	enginesql "github.com/amberpixels/r3/engine/sql"
)

type reflectWidget struct {
	ID        int64     `db:"id,pk"`
	Title     string    `db:"title"`
	CreatedAt time.Time `db:"created_at"`
	Ignored   string    `db:"-"`

	Parent *reflectWidget
	Tags   []string
}

func TestCopyColumnFields(t *testing.T) {
	now := time.Now()
	parent := &reflectWidget{ID: 7}

	// dst is the entity a driver is about to return: caller-supplied relations,
	// stale or zeroed columns.
	dst := reflectWidget{
		ID:      1,
		Title:   "caller",
		Ignored: "caller",
		Parent:  parent,
		Tags:    []string{"a"},
	}
	// src is the row as read back from the database.
	src := reflectWidget{ID: 1, Title: "persisted", CreatedAt: now, Ignored: "db"}

	meta := enginesql.GetStructMeta[reflectWidget]()
	meta.CopyColumnFields(&dst, src)

	be.AssertThat(t, dst.Title, be.Eq("persisted"))
	be.AssertThat(t, dst.CreatedAt, be.Eq(now))
	// Non-column fields are left alone: relations the read-back never loaded, and
	// anything tagged out of the column set.
	be.AssertThat(t, dst.Ignored, be.Eq("caller"))
	be.AssertThat(t, dst.Parent, be.Eq(parent))
	be.AssertThat(t, dst.Tags, be.HaveLength(1))
}

func TestCopyColumnFields_NonPointerDestinationIsANoop(t *testing.T) {
	dst := reflectWidget{Title: "caller"}
	meta := enginesql.GetStructMeta[reflectWidget]()
	meta.CopyColumnFields(dst, reflectWidget{Title: "persisted"})

	be.AssertThat(t, dst.Title, be.Eq("caller"))
}
