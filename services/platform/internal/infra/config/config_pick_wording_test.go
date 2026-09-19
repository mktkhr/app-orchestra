// config_pick_wording_test.go: config_pick_wording.go's own tests, split
// out of config_test.go the same way config_max_tokens_test.go is split
// out - config_test.go was already at its own 1000-line cap
// (guard-filelen, harness/quality/filelen.sh) before
// ORCHESTRA_PICK_WORDING existed.

package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/infra/config"
)

// TestLoadPickWordingDefaultsToV1 is the pick's own version of
// TestLoadPlannerWordingDefaultsToV6UnmatchedFilter: an unset
// ORCHESTRA_PICK_WORDING resolves to pick.DefaultWording().Name, "v1" -
// today's text, byte for byte.
func TestLoadPickWordingDefaultsToV1(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PICK_WORDING", "")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, "v1", cfg.PickWording)
}

func TestLoadReadsAKnownPickWording(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PICK_WORDING", "v2-strict-capabilities")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, "v2-strict-capabilities", cfg.PickWording)
}

// TestLoadRejectsAnUnknownPickWording mirrors
// TestLoadRejectsAnUnknownPlannerWording: an unrecognised
// ORCHESTRA_PICK_WORDING fails startup, naming every known set in its
// message rather than silently falling back to the default.
func TestLoadRejectsAnUnknownPickWording(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PICK_WORDING", "not-a-real-wording")

	_, err := config.Load()

	require.Error(t, err)
	require.ErrorIs(t, err, config.ErrInvalidPickWording)
	assert.Contains(t, err.Error(), "v1")
	assert.Contains(t, err.Error(), "v2-strict-capabilities")
}
