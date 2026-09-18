package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/infra/config"
)

// config_service_router_test.go: ORCHESTRA_SERVICE_ROUTER/
// ORCHESTRA_SERVICE_ROUTER_THRESHOLD's own tests, split out of
// config_test.go for guard-filelen (harness/quality/filelen.sh's
// 1000-line cap - config_test.go was already at its own cap before this
// subproject existed).

// TestLoadServiceRouterDefaultsToNone documents that an unset
// ORCHESTRA_SERVICE_ROUTER resolves to config.ServiceRouterNone, with no
// Jev key required on its own, and the threshold defaults to 0.5 - the
// same default internal/adapter/planner/jev's own defaultAmbiguityThreshold
// uses (config_service_router.go's own doc comment).
func TestLoadServiceRouterDefaultsToNone(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, config.ServiceRouterNone, cfg.ServiceRouter)
	assert.InDelta(t, 0.5, cfg.ServiceRouterThreshold, 0.0001)
}

func TestLoadServiceRouterJevWithoutAKeyIsAnError(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_SERVICE_ROUTER", "jev")

	_, err := config.Load()

	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrMissingJevAPIKey)
}

func TestLoadServiceRouterJevWithAKeyReadsIt(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_SERVICE_ROUTER", "jev")
	t.Setenv("ORCHESTRA_JEV_API_KEY", "test-key")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, config.ServiceRouterJev, cfg.ServiceRouter)
	assert.Equal(t, "test-key", cfg.JevAPIKey)
}

func TestLoadRejectsAnUnknownServiceRouter(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_SERVICE_ROUTER", "remote")

	_, err := config.Load()

	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrInvalidServiceRouter)
}

func TestLoadServiceRouterThresholdReadsACustomValue(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_SERVICE_ROUTER", "jev")
	t.Setenv("ORCHESTRA_JEV_API_KEY", "test-key")
	t.Setenv("ORCHESTRA_SERVICE_ROUTER_THRESHOLD", "0.6")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.InDelta(t, 0.6, cfg.ServiceRouterThreshold, 0.0001)
}

func TestLoadRejectsAnInvalidServiceRouterThreshold(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_SERVICE_ROUTER_THRESHOLD", "not-a-number")

	_, err := config.Load()

	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrInvalidServiceRouterThreshold)
}
