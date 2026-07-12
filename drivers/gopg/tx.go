package r3gopg

import (
	"context"

	"github.com/amberpixels/r3"
	"github.com/go-pg/pg/v10"
)

// gopgTxCRUD is a transactional go-pg CRUD that wraps a *pg.Tx.
type gopgTxCRUD[T any, ID comparable] struct {
	*GoPgCRUD[T, ID]
	tx *pg.Tx
}

var _ r3.Transactor[any, any] = &GoPgCRUD[any, any]{}
var _ r3.TxCRUD[any, any] = &gopgTxCRUD[any, any]{}

func (t *gopgTxCRUD[T, ID]) Commit() error   { return t.tx.Commit() }
func (t *gopgTxCRUD[T, ID]) Rollback() error { return t.tx.Rollback() }

// BeginTx starts a go-pg transaction and returns an [r3.TxCRUD] scoped to it.
// The caller must call Commit or Rollback on the returned TxCRUD.
func (r *GoPgCRUD[T, ID]) BeginTx(ctx context.Context) (r3.TxCRUD[T, ID], error) {
	if r.pgDB == nil {
		return nil, r3.ErrTransactionsNotSupported
	}

	tx, err := r.pgDB.WithContext(ctx).Begin()
	if err != nil {
		return nil, err
	}

	txCrud := &GoPgCRUD[T, ID]{
		db:     tx,
		pgDB:   nil,                   // signal that this is a tx-mode GoPgCRUD
		raw:    NewGoPgRaw[T, ID](tx), // Raw bound to the tx, not the root DB
		Config: r.Config,
	}
	txCrud.DefaultsManager = r3.NewDefaultsManager()
	txCrud.SetDefaultListQuery(r.GetDefaultListQuery())
	txCrud.SetDefaultGetQuery(r.GetDefaultGetQuery())

	return &gopgTxCRUD[T, ID]{
		GoPgCRUD: txCrud,
		tx:       tx,
	}, nil
}
