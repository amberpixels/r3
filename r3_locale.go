package r3

import "context"

// Locale is a language tag carried in context and read back by locale-aware
// features (see features/i18n) to serve content in the caller's language. It is a
// plain string (e.g. "ru", or any BCP-47 tag); R3 never interprets it. An empty
// locale means "no preference" - those features pass entities through untouched.

type localeContextKey struct{}

// WithLocale attaches a locale tag to the context, typically after language
// negotiation in HTTP middleware:
//
//	ctx := r3.WithLocale(r.Context(), "ru")
func WithLocale(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, localeContextKey{}, locale)
}

// GetLocale returns the locale tag from the context, or "" if none is set.
func GetLocale(ctx context.Context) string {
	if l, ok := ctx.Value(localeContextKey{}).(string); ok {
		return l
	}
	return ""
}
