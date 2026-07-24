package r3_test

import (
	"testing"

	"github.com/expectto/be"
	"github.com/expectto/be/be_math"
	"github.com/expectto/be/be_struct"

	"github.com/amberpixels/r3"
)

func TestDefaultConfig(t *testing.T) {
	cfg := r3.DefaultConfig()
	be.AssertThat(t, cfg.Naming, be_struct.HavingField[r3.NamingConfig]("CreatedAtField", "created_at"))
	be.AssertThat(t, cfg.Naming, be_struct.HavingField[r3.NamingConfig]("UpdatedAtField", "updated_at"))
	be.AssertThat(t, cfg.Naming, be_struct.HavingField[r3.NamingConfig]("DeletedAtField", "deleted_at"))
	be.AssertThat(t, cfg.Defaults, be_struct.HavingField[r3.DefaultsConfig]("PageSize", r3.PageSizeDefault))
	// PageSizeDefault must be a sane, positive page size.
	be.AssertThat(t, cfg.Defaults.PageSize, be_math.Positive())
}

func TestWithConfig(t *testing.T) {
	cfg := r3.DefaultConfig()
	cfg.Naming.CreatedAtField = "creation_date"
	cfg.Defaults.PageSize = 25

	opts := r3.ResolveOptions(r3.WithConfig(cfg))
	be.AssertThat(t, opts.Config.Naming, be_struct.HavingField[r3.NamingConfig]("CreatedAtField", "creation_date"))
	be.AssertThat(t, opts.Config.Defaults, be_struct.HavingField[r3.DefaultsConfig]("PageSize", 25))
	// Unchanged fields keep defaults
	be.AssertThat(t, opts.Config.Naming, be_struct.HavingField[r3.NamingConfig]("UpdatedAtField", "updated_at"))
	be.AssertThat(t, opts.Config.Naming, be_struct.HavingField[r3.NamingConfig]("DeletedAtField", "deleted_at"))
}

func TestResolveOptionsDefaults(t *testing.T) {
	opts := r3.ResolveOptions() // no options
	be.AssertThat(t, opts.Config, be.Eq(r3.DefaultConfig()))
}
