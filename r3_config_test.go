package r3_test

import (
	"testing"

	"github.com/amberpixels/r3"
	"github.com/expectto/be"
	"github.com/expectto/be/be_math"
	"github.com/expectto/be/be_struct"
	betestify "github.com/expectto/be/x/testify"
)

func TestDefaultConfig(t *testing.T) {
	cfg := r3.DefaultConfig()
	betestify.Assert(t, cfg.Naming, be_struct.HavingField[r3.NamingConfig]("CreatedAtField", "created_at"))
	betestify.Assert(t, cfg.Naming, be_struct.HavingField[r3.NamingConfig]("UpdatedAtField", "updated_at"))
	betestify.Assert(t, cfg.Naming, be_struct.HavingField[r3.NamingConfig]("DeletedAtField", "deleted_at"))
	betestify.Assert(t, cfg.Defaults, be_struct.HavingField[r3.DefaultsConfig]("PageSize", r3.PageSizeDefault))
	// PageSizeDefault must be a sane, positive page size.
	betestify.Assert(t, cfg.Defaults.PageSize, be_math.Positive())
}

func TestWithConfig(t *testing.T) {
	cfg := r3.DefaultConfig()
	cfg.Naming.CreatedAtField = "creation_date"
	cfg.Defaults.PageSize = 25

	opts := r3.ResolveOptions(r3.WithConfig(cfg))
	betestify.Assert(t, opts.Config.Naming, be_struct.HavingField[r3.NamingConfig]("CreatedAtField", "creation_date"))
	betestify.Assert(t, opts.Config.Defaults, be_struct.HavingField[r3.DefaultsConfig]("PageSize", 25))
	// Unchanged fields keep defaults
	betestify.Assert(t, opts.Config.Naming, be_struct.HavingField[r3.NamingConfig]("UpdatedAtField", "updated_at"))
	betestify.Assert(t, opts.Config.Naming, be_struct.HavingField[r3.NamingConfig]("DeletedAtField", "deleted_at"))
}

func TestResolveOptionsDefaults(t *testing.T) {
	opts := r3.ResolveOptions() // no options
	betestify.Assert(t, opts.Config, be.Eq(r3.DefaultConfig()))
}
