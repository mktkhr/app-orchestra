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

// TestLoadNarrowingDefaultsToOff is docs/specs/shortlisting.md H7: with
// none of the three ORCHESTRA_NARROWING_* variables set, narrowing is off
// and Load does not fail startup over it.
func TestLoadNarrowingDefaultsToOff(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_NARROWING_EMBED_MODEL", "")
	t.Setenv("ORCHESTRA_NARROWING_RERANK_MODEL", "")
	t.Setenv("ORCHESTRA_NARROWING_K", "")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Empty(t, cfg.NarrowingEmbedModel)
	assert.Empty(t, cfg.NarrowingRerankModel)
	assert.Zero(t, cfg.NarrowingK)
}

// TestLoadReadsNarrowingWhenAllThreeAreSet is the "all" half of the
// section 5 rule: with every variable set, Load parses each of them.
func TestLoadReadsNarrowingWhenAllThreeAreSet(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_NARROWING_EMBED_MODEL", "e5-large-q8")
	t.Setenv("ORCHESTRA_NARROWING_RERANK_MODEL", "bge-reranker-v2-m3-q8")
	t.Setenv("ORCHESTRA_NARROWING_K", "20")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, "e5-large-q8", cfg.NarrowingEmbedModel)
	assert.Equal(t, "bge-reranker-v2-m3-q8", cfg.NarrowingRerankModel)
	assert.Equal(t, 20, cfg.NarrowingK)
}

// TestLoadRejectsNarrowingWithOnlyEmbedModelSet is the "one of three" half
// of the "all or none" rule (ErrNarrowingIncomplete).
func TestLoadRejectsNarrowingWithOnlyEmbedModelSet(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_NARROWING_EMBED_MODEL", "e5-large-q8")
	t.Setenv("ORCHESTRA_NARROWING_RERANK_MODEL", "")
	t.Setenv("ORCHESTRA_NARROWING_K", "")

	_, err := config.Load()

	require.ErrorIs(t, err, config.ErrNarrowingIncomplete)
}

// TestLoadRejectsNarrowingWithOnlyTwoOfThreeSet is the "two of three" half
// of the same rule - a startup error the same shape as one of three, not a
// special case of its own.
func TestLoadRejectsNarrowingWithOnlyTwoOfThreeSet(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_NARROWING_EMBED_MODEL", "e5-large-q8")
	t.Setenv("ORCHESTRA_NARROWING_RERANK_MODEL", "bge-reranker-v2-m3-q8")
	t.Setenv("ORCHESTRA_NARROWING_K", "")

	_, err := config.Load()

	require.ErrorIs(t, err, config.ErrNarrowingIncomplete)
}

// TestLoadRejectsANonPositiveNarrowingK mirrors
// TestLoadRejectsANonPositiveContextTurns for ORCHESTRA_NARROWING_K.
func TestLoadRejectsANonPositiveNarrowingK(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_NARROWING_EMBED_MODEL", "e5-large-q8")
	t.Setenv("ORCHESTRA_NARROWING_RERANK_MODEL", "bge-reranker-v2-m3-q8")
	t.Setenv("ORCHESTRA_NARROWING_K", "0")

	_, err := config.Load()

	require.ErrorIs(t, err, config.ErrInvalidNarrowingK)
}

