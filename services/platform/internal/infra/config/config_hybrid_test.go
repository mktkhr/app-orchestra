package config_test

// config_hybrid_test.go: internal/adapter/planner/hybrid's own
// ORCHESTRA_PICKER=hybrid tests, split out of config_test.go for
// guard-filelen (harness/quality/filelen.sh's 1000-line cap -
// config_test.go was already at its own cap once these were added), the
// same split config.go's own doc comment describes for config_hybrid.go.

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/infra/config"
)

// TestLoadPickerHybridWithoutAKeyIsAnError mirrors
// TestLoadPickerJevWithoutAKeyIsAnError: PickerHybrid needs a jev.Picker
// for its own Jev half exactly as PickerJev does.
func TestLoadPickerHybridWithoutAKeyIsAnError(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PICKER", "hybrid")

	_, err := config.Load()

	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrMissingJevAPIKey)
}

func TestLoadPickerHybridWithAKeyParses(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PICKER", "hybrid")
	t.Setenv("ORCHESTRA_JEV_API_KEY", "test-key")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, config.PickerHybrid, cfg.Picker)
	assert.Equal(t, "test-key", cfg.JevAPIKey)
}

// TestLoadHybridJevTimeoutAndThresholdDefault documents that unset
// ORCHESTRA_HYBRID_JEV_TIMEOUT/ORCHESTRA_HYBRID_THRESHOLD resolve to
// 800ms and 0.7, mirroring internal/adapter/planner/hybrid's own
// defaultJevTimeout/defaultThreshold - read regardless of ORCHESTRA_PICKER,
// the same "always parsed" shape JevCriteria's own default already has.
func TestLoadHybridJevTimeoutAndThresholdDefault(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, 800*time.Millisecond, cfg.HybridJevTimeout)
	assert.InDelta(t, 0.7, cfg.HybridThreshold, 0.0001)
}

func TestLoadHybridJevTimeoutReadsACustomDuration(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_HYBRID_JEV_TIMEOUT", "1500ms")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, 1500*time.Millisecond, cfg.HybridJevTimeout)
}

func TestLoadRejectsAnUnparseableHybridJevTimeout(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_HYBRID_JEV_TIMEOUT", "not-a-duration")

	_, err := config.Load()

	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrInvalidHybridJevTimeout)
}

func TestLoadHybridThresholdReadsACustomValue(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_HYBRID_THRESHOLD", "0.6")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.InDelta(t, 0.6, cfg.HybridThreshold, 0.0001)
}

func TestLoadRejectsAnUnparseableHybridThreshold(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_HYBRID_THRESHOLD", "not-a-number")

	_, err := config.Load()

	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrInvalidHybridThreshold)
}
