package i18n

import "errors"

// ErrNotTranslatable reports an invalid WithFields configuration: a requested
// field is missing from the entity struct or not a string. Raised as a panic
// (wrapping this error) at WithTranslations time - a wiring-time programming
// error, not a request-time one.
var ErrNotTranslatable = errors.New("i18n: field is not translatable")
