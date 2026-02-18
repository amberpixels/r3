package r3_test

import (
	"testing"

	"github.com/amberpixels/r3"
	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	cfg := r3.DefaultConfig()
	assert.Equal(t, "created_at", cfg.Naming.CreatedAtField)
	assert.Equal(t, "updated_at", cfg.Naming.UpdatedAtField)
	assert.Equal(t, "deleted_at", cfg.Naming.DeletedAtField)
	assert.Equal(t, r3.PageSizeDefault, cfg.Defaults.PageSize)
}

func TestWithConfig(t *testing.T) {
	cfg := r3.DefaultConfig()
	cfg.Naming.CreatedAtField = "creation_date"
	cfg.Defaults.PageSize = 25

	opts := r3.ResolveOptions(r3.WithConfig(cfg))
	assert.Equal(t, "creation_date", opts.Config.Naming.CreatedAtField)
	assert.Equal(t, 25, opts.Config.Defaults.PageSize)
	// Unchanged fields keep defaults
	assert.Equal(t, "updated_at", opts.Config.Naming.UpdatedAtField)
	assert.Equal(t, "deleted_at", opts.Config.Naming.DeletedAtField)
}

func TestResolveOptionsDefaults(t *testing.T) {
	opts := r3.ResolveOptions() // no options
	assert.Equal(t, r3.DefaultConfig(), opts.Config)
}
