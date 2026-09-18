package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/infra/config"
)

func TestLoadFillDefaultsToBothArmsOff(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.False(t, cfg.FillSkipEmpty)
	assert.Equal(t, config.FillEnumNone, cfg.FillEnum)
	assert.InDelta(t, 0.5, cfg.FillEnumThreshold, 0.0001)
	assert.Empty(t, cfg.FillEnumRefusal)
	assert.Equal(t, config.FillEnumUnsetWordingNarrow, cfg.FillEnumUnsetWording)
}

func TestLoadFillEnumRefusalReadsExactly1(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_FILL_ENUM_REFUSAL", "1")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, config.FillEnumRefusalOn, cfg.FillEnumRefusal)
}

func TestLoadFillEnumRefusalReadsSeparate(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_FILL_ENUM_REFUSAL", "separate")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, config.FillEnumRefusalSeparate, cfg.FillEnumRefusal)
}

func TestLoadRejectsAnUnknownFillEnumRefusal(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_FILL_ENUM_REFUSAL", "true")

	_, err := config.Load()

	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrInvalidFillEnumRefusal)
}

func TestLoadFillEnumUnsetWordingReadsWide(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_FILL_ENUM_UNSET_WORDING", "wide")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, config.FillEnumUnsetWordingWide, cfg.FillEnumUnsetWording)
}

func TestLoadRejectsAnUnknownFillEnumUnsetWording(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_FILL_ENUM_UNSET_WORDING", "loose")

	_, err := config.Load()

	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrInvalidFillEnumUnsetWording)
}

func TestLoadFillSkipEmptyReadsExactly1(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_FILL_SKIP_EMPTY", "1")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.True(t, cfg.FillSkipEmpty)
}

func TestLoadFillSkipEmptyIgnoresAnythingOtherThan1(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_FILL_SKIP_EMPTY", "true")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.False(t, cfg.FillSkipEmpty)
}

func TestLoadFillEnumJevWithoutAKeyIsAnError(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_FILL_ENUM", "jev")

	_, err := config.Load()

	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrMissingJevAPIKey)
}

func TestLoadFillEnumJevWithAKeyReadsIt(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_FILL_ENUM", "jev")
	t.Setenv("ORCHESTRA_JEV_API_KEY", "test-key")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, config.FillEnumJev, cfg.FillEnum)
	assert.Equal(t, "test-key", cfg.JevAPIKey)
}

func TestLoadRejectsAnUnknownFillEnum(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_FILL_ENUM", "remote")

	_, err := config.Load()

	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrInvalidFillEnum)
}

func TestLoadFillEnumThresholdReadsACustomValue(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_FILL_ENUM", "jev")
	t.Setenv("ORCHESTRA_JEV_API_KEY", "test-key")
	t.Setenv("ORCHESTRA_FILL_ENUM_THRESHOLD", "0.8")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.InDelta(t, 0.8, cfg.FillEnumThreshold, 0.0001)
}

func TestLoadRejectsAnInvalidFillEnumThreshold(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_FILL_ENUM_THRESHOLD", "not-a-number")

	_, err := config.Load()

	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrInvalidFillEnumThreshold)
}

// TestLoadFillSkipEmptyAndFillEnumAreIndependent proves the two arms can
// be set together, or either one alone, without one disabling the other.
func TestLoadFillSkipEmptyAndFillEnumAreIndependent(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_FILL_SKIP_EMPTY", "1")
	t.Setenv("ORCHESTRA_FILL_ENUM", "jev")
	t.Setenv("ORCHESTRA_JEV_API_KEY", "test-key")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.True(t, cfg.FillSkipEmpty)
	assert.Equal(t, config.FillEnumJev, cfg.FillEnum)
}
