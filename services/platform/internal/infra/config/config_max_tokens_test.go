// config_max_tokens_test.go: config_max_tokens.go's own tests, split out of
// config_test.go the same way config_fill_test.go and config_hybrid_test.go
// are split out - config_test.go was already at its own 1000-line cap
// (guard-filelen, harness/quality/filelen.sh) before
// ORCHESTRA_PLANNER_MAX_TOKENS/ORCHESTRA_PLANNER_PICK_MAX_TOKENS existed.

package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/infra/config"
)

func TestLoadPlannerMaxTokensDefaultsTo1024(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PLANNER_MAX_TOKENS", "")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, 1024, cfg.PlannerMaxTokens)
}

func TestLoadPlannerMaxTokensParsesAnInt(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PLANNER_MAX_TOKENS", "4000")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, 4000, cfg.PlannerMaxTokens)
}

func TestLoadRejectsANonPositivePlannerMaxTokens(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PLANNER_MAX_TOKENS", "0")

	_, err := config.Load()

	require.ErrorIs(t, err, config.ErrInvalidPlannerMaxTokens)
}

func TestLoadRejectsANegativePlannerMaxTokens(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PLANNER_MAX_TOKENS", "-1")

	_, err := config.Load()

	require.ErrorIs(t, err, config.ErrInvalidPlannerMaxTokens)
}

func TestLoadRejectsAnUnparseablePlannerMaxTokens(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PLANNER_MAX_TOKENS", "not-a-number")

	_, err := config.Load()

	require.ErrorIs(t, err, config.ErrInvalidPlannerMaxTokens)
}

func TestLoadPlannerPickMaxTokensDefaultsTo200(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PLANNER_PICK_MAX_TOKENS", "")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, 200, cfg.PlannerPickMaxTokens)
}

func TestLoadPlannerPickMaxTokensParsesAnInt(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PLANNER_PICK_MAX_TOKENS", "500")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, 500, cfg.PlannerPickMaxTokens)
}

func TestLoadRejectsANonPositivePlannerPickMaxTokens(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PLANNER_PICK_MAX_TOKENS", "0")

	_, err := config.Load()

	require.ErrorIs(t, err, config.ErrInvalidPlannerPickMaxTokens)
}

func TestLoadRejectsAnUnparseablePlannerPickMaxTokens(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PLANNER_PICK_MAX_TOKENS", "not-a-number")

	_, err := config.Load()

	require.ErrorIs(t, err, config.ErrInvalidPlannerPickMaxTokens)
}
