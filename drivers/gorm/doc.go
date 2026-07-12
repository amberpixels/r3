// Package r3gorm is an r3.CRUD[T, ID] driver backed by GORM (gorm.io/gorm). It is
// the most complete SQL driver: alongside CRUD it wires preloads (native
// Preload plus r3-managed relations), soft-delete (Unscoped, Restore, HardDelete),
// transactions, aggregation, upsert, bulk patch, relationship ("has") filters, and
// transparent value-codec support via GORM serializers.
package r3gorm
