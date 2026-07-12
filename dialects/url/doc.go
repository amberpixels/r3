// Package r3url is the serialization dialect between r3 query types and URL query
// parameters. Two parsing modes:
//
//   - Unified: the whole query is one JSON parameter (?query={...}), same schema as r3json.
//   - Decomposed: one URL parameter per r3 component
//     (?fields=a,b&filters=[...]&sort=name:asc&page=2&page_size=25).
//
// [ModeAuto] (default) picks unified when a "query" param is present, else decomposed.
// Param names, sort formats, and filter styles are configurable via [Option]. Django-style
// shorthand filters (?status=active&age__gte=18) are opt-in ([WithDjangoStyleFilters]).
package r3url
