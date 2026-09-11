package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/attendance/internal/infra/config"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("ORCHESTRA_PORT", "")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, 8082, cfg.Port)
}

func TestLoadReadsPort(t *testing.T) {
	t.Setenv("ORCHESTRA_PORT", "9092")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, 9092, cfg.Port)
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	t.Setenv("ORCHESTRA_PORT", "not-a-number")

	_, err := config.Load()

	require.Error(t, err)
}
