package enginesql_test

import (
	"testing"

	enginesql "github.com/amberpixels/r3/engine/sql"
	"github.com/expectto/be"
)

// TestQuoteIdentifiers verifies that the dialect's ANSI double-quoted identifiers
// are rewritten to each flavor's identifier quote. This is the fix for the bug
// where filtering/sorting by column silently broke on MySQL: without ANSI_QUOTES,
// MySQL reads `"col"` as a string literal, so `WHERE "visible" = ?` matches
// nothing. MySQL must receive backtick-quoted identifiers instead.
func TestQuoteIdentifiers(t *testing.T) {
	const ansi = `SELECT "id", "name" FROM "x" WHERE "visible" = ? ORDER BY "price" ASC`

	tests := []struct {
		name   string
		flavor enginesql.Flavor
		want   string
	}{
		{
			name:   "postgres keeps ANSI double quotes",
			flavor: enginesql.FlavorPostgres,
			want:   ansi,
		},
		{
			name:   "sqlite keeps ANSI double quotes",
			flavor: enginesql.FlavorSQLite,
			want:   ansi,
		},
		{
			name:   "mysql rewrites to backticks",
			flavor: enginesql.FlavorMySQL,
			want:   "SELECT `id`, `name` FROM `x` WHERE `visible` = ? ORDER BY `price` ASC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			be.AssertThat(t, tt.flavor.QuoteIdentifiers(ansi), be.Eq(tt.want))
		})
	}
}

// TestQuoteIdentifiers_DottedAndEscaped covers dotted identifiers (joins) and the
// doubled-quote escape, which must map to the doubled target quote for MySQL.
func TestQuoteIdentifiers_DottedAndEscaped(t *testing.T) {
	be.AssertThat(t, enginesql.FlavorMySQL.QuoteIdentifiers(`"users"."name"`), be.Eq("`users`.`name`"))
	be.AssertThat(t, enginesql.FlavorMySQL.QuoteIdentifiers(`"a""b"`), be.Eq("`a``b`"))
}
