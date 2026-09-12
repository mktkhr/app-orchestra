package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/infra/config"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PORT", "")
	t.Setenv("ORCHESTRA_STATIC_DIR", "")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, 8080, cfg.Port)
	assert.Empty(t, cfg.StaticDir)
}

func TestLoadContextTurnsDefaultsToEight(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_CONTEXT_TURNS", "")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, 8, cfg.ContextTurns)
}

func TestLoadReadsContextTurns(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_CONTEXT_TURNS", "3")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, 3, cfg.ContextTurns)
}

func TestLoadRejectsANonPositiveContextTurns(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_CONTEXT_TURNS", "0")

	_, err := config.Load()

	require.ErrorIs(t, err, config.ErrInvalidContextTurns)
}

func TestLoadRejectsAnUnparseableContextTurns(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_CONTEXT_TURNS", "not-a-number")

	_, err := config.Load()

	require.ErrorIs(t, err, config.ErrInvalidContextTurns)
}

func TestLoadReadsPortAndStaticDir(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PORT", "9090")
	t.Setenv("ORCHESTRA_STATIC_DIR", "/var/www")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, 9090, cfg.Port)
	assert.Equal(t, "/var/www", cfg.StaticDir)
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PORT", "not-a-number")

	_, err := config.Load()

	require.Error(t, err)
}

func TestLoadParsesServices(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
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
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_SERVICES", "")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Empty(t, cfg.Services)
}

func TestLoadRejectsMalformedServiceEntry(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_SERVICES", "inventory-without-equals-sign")

	_, err := config.Load()

	require.Error(t, err)
}

func TestLoadRejectsServiceEntryWithEmptyName(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_SERVICES", "=http://localhost:8081")

	_, err := config.Load()

	require.Error(t, err)
}

func TestLoadPlanFixturesDefaultsToEmpty(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PLAN_FIXTURES", "")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Empty(t, cfg.PlanFixtures)
}

func TestLoadParsesPlanFixtures(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv(
		"ORCHESTRA_PLAN_FIXTURES",
		`[{"query":"在庫の一覧を見せて","service":"inventory","operationId":"ListInventoryItems"},`+
			`{"query":"破損した在庫を見せて","answers":[{"param":"status","value":"quarantined"}],`+
			`"ask":true,"question":"どのステータスですか？","param":"status","service":"inventory",`+
			`"operationId":"ListInventoryItems","args":{"status":"quarantined"}}]`,
	)

	cfg, err := config.Load()

	require.NoError(t, err)
	require.Len(t, cfg.PlanFixtures, 2)
	assert.Equal(t, config.PlanFixture{
		Query:       "在庫の一覧を見せて",
		Service:     "inventory",
		OperationID: "ListInventoryItems",
	}, cfg.PlanFixtures[0])
	assert.Equal(t, config.PlanFixture{
		Query:       "破損した在庫を見せて",
		Answers:     []config.Answer{{Param: "status", Value: "quarantined"}},
		Ask:         true,
		Question:    "どのステータスですか？",
		Param:       "status",
		Service:     "inventory",
		OperationID: "ListInventoryItems",
		Args:        map[string]any{"status": "quarantined"},
	}, cfg.PlanFixtures[1])
}

func TestLoadRejectsMalformedPlanFixtures(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PLAN_FIXTURES", "not-json")

	_, err := config.Load()

	require.Error(t, err)
}

func TestLoadLLMSettingsDefaultToEmpty(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_LLM_BASE_URL", "")
	t.Setenv("ORCHESTRA_LLM_API_KEY", "")
	t.Setenv("ORCHESTRA_LLM_MODEL", "")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Empty(t, cfg.LLMBaseURL)
	assert.Empty(t, cfg.LLMAPIKey)
	assert.Empty(t, cfg.LLMModel)
}

func TestLoadReadsLLMSettings(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_LLM_BASE_URL", "http://localhost:11435/v1")
	t.Setenv("ORCHESTRA_LLM_API_KEY", "test-key")
	t.Setenv("ORCHESTRA_LLM_MODEL", "gemma4-26b-a4b-qat")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, "http://localhost:11435/v1", cfg.LLMBaseURL)
	assert.Equal(t, "test-key", cfg.LLMAPIKey)
	assert.Equal(t, "gemma4-26b-a4b-qat", cfg.LLMModel)
}

func TestLoadLLMModeDefaultsToToolCall(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_LLM_MODE", "")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, config.LLMModeToolCall, cfg.LLMMode)
}

func TestLoadReadsLLMModeJSON(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_LLM_MODE", "json")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, config.LLMModeJSON, cfg.LLMMode)
}

func TestLoadRejectsAnUnknownLLMMode(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_LLM_MODE", "not-a-real-mode")

	_, err := config.Load()

	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrInvalidLLMMode)
}

func TestLoadReadsDBPath(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/var/lib/orchestra/workspaces.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, "/var/lib/orchestra/workspaces.db", cfg.DBPath)
}

// TestLoadRejectsMissingDBPath is the point of ORCHESTRA_DB_PATH having no
// default (docs/plans/workspaces.md, Task 0, Step 3): a platform that
// silently forgets where its workspaces live is worse than one that
// refuses to start.
func TestLoadRejectsMissingDBPath(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")

	_, err := config.Load()

	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrMissingDBPath)
}

func TestLoadReadsAdminPassword(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/var/lib/orchestra/workspaces.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, "correct horse battery staple", cfg.AdminPassword)
}

// TestLoadRejectsMissingAdminPassword is the point of
// ORCHESTRA_ADMIN_PASSWORD having no default (docs/specs/auth.md,
// section 3): a default password is a way of having no password at all
// while appearing to.
func TestLoadRejectsMissingAdminPassword(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/var/lib/orchestra/workspaces.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "")

	_, err := config.Load()

	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrMissingAdminPassword)
}

func TestLoadSeedAccountsDefaultsToEmpty(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_SEED_ACCOUNTS", "")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Empty(t, cfg.SeedAccounts)
}

func TestLoadParsesSeedAccounts(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv(
		"ORCHESTRA_SEED_ACCOUNTS",
		`[{"name":"yamada","password":"correct horse battery staple 2","role":"user"}]`,
	)

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, []config.SeedAccount{
		{Name: "yamada", Password: "correct horse battery staple 2", Role: "user"},
	}, cfg.SeedAccounts)
}

func TestLoadRejectsMalformedSeedAccounts(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_SEED_ACCOUNTS", "not-json")

	_, err := config.Load()

	require.Error(t, err)
}
