package r3bson

import (
	"regexp"
	"strings"
)

// regexMetaChars matches regex special characters that need escaping.
var regexMetaChars = regexp.MustCompile(`[.+*?^${}()|[\]\\]`)

// likeToRegex converts a SQL LIKE pattern to a MongoDB-compatible regex pattern.
//
// SQL LIKE uses:
//   - % to match any sequence of characters (→ .*)
//   - _ to match any single character (→ .)
//
// All regex meta-characters in the literal parts are escaped first.
func likeToRegex(like string) string {
	// First, escape regex meta-chars (but NOT the SQL wildcards % and _)
	// We need to be careful: replace % and _ with placeholders first.
	const pctPlaceholder = "\x00PCT\x00"
	const usPlaceholder = "\x00US\x00"

	s := strings.ReplaceAll(like, "%", pctPlaceholder)
	s = strings.ReplaceAll(s, "_", usPlaceholder)

	// Escape all regex metacharacters in the literal parts
	s = regexMetaChars.ReplaceAllString(s, `\$0`)

	// Replace placeholders with regex equivalents
	s = strings.ReplaceAll(s, pctPlaceholder, ".*")
	s = strings.ReplaceAll(s, usPlaceholder, ".")

	// Anchor the pattern
	return "^" + s + "$"
}
