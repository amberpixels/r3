// Package petstore demonstrates a full CRUD JSON API server using r3 with GORM and PostgreSQL.
package petstore

import (
	"time"

	"gorm.io/gorm"
)

// Species represents a pet species (e.g. Dog, Cat, Bird).
type Species struct {
	ID   int64  `gorm:"primaryKey"           json:"id"`
	Name string `gorm:"uniqueIndex;not null" json:"name"`
}

// Pet represents a pet in the store.
type Pet struct {
	ID        int64          `gorm:"primaryKey"                   json:"id"`
	Name      string         `gorm:"not null"                     json:"name"`
	SpeciesID int64          `gorm:"not null"                     json:"species_id"`
	Species   *Species       `gorm:"foreignKey:SpeciesID"         json:"species,omitempty"`
	Status    string         `gorm:"not null;default:'available'" json:"status"` // available, pending, sold
	Age       int            `                                    json:"age"`
	Price     float64        `                                    json:"price"`
	Tags      string         `                                    json:"tags"` // comma-separated tags for simplicity
	CreatedAt time.Time      `                                    json:"created_at"`
	UpdatedAt time.Time      `                                    json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index"                        json:"-"`
}
