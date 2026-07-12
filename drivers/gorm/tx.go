package r3gorm

import (
	"context"

	"github.com/amberpixels/r3"
	"gorm.io/gorm"
)

// gormTxCRUD is a transactional GORM CRUD that wraps a *gorm.DB transaction.
type gormTxCRUD[T any, ID comparable] struct {
	*GormCRUD[T, ID]
	tx *gorm.DB
}

var _ r3.Transactor[any, any] = &GormCRUD[any, any]{}
var _ r3.TxCRUD[any, any] = &gormTxCRUD[any, any]{}

func (t *gormTxCRUD[T, ID]) Commit() error   { return t.tx.Commit().Error }
func (t *gormTxCRUD[T, ID]) Rollback() error { return t.tx.Rollback().Error }

// BeginTx starts a GORM transaction and returns an [r3.TxCRUD] scoped to it.
// The caller must call Commit or Rollback on the returned TxCRUD.
func (r *GormCRUD[T, ID]) BeginTx(ctx context.Context) (r3.TxCRUD[T, ID], error) {
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}

	txCrud := &GormCRUD[T, ID]{
		db:     tx,
		Config: r.Config,
		raw:    NewGormRaw[T, ID](tx),
	}
	// Initialize a fresh DefaultsManager and snapshot the current defaults.
	txCrud.DefaultsManager = r3.NewDefaultsManager()
	txCrud.SetDefaultListQuery(r.GetDefaultListQuery())
	txCrud.SetDefaultGetQuery(r.GetDefaultGetQuery())

	return &gormTxCRUD[T, ID]{
		GormCRUD: txCrud,
		tx:       tx,
	}, nil
}
