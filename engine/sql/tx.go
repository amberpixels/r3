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

// Compile-time checks.
var _ r3.TxCRUD[any, any] = &baseTxCRUD[any, any]{}

func (t *baseTxCRUD[T, ID]) Commit() error   { return t.tx.Commit() }
func (t *baseTxCRUD[T, ID]) Rollback() error { return t.tx.Rollback() }

// BeginTx starts a database transaction and returns an [r3.TxCRUD] that executes
// all CRUD operations within that transaction. The caller must call Commit or
// Rollback on the returned TxCRUD.
//
// This method requires the BaseCRUD to hold a *sql.DB (i.e., not already inside
// a transaction). Returns an error if called on a transactional BaseCRUD.
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
		sqlDB:    nil, // signal that this is a tx-mode BaseCRUD
		Meta:     r.Meta,
		Flavor:   r.Flavor,
		Config:   r.Config,
		Raw:      NewBaseRaw[T, ID](tx, r.Meta),
	}
	// Initialize a fresh DefaultsManager (can't copy because of sync.RWMutex)
	// and snapshot the current defaults from the parent.
	txCrud.DefaultsManager = NewDefaultsManager()
	txCrud.SetDefaultListQuery(r.GetDefaultListQuery())
	txCrud.SetDefaultGetQuery(r.GetDefaultGetQuery())

	return &baseTxCRUD[T, ID]{
		BaseCRUD: txCrud,
		tx:       tx,
	}, nil
}
