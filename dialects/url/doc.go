// Package r3url provides bidirectional conversion between r3 query types and URL query parameters.
//
// Category: Serialization dialect.
//
// It supports two parsing modes:
//
//   - Unified mode: the entire query is passed as a single JSON parameter (?query={...}),
//     using the same schema as the r3json dialect.
//   - Decomposed mode: each r3 component is a separate URL parameter
//     (?fields=a,b&filters=[...]&sort=name:asc&page=2&page_size=25).
//
// The default behavior (ModeAuto) automatically detects which mode to use:
// if a "query" parameter is present, unified mode is used; otherwise, decomposed mode.
//
// All parameter names, sort formats, and filter styles are configurable via functional options.
// Django-style shorthand filters (?status=active&age__gte=18) are supported as an opt-in feature.
package r3url
