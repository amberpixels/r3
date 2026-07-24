package r3bun

import (
	"context"
	"database/sql"

	"github.com/uptrace/bun"

	"github.com/amberpixels/r3"
)

// bunTxCRUD is a transactional Bun CRUD that wraps a bun.Tx.
type bunTxCRUD[T any, ID comparable] struct {
	*BunCRUD[T, ID]

	tx bun.Tx
}

var (
	_ r3.Transactor[any, any] = &BunCRUD[any, any]{}
	_ r3.TxCRUD[any, any]     = &bunTxCRUD[any, any]{}
)

func (t *bunTxCRUD[T, ID]) Commit() error   { return t.tx.Commit() }
func (t *bunTxCRUD[T, ID]) Rollback() error { return t.tx.Rollback() }

// BeginTx starts a Bun transaction and returns an [r3.TxCRUD] scoped to it.
// The caller must call Commit or Rollback on the returned TxCRUD.
func (r *BunCRUD[T, ID]) BeginTx(ctx context.Context) (r3.TxCRUD[T, ID], error) {
	if r.sqlDB == nil {
		return nil, r3.ErrTransactionsNotSupported
	}

	tx, err := r.sqlDB.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, err
	}

	txCrud := &BunCRUD[T, ID]{
		db:     tx,
		sqlDB:  nil,                  // signal that this is a tx-mode BunCRUD
		raw:    NewBunRaw[T, ID](tx), // Raw bound to the tx, not the root DB
		Config: r.Config,
	}
	txCrud.DefaultsManager = r3.NewDefaultsManager()
	txCrud.SetDefaultListQuery(r.GetDefaultListQuery())
	txCrud.SetDefaultGetQuery(r.GetDefaultGetQuery())

	return &bunTxCRUD[T, ID]{
		BunCRUD: txCrud,
		tx:      tx,
	}, nil
}
