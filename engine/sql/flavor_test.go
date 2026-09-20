package enginesql_test

import (
	"testing"

	"github.com/expectto/be"

	enginesql "github.com/amberpixels/r3/engine/sql"
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

// The accumulating conflict assignment is the one part of an upsert whose
// rendering differs per flavor, so each gets its own assertion rather than
// trusting a shared case.
func TestUpsertIncrementExpr(t *testing.T) {
	t.Run("ON CONFLICT flavors accumulate against EXCLUDED", func(t *testing.T) {
		for _, f := range []enginesql.Flavor{enginesql.FlavorPostgres, enginesql.FlavorSQLite} {
			got, err := f.UpsertIncrementExpr("counters", "hits")
			be.NoError(t, err)
			be.AssertThat(t, got, be.Eq("counters.hits + EXCLUDED.hits"),
				"the stored value is table-qualified: Postgres reads a bare name as ambiguous")
		}
	})

	t.Run("MySQL accumulates against VALUES()", func(t *testing.T) {
		got, err := enginesql.FlavorMySQL.UpsertIncrementExpr("counters", "hits")
		be.NoError(t, err)
		be.AssertThat(t, got, be.Eq("counters.hits + VALUES(hits)"))
	})

	// A zero Flavor is what a driver gets for a dialect it does not recognize.
	// Silently overwriting a counter there would reset it with nothing to notice.
	t.Run("a zero flavor degrades loudly", func(t *testing.T) {
		_, err := enginesql.Flavor{}.UpsertIncrementExpr("counters", "hits")
		be.AssertThat(t, err, be.Not(be.Nil()))
	})
}

func TestUpsertClauseWithIncrements(t *testing.T) {
	t.Run("postgres renders overwrites then accumulations", func(t *testing.T) {
		got := enginesql.FlavorPostgres.UpsertClause(
			"usage", []string{"model"}, []string{"last_seen"}, []string{"hits"})
		be.AssertThat(t, got, be.Eq(
			"ON CONFLICT (model) DO UPDATE SET last_seen = EXCLUDED.last_seen, "+
				"hits = usage.hits + EXCLUDED.hits"))
	})

	t.Run("mysql renders its own form", func(t *testing.T) {
		got := enginesql.FlavorMySQL.UpsertClause(
			"usage", []string{"model"}, []string{"last_seen"}, []string{"hits"})
		be.AssertThat(t, got, be.Eq(
			"ON DUPLICATE KEY UPDATE last_seen = VALUES(last_seen), "+
				"hits = usage.hits + VALUES(hits)"))
	})

	t.Run("increments alone still produce a DO UPDATE", func(t *testing.T) {
		got := enginesql.FlavorPostgres.UpsertClause("usage", []string{"model"}, nil, []string{"hits"})
		be.AssertThat(t, got, be.Eq("ON CONFLICT (model) DO UPDATE SET hits = usage.hits + EXCLUDED.hits"))
	})

	t.Run("neither set still means DO NOTHING", func(t *testing.T) {
		got := enginesql.FlavorPostgres.UpsertClause("usage", []string{"model"}, nil, nil)
		be.AssertThat(t, got, be.Eq("ON CONFLICT (model) DO NOTHING"))
	})
}
