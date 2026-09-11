package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/infra/config"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("ORCHESTRA_PORT", "")
	t.Setenv("ORCHESTRA_STATIC_DIR", "")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, 8080, cfg.Port)
	assert.Empty(t, cfg.StaticDir)
}

func TestLoadReadsPortAndStaticDir(t *testing.T) {
	t.Setenv("ORCHESTRA_PORT", "9090")
	t.Setenv("ORCHESTRA_STATIC_DIR", "/var/www")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, 9090, cfg.Port)
	assert.Equal(t, "/var/www", cfg.StaticDir)
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	t.Setenv("ORCHESTRA_PORT", "not-a-number")

	_, err := config.Load()

	require.Error(t, err)
}

func TestLoadParsesServices(t *testing.T) {
	t.Setenv(
		"ORCHESTRA_SERVICES",
		"inventory=http://localhost:8081,attendance=http://localhost:8082",
	)

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, []config.Service{
		{Name: "inventory", URL: "http://localhost:8081"},
		{Name: "attendance", URL: "http://localhost:8082"},
	}, cfg.Services)
}

func TestLoadServicesDefaultsToEmpty(t *testing.T) {
	t.Setenv("ORCHESTRA_SERVICES", "")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Empty(t, cfg.Services)
}

func TestLoadRejectsMalformedServiceEntry(t *testing.T) {
	t.Setenv("ORCHESTRA_SERVICES", "inventory-without-equals-sign")

	_, err := config.Load()

	require.Error(t, err)
}

func TestLoadRejectsServiceEntryWithEmptyName(t *testing.T) {
	t.Setenv("ORCHESTRA_SERVICES", "=http://localhost:8081")

	_, err := config.Load()

	require.Error(t, err)
}
