package r3tag_test

import (
	"reflect"
	"testing"

	"github.com/expectto/be"

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
	be.RequireThat(t, ok, be.True(), "field %q not found", name)
	return f
}

func TestParseColumnTag_CapabilityFlags(t *testing.T) {
	slug := r3tag.ParseColumnTag(fieldByName(t, "Slug"))
	be.AssertThat(t, slug, be.HaveFields(map[string]any{
		"Column": "slug", "Immutable": true, "ReadOnly": false,
	}))

	pop := r3tag.ParseColumnTag(fieldByName(t, "Pop"))
	be.AssertThat(t, pop, be.HaveFields(map[string]any{"Column": "population", "ReadOnly": true}))

	secret := r3tag.ParseColumnTag(fieldByName(t, "Secret"))
	be.AssertThat(t, secret, be.HaveFields(map[string]any{
		"NoFilter": true, "NoSort": true, "NoOutput": true,
	}))

	status := r3tag.ParseColumnTag(fieldByName(t, "Status"))
	be.AssertThat(t, status.Enum, be.Eq([]string{"draft", "planned", "published"}))
	be.AssertThat(t, status.Column, be.Eq("status"))

	start := r3tag.ParseColumnTag(fieldByName(t, "Start"))
	be.AssertThat(t, start, be.HaveFields(map[string]any{"Column": "started_at", "Codec": "unixtime"}))

	// A codec composes with capability flags on the same field.
	combo := r3tag.ParseColumnTag(fieldByName(t, "Combo"))
	be.AssertThat(t, combo, be.HaveFields(map[string]any{
		"Column": "combo", "Codec": "unixmilli", "ReadOnly": true,
	}))
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
	be.RequireThat(t, ok, be.True(), "field %q not found", name)
	return f
}

func TestParseColumnTag_GormFallback(t *testing.T) {
	id := r3tag.ParseColumnTag(gormFieldByName(t, "ID"))
	be.AssertThat(t, id, be.HaveFields(map[string]any{"Column": "id", "IsPK": true}))

	loc := r3tag.ParseColumnTag(gormFieldByName(t, "LocationID"))
	be.AssertThat(t, loc.Column, be.Eq("venue_id"))

	name := r3tag.ParseColumnTag(gormFieldByName(t, "Name"))
	be.AssertThat(t, name, be.HaveFields(map[string]any{"Column": "full_name", "IsPK": true}))

	skipped := r3tag.ParseColumnTag(gormFieldByName(t, "Skipped"))
	be.AssertThat(t, skipped.Skip, be.True())

	dbWins := r3tag.ParseColumnTag(gormFieldByName(t, "DBWins"))
	be.AssertThat(t, dbWins.Column, be.Eq("db_name"))

	r3Wins := r3tag.ParseColumnTag(gormFieldByName(t, "R3Wins"))
	be.AssertThat(t, r3Wins, be.HaveFields(map[string]any{"Column": "r3_name", "Skip": false}))

	plain := r3tag.ParseColumnTag(gormFieldByName(t, "Plain"))
	be.AssertThat(t, plain.Column, be.Eq("plain"))
}

func TestParseColumnTag_PreservesExistingBehavior(t *testing.T) {
	id := r3tag.ParseColumnTag(fieldByName(t, "ID"))
	be.AssertThat(t, id, be.HaveFields(map[string]any{"Column": "id", "IsPK": true}))

	del := r3tag.ParseColumnTag(fieldByName(t, "Del"))
	be.AssertThat(t, del, be.HaveFields(map[string]any{"Column": "deleted_at", "SoftDelete": true}))
}
