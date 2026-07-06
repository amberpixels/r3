package i18n

import "errors"

// ErrNotTranslatable reports an invalid WithFields configuration: a requested
// field does not exist on the entity struct or is not a string. Raised (as a
// panic, wrapping this error) at WithTranslations time — misconfigured
// translatable fields are a programming error, caught at wiring, not at
// request time.
var ErrNotTranslatable = errors.New("i18n: field is not translatable")
