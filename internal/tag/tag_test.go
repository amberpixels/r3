package r3tag_test

import (
	"reflect"
	"slices"
	"testing"

	r3tag "github.com/amberpixels/r3/internal/tag"
)

type capModel struct {
	ID     int64  `r3:"id,pk"`
	Title  string `r3:"title"`
	Slug   string `r3:"slug,immutable"`
	Pop    int    `r3:"population,readonly"`
	Secret string `r3:"secret_token,no-filter,no-sort,no-output"`
	Status string `r3:"status,enum:draft|planned|published"`
	Del    string `r3:"deleted_at,soft_delete"`
	Start  int64  `r3:"started_at,codec:unixtime"`
	Combo  int64  `r3:"combo,readonly,codec:unixmilli"`
}

func fieldByName(t *testing.T, name string) reflect.StructField {
	t.Helper()
	f, ok := reflect.TypeFor[capModel]().FieldByName(name)
	if !ok {
		t.Fatalf("field %q not found", name)
	}
	return f
}

func TestParseColumnTag_CapabilityFlags(t *testing.T) {
	slug := r3tag.ParseColumnTag(fieldByName(t, "Slug"))
	if slug.Column != "slug" || !slug.Immutable || slug.ReadOnly {
		t.Errorf("slug = %+v, want column=slug immutable=true readonly=false", slug)
	}

	pop := r3tag.ParseColumnTag(fieldByName(t, "Pop"))
	if pop.Column != "population" || !pop.ReadOnly {
		t.Errorf("population = %+v, want readonly=true", pop)
	}

	secret := r3tag.ParseColumnTag(fieldByName(t, "Secret"))
	if !secret.NoFilter || !secret.NoSort || !secret.NoOutput {
		t.Errorf("secret = %+v, want all hide flags set", secret)
	}

	status := r3tag.ParseColumnTag(fieldByName(t, "Status"))
	if want := []string{"draft", "planned", "published"}; !slices.Equal(status.Enum, want) {
		t.Errorf("status enum = %v, want %v", status.Enum, want)
	}
	if status.Column != "status" {
		t.Errorf("status column = %q, want status", status.Column)
	}

	start := r3tag.ParseColumnTag(fieldByName(t, "Start"))
	if start.Column != "started_at" || start.Codec != "unixtime" {
		t.Errorf("start = %+v, want column=started_at codec=unixtime", start)
	}

	// A codec composes with capability flags on the same field.
	combo := r3tag.ParseColumnTag(fieldByName(t, "Combo"))
	if combo.Column != "combo" || combo.Codec != "unixmilli" || !combo.ReadOnly {
		t.Errorf("combo = %+v, want column=combo codec=unixmilli readonly=true", combo)
	}
}

type gormModel struct {
	ID         int64  `gorm:"primaryKey"`
	LocationID int64  `gorm:"column:venue_id"`
	Name       string `gorm:"column:full_name;primary_key"`
	Skipped    string `gorm:"-"`
	DBWins     string `gorm:"column:gorm_name"             db:"db_name"`
	R3Wins     string `gorm:"column:gorm_name;-"                        r3:"r3_name"`
	Plain      string
}

func gormFieldByName(t *testing.T, name string) reflect.StructField {
	t.Helper()
	f, ok := reflect.TypeFor[gormModel]().FieldByName(name)
	if !ok {
		t.Fatalf("field %q not found", name)
	}
	return f
}

func TestParseColumnTag_GormFallback(t *testing.T) {
	id := r3tag.ParseColumnTag(gormFieldByName(t, "ID"))
	if id.Column != "id" || !id.IsPK {
		t.Errorf("id = %+v, want column=id pk=true", id)
	}

	loc := r3tag.ParseColumnTag(gormFieldByName(t, "LocationID"))
	if loc.Column != "venue_id" {
		t.Errorf("location = %+v, want column=venue_id", loc)
	}

	name := r3tag.ParseColumnTag(gormFieldByName(t, "Name"))
	if name.Column != "full_name" || !name.IsPK {
		t.Errorf("name = %+v, want column=full_name pk=true", name)
	}

	skipped := r3tag.ParseColumnTag(gormFieldByName(t, "Skipped"))
	if !skipped.Skip {
		t.Errorf("skipped = %+v, want skip=true", skipped)
	}

	dbWins := r3tag.ParseColumnTag(gormFieldByName(t, "DBWins"))
	if dbWins.Column != "db_name" {
		t.Errorf("dbWins = %+v, want column=db_name (db tag beats gorm)", dbWins)
	}

	r3Wins := r3tag.ParseColumnTag(gormFieldByName(t, "R3Wins"))
	if r3Wins.Column != "r3_name" || r3Wins.Skip {
		t.Errorf("r3Wins = %+v, want column=r3_name skip=false (r3 tag beats gorm)", r3Wins)
	}

	plain := r3tag.ParseColumnTag(gormFieldByName(t, "Plain"))
	if plain.Column != "plain" {
		t.Errorf("plain = %+v, want column=plain (snake_case fallback)", plain)
	}
}

func TestParseColumnTag_PreservesExistingBehavior(t *testing.T) {
	id := r3tag.ParseColumnTag(fieldByName(t, "ID"))
	if id.Column != "id" || !id.IsPK {
		t.Errorf("id = %+v, want column=id pk=true", id)
	}
	del := r3tag.ParseColumnTag(fieldByName(t, "Del"))
	if del.Column != "deleted_at" || !del.SoftDelete {
		t.Errorf("del = %+v, want column=deleted_at soft_delete=true", del)
	}
}
