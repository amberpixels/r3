package r3bson

import (
	"regexp"
	"strings"
)

// regexMetaChars matches regex special characters that need escaping.
var regexMetaChars = regexp.MustCompile(`[.+*?^${}()|[\]\\]`)

// likeToRegex converts a SQL LIKE pattern to an anchored MongoDB regex: % to .*
// and _ to . The wildcards are stashed as placeholders so the literal parts can
// be escaped without touching them, then restored.
func likeToRegex(like string) string {
	const pctPlaceholder = "\x00PCT\x00"
	const usPlaceholder = "\x00US\x00"

	s := strings.ReplaceAll(like, "%", pctPlaceholder)
	s = strings.ReplaceAll(s, "_", usPlaceholder)

	s = regexMetaChars.ReplaceAllString(s, `\$0`)

	s = strings.ReplaceAll(s, pctPlaceholder, ".*")
	s = strings.ReplaceAll(s, usPlaceholder, ".")

	return "^" + s + "$"
}
