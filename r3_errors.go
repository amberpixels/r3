package r3

import "errors"

// ErrNotFound is returned by Get (and other single-record operations) when no
// record matches the requested ID.
//
// Every backend normalizes its native "no rows" / "no documents" error to this
// sentinel — database/sql's sql.ErrNoRows, GORM's gorm.ErrRecordNotFound,
// MongoDB's mongo.ErrNoDocuments, and the file engine's internal not-found
// error all surface as r3.ErrNotFound. This lets business code detect a missing
// record identically regardless of the concrete driver:
//
//	user, err := repo.Get(ctx, id)
//	if errors.Is(err, r3.ErrNotFound) {
//	    // respond 404, etc.
//	}
var ErrNotFound = errors.New("r3: record not found")
