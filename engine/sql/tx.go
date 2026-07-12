package enginesql

import (
	"context"
	"database/sql"
	"errors"

	"github.com/amberpixels/r3"
)

// baseTxCRUD wraps a BaseCRUD operating within a *sql.Tx and adds
// Commit/Rollback to satisfy [r3.TxCRUD].
type baseTxCRUD[T any, ID comparable] struct {
	*BaseCRUD[T, ID]
	tx *sql.Tx
}

var _ r3.TxCRUD[any, any] = &baseTxCRUD[any, any]{}

func (t *baseTxCRUD[T, ID]) Commit() error   { return t.tx.Commit() }
func (t *baseTxCRUD[T, ID]) Rollback() error { return t.tx.Rollback() }

// BeginTx starts a transaction and returns an [r3.TxCRUD] running all ops within
// it; the caller must Commit or Rollback. Errors if this BaseCRUD is already
// inside a transaction (holds no *sql.DB).
func (r *BaseCRUD[T, ID]) BeginTx(ctx context.Context) (r3.TxCRUD[T, ID], error) {
	if r.sqlDB == nil {
		return nil, errors.New("r3/engine/sql: BeginTx called on a BaseCRUD that is already inside a transaction")
	}

	tx, err := r.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	txCrud := &BaseCRUD[T, ID]{
		Executor: tx,
		sqlDB:    nil, // nil marks a tx-mode BaseCRUD
		Meta:     r.Meta,
		Flavor:   r.Flavor,
		Config:   r.Config,
		Raw:      NewBaseRaw[T, ID](tx, r.Meta),
	}
	// Fresh DefaultsManager (its sync.RWMutex can't be copied), snapshotting the
	// parent's defaults.
	txCrud.DefaultsManager = NewDefaultsManager()
	txCrud.SetDefaultListQuery(r.GetDefaultListQuery())
	txCrud.SetDefaultGetQuery(r.GetDefaultGetQuery())

	return &baseTxCRUD[T, ID]{
		BaseCRUD: txCrud,
		tx:       tx,
	}, nil
}
