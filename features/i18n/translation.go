package i18n

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/google/uuid"

	"github.com/amberpixels/r3"
)

// Source records where a translation came from, so human work is never
// overwritten by machines.
type Source string

const (
	// SourceAI marks a machine translation (an LLM/MT worker). Workers may
	// freely re-translate AI rows when they go stale.
	SourceAI Source = "ai"
	// SourceHuman marks a translation written or corrected by a person.
	// Workers must not overwrite human rows - flag them for review instead.
	SourceHuman Source = "human"
	// SourceImport marks a translation that arrived with the data itself
	// (e.g. a video that ships titles in several languages).
	SourceImport Source = "import"
)

// Translation is one translated value of one field of one entity, in one
// language - a first-class r3 entity, stored via any r3.CRUD[Translation, string].
// Uniqueness is (EntityType, EntityID, Field, Lang); enforce it with a unique
// index in your storage, and Upsert respects it.
type Translation struct {
	// ID is the unique identifier for this translation row (UUID).
	ID string `json:"id" db:"id,pk" bson:"_id"`

	// EntityType names the translated entity's type (e.g. "locations",
	// "videos"). Derived from the struct type or set via WithEntityType.
	EntityType string `json:"entity_type" db:"entity_type" bson:"entity_type"`

	// EntityID is the primary key of the translated entity, stringified.
	EntityID string `json:"entity_id" db:"entity_id" bson:"entity_id"`

	// Field is the storage name of the translated field (e.g. "title") -
	// the db tag of the struct field, or its snake_cased Go name.
	Field string `json:"field" db:"field" bson:"field"`

	// Lang is the language tag of Value (matches r3.GetLocale values).
	Lang string `json:"lang" db:"lang" bson:"lang"`

	// Value is the translated text. An empty Value is treated as "no
	// translation" and never overlaid.
	Value string `json:"value" db:"value" bson:"value"`

	// ValueNorm is an optional normalized copy of Value for search (case/diacritics
	// folded as the app defines). R3 never computes or reads it; it exists so
	// translated content is searchable with the app's own normalization.
	ValueNorm string `json:"value_norm,omitempty" db:"value_norm" bson:"value_norm,omitempty"`

	// Source records who produced this translation: ai, human, or import.
	Source Source `json:"source" db:"source" bson:"source"`

	// Model optionally names the model that produced an AI translation.
	Model string `json:"model,omitempty" db:"model" bson:"model,omitempty"`

	// SourceHash is Hash() of the source text this translation was made
	// from. The decorator compares it against the current source text on
	// Update/Patch to detect staleness.
	SourceHash string `json:"source_hash" db:"source_hash" bson:"source_hash"`

	// Stale is set when the source text changed after this translation was
	// made. Stale translations are still served by default (see
	// WithoutStale) and form the re-translation queue for workers.
	Stale bool `json:"stale" db:"stale" bson:"stale"`

	// CreatedAt is when this translation row was first written.
	CreatedAt time.Time `json:"created_at" db:"created_at" bson:"created_at"`

	// UpdatedAt is when Value was last (re)written.
	UpdatedAt time.Time `json:"updated_at" db:"updated_at" bson:"updated_at"`
}

// Hash returns the canonical hash of a source text, used for SourceHash.
// Translation writers must use it so the decorator's staleness detection
// agrees with them.
func Hash(sourceText string) string {
	sum := sha256.Sum256([]byte(sourceText))
	return hex.EncodeToString(sum[:])
}

// Upsert writes a translation for (EntityType, EntityID, Field, Lang), updating
// the existing row or creating it. Identity fields come from tr; Value, ValueNorm,
// Source, Model, SourceHash are applied; Stale is cleared (an upsert IS the
// re-translation). Timestamps are set from time.Now.
func Upsert(ctx context.Context, store r3.CRUD[Translation, string], tr Translation) (Translation, error) {
	existing, _, err := store.List(ctx, QueryFor(tr.EntityType, tr.EntityID, tr.Lang, tr.Field))
	if err != nil {
		return Translation{}, err
	}

	now := time.Now()
	if len(existing) > 0 {
		row := existing[0]
		row.Value = tr.Value
		row.ValueNorm = tr.ValueNorm
		row.Source = tr.Source
		row.Model = tr.Model
		row.SourceHash = tr.SourceHash
		row.Stale = false
		row.UpdatedAt = now
		return store.Update(ctx, row)
	}

	tr.ID = uuid.NewString()
	tr.Stale = false
	tr.CreatedAt = now
	tr.UpdatedAt = now
	return store.Create(ctx, tr)
}