// TestLoadRejectsAnUnparseableNarrowingK mirrors
// TestLoadRejectsAnUnparseableContextTurns for ORCHESTRA_NARROWING_K.
func TestLoadRejectsAnUnparseableNarrowingK(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_NARROWING_EMBED_MODEL", "e5-large-q8")
	t.Setenv("ORCHESTRA_NARROWING_RERANK_MODEL", "bge-reranker-v2-m3-q8")
	t.Setenv("ORCHESTRA_NARROWING_K", "not-a-number")

	_, err := config.Load()

	require.ErrorIs(t, err, config.ErrInvalidNarrowingK)
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

// TestLoadParsesAProposePlanFixture is part of the Gap this subproject's
// plan (docs/plans/proposing.md, Task 2) closes: ORCHESTRA_PLAN_FIXTURES
// can name a propose fixture, carrying its own component/chart/title, the
// same way it already carries an ask fixture's question/param above.
func TestLoadParsesAProposePlanFixture(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv(
		"ORCHESTRA_PLAN_FIXTURES",
		`[{"query":"在庫をステータス別に棒グラフで置いて","propose":true,`+
			`"service":"inventory","operationId":"SummarizeInventory","args":{},`+
			`"component":"chart","title":"ステータス別の在庫",`+
			`"chart":{"category":"status","value":"count","kind":"bar"}}]`,
	)

	cfg, err := config.Load()

	require.NoError(t, err)
	require.Len(t, cfg.PlanFixtures, 1)
	assert.Equal(t, config.PlanFixture{
		Query:       "在庫をステータス別に棒グラフで置いて",
		Propose:     true,
		Service:     "inventory",
		OperationID: "SummarizeInventory",
		Args:        map[string]any{},
		Component:   "chart",
		Title:       "ステータス別の在庫",
		Chart:       &config.Chart{Category: "status", Value: "count", Kind: "bar"},
	}, cfg.PlanFixtures[0])
}

// TestLoadParsesAnAskPlanFixtureWithOptions is the ORCHESTRA_PLAN_FIXTURES
// half of the ask_user degradation fix (2026-09-16, TODO.md item 3):
// "options" lets a fixture stand in for a model's own ask_user "options"
// argument, exercising askDegrade's rule 2
// (internal/usecase/orchestrator_ask.go) from a process started off the
// built binary (e2e/src/orchestration.test.ts), the same way
// TestLoadParsesAProposePlanFixture already does for a propose fixture.
func TestLoadParsesAnAskPlanFixtureWithOptions(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv(
		"ORCHESTRA_PLAN_FIXTURES",
		`[{"query":"注文を見たい","ask":true,"question":"受注ですか、発注ですか？","param":"kind",`+
			`"options":[{"value":"sales","label":"受注"},{"value":"purchase","label":"発注"}],`+
			`"service":"inventory","operationId":"ListInventoryItems"}]`,
	)

	cfg, err := config.Load()

	require.NoError(t, err)
	require.Len(t, cfg.PlanFixtures, 1)
	assert.Equal(t, config.PlanFixture{
		Query:       "注文を見たい",
		Ask:         true,
		Question:    "受注ですか、発注ですか？",
		Param:       "kind",
		Options:     []config.Option{{Value: "sales", Label: "受注"}, {Value: "purchase", Label: "発注"}},
		Service:     "inventory",
		OperationID: "ListInventoryItems",
	}, cfg.PlanFixtures[0])
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

// TestLoadPlannerWordingDefaultsToV6UnmatchedFilter is the second default
// switch: an unset ORCHESTRA_PLANNER_WORDING now resolves to
// wording.Default().Name, "v6-unmatched-filter" as of DECISIONS.md,
// 2026-09-16 ("wording: v6-unmatched-filter becomes the default") - v1 and
// v2-commit both stay reachable by name (see TestLoadReadsAKnownPlannerWording
// below).
func TestLoadPlannerWordingDefaultsToV6UnmatchedFilter(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PLANNER_WORDING", "")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, "v6-unmatched-filter", cfg.PlannerWording)
}

func TestLoadReadsAKnownPlannerWording(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PLANNER_WORDING", "v3-ask-on-collision")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, "v3-ask-on-collision", cfg.PlannerWording)
}

// TestLoadRejectsAnUnknownPlannerWording is AC-Q-102: an unrecognised
// ORCHESTRA_PLANNER_WORDING fails startup, naming every known wording in
// its message rather than silently falling back to the default.
func TestLoadRejectsAnUnknownPlannerWording(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PLANNER_WORDING", "not-a-real-wording")

	_, err := config.Load()

	require.Error(t, err)
	require.ErrorIs(t, err, config.ErrInvalidPlannerWording)
	assert.Contains(t, err.Error(), "v1")
	assert.Contains(t, err.Error(), "v2-commit")
	assert.Contains(t, err.Error(), "v3-ask-on-collision")
	assert.Contains(t, err.Error(), "v4-commit-and-ask")
	assert.Contains(t, err.Error(), "v5-examples-in-tools")
	assert.Contains(t, err.Error(), "v6-unmatched-filter")
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

// TestLoadPlannerThinkingDefaultsToFalse is the toolcall planner-knob half
// of the platform-knobs subproject: an unset ORCHESTRA_PLANNER_THINKING
// resolves to false - thinking off, the default decided 2026-09-16
// (measured: correct@1 67 / correct@shown 71 at a mean 1377ms with
// thinking off, never hitting max_tokens, against 67/70 at a mean 6223ms
// with thinking on; docs/specs/shortlisting.md).
func TestLoadPlannerThinkingDefaultsToFalse(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PLANNER_THINKING", "")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.False(t, cfg.PlannerThinking)
}

func TestLoadPlannerThinkingOn(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PLANNER_THINKING", "on")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.True(t, cfg.PlannerThinking)
}

func TestLoadPlannerThinkingOff(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PLANNER_THINKING", "off")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.False(t, cfg.PlannerThinking)
}

func TestLoadRejectsAnUnknownPlannerThinking(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PLANNER_THINKING", "maybe")

	_, err := config.Load()

	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrInvalidPlannerThinking)
}

// TestLoadPlannerStagesDefaultsToTwo documents the 2026-09-16 default: an
// unset ORCHESTRA_PLANNER_STAGES resolves to 2, the pick-then-fill path.
func TestLoadPlannerStagesDefaultsToTwo(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PLANNER_STAGES", "")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, 2, cfg.PlannerStages)
}

func TestLoadPlannerStagesReadsOneExplicitly(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PLANNER_STAGES", "1")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, 1, cfg.PlannerStages)
}

func TestLoadPlannerStagesReadsTwo(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PLANNER_STAGES", "2")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, 2, cfg.PlannerStages)
}

func TestLoadRejectsAnUnknownPlannerStages(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PLANNER_STAGES", "3")

	_, err := config.Load()

	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrInvalidPlannerStages)
}

// TestLoadPlannerRepeatPenaltyDefaultsToUnset documents that an unset
// ORCHESTRA_PLANNER_REPEAT_PENALTY sends nothing - today's behaviour.
func TestLoadPlannerRepeatPenaltyDefaultsToUnset(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PLANNER_REPEAT_PENALTY", "")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Nil(t, cfg.PlannerRepeatPenalty)
}

func TestLoadPlannerRepeatPenaltyParsesAFloat(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PLANNER_REPEAT_PENALTY", "1.1")

	cfg, err := config.Load()

	require.NoError(t, err)
	require.NotNil(t, cfg.PlannerRepeatPenalty)
	assert.InDelta(t, 1.1, *cfg.PlannerRepeatPenalty, 0)
}

func TestLoadRejectsAnInvalidPlannerRepeatPenalty(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PLANNER_REPEAT_PENALTY", "not-a-number")

	_, err := config.Load()

	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrInvalidPlannerRepeatPenalty)
}

// TestLoadPlannerRepeatLastNDefaultsTo64 documents
// Config.PlannerRepeatLastN's default (Config.PlannerRepeatLastN's own doc
// comment) - it is set even when ORCHESTRA_PLANNER_REPEAT_PENALTY is
// unset, since it has no effect on its own.
func TestLoadPlannerRepeatLastNDefaultsTo64(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PLANNER_REPEAT_LAST_N", "")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, 64, cfg.PlannerRepeatLastN)
}

func TestLoadPlannerRepeatLastNParsesAnInt(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PLANNER_REPEAT_LAST_N", "128")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, 128, cfg.PlannerRepeatLastN)
}

func TestLoadRejectsAnInvalidPlannerRepeatLastN(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_PLANNER_REPEAT_LAST_N", "0")

	_, err := config.Load()

	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrInvalidPlannerRepeatLastN)
}
