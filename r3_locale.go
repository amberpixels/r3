package r3

import "context"

// Locale is carried in context.Context and read back by locale-aware features
// (see features/i18n) to serve entity content in the caller's language. It is
// a plain language tag string (e.g. "ro", "ru", "en", or any BCP-47 tag the
// application chooses) — R3 itself never interprets it.
//
// An empty locale means "no preference": locale-aware features pass entities
// through untouched.

type localeContextKey struct{}

// WithLocale returns a new context with the given locale tag attached.
// Typically called in HTTP middleware after language negotiation:
//
//	ctx := r3.WithLocale(r.Context(), "ru")
func WithLocale(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, localeContextKey{}, locale)
}

// GetLocale retrieves the locale tag from the context.
// Returns "" if no locale was set.
func GetLocale(ctx context.Context) string {
	if l, ok := ctx.Value(localeContextKey{}).(string); ok {
		return l
	}
	return ""
}
