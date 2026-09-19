package pick_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/pick"
)

// wordingV1ListCapabilities, wordingV1ProposePanel and wordingV1None are
// copied, byte for byte, from pick.PhraseListCapabilities,
// pick.PhraseProposePanel and pick.PhraseNone as they stand today - not
// referenced from those constants directly, so a stray edit to prompt.go's
// v1Wording (or to the constants it is built from) cannot pass by
// comparing itself to itself, the same reasoning
// wording_test.go's own systemPromptAsOf5bf5cf8 gives for the toolcall
// planner's own v1.
const (
	wordingV1ListCapabilities = "使える操作の一覧そのものを求めている"
	wordingV1ProposePanel     = "画面に出したい"
	wordingV1None             = "どの候補も質問に合わない（業務と無関係な質問、候補の操作では答えられない質問）"
)

// TestWordingByNameV1IsByteIdenticalToTodaysPhrases is the byte-identity
// guard the task asks for: pick.WordingByName("v1") must equal the three
// literals above, not prompt.go's own constants.
func TestWordingByNameV1IsByteIdenticalToTodaysPhrases(t *testing.T) {
	w, ok := pick.WordingByName("v1")
	require.True(t, ok)

	assert.Equal(t, "v1", w.Name)
	assert.Equal(t, wordingV1ListCapabilities, w.ListCapabilities)
	assert.Equal(t, wordingV1ProposePanel, w.ProposePanel)
	assert.Equal(t, wordingV1None, w.None)
	assert.Equal(t, pick.SystemPrompt, w.SystemPrompt,
		"v1's SystemPrompt must be pick.SystemPrompt (the package const), byte for byte")
}

// TestDefaultWordingIsV1 asserts ORCHESTRA_PICK_WORDING unset (internal/infra/config)
// resolves to today's text, byte for byte - AC-Q-101's own shape, for the
// pick's own wording.
func TestDefaultWordingIsV1(t *testing.T) {
	assert.Equal(t, "v1", pick.DefaultWording().Name)
	assert.Equal(t, pick.DefaultWording(), mustWording(t, "v1"))
}

// mustWording is pick.WordingByName, failing the test rather than
// returning ok=false - the shared lookup every test below that expects a
// name to exist uses.
func mustWording(t *testing.T, name string) pick.Wording {
	t.Helper()

	w, ok := pick.WordingByName(name)
	require.True(t, ok)

	return w
}

// TestWordingNamesIsDeclaredOrderV1First asserts WordingNames' exact
// order, declared, not sorted - v1 first, since it is DefaultWording.
func TestWordingNamesIsDeclaredOrderV1First(t *testing.T) {
	assert.Equal(t, []string{"v1", "v2-strict-capabilities", "v3-verb", "v4-specific", "v5-commit-to-a-candidate"}, pick.WordingNames())
}

// TestWordingNamesAreUnique guards against a copy-pasted Name colliding
// with an existing one.
func TestWordingNamesAreUnique(t *testing.T) {
	seen := make(map[string]bool)

	for _, name := range pick.WordingNames() {
		assert.False(t, seen[name], "duplicate name %q", name)
		seen[name] = true
	}
}

// TestWordingByNameRoundTripsEveryDeclaredName asserts WordingByName finds
// every name WordingNames reports, and that the Wording it returns names
// itself the same way, with every field non-empty.
func TestWordingByNameRoundTripsEveryDeclaredName(t *testing.T) {
	for _, name := range pick.WordingNames() {
		w := mustWording(t, name)

		assert.Equal(t, name, w.Name)
		assert.NotEmpty(t, w.ListCapabilities, "%s: ListCapabilities", name)
		assert.NotEmpty(t, w.ProposePanel, "%s: ProposePanel", name)
		assert.NotEmpty(t, w.None, "%s: None", name)
	}
}

// TestWordingByNameOnUnknownNameReturnsFalse is the other half of
// WordingByName's contract: internal/infra/config's
// ORCHESTRA_PICK_WORDING validation depends on this returning false, not a
// zero Wording nobody checked.
func TestWordingByNameOnUnknownNameReturnsFalse(t *testing.T) {
	_, ok := pick.WordingByName("v99-does-not-exist")
	assert.False(t, ok)
}

// TestV2StrictCapabilitiesDiffersOnlyInListCapabilities is the task's own
// shape requirement: v2-strict-capabilities touches ListCapabilities alone
// - ProposePanel and None stay exactly v1's, since bonsai2-27b's four
// losses never touched those two lines (DECISIONS.md, 2026-09-19).
func TestV2StrictCapabilitiesDiffersOnlyInListCapabilities(t *testing.T) {
	v1 := mustWording(t, "v1")
	v2 := mustWording(t, "v2-strict-capabilities")

	assert.NotEqual(t, v1.ListCapabilities, v2.ListCapabilities, "v2-strict-capabilities must change ListCapabilities")
	assert.Contains(t, v2.ListCapabilities, v1.ListCapabilities,
		"v2-strict-capabilities must extend v1's ListCapabilities, not replace it")
	assert.Equal(t, v1.ProposePanel, v2.ProposePanel, "v2-strict-capabilities must not touch ProposePanel")
	assert.Equal(t, v1.None, v2.None, "v2-strict-capabilities must not touch None")
	assert.Equal(t, v1.SystemPrompt, v2.SystemPrompt,
		"v2-strict-capabilities must not touch SystemPrompt - it only changed a built-in line")
}

// TestVerbAndSpecificDifferFromV1OnlyInSystemPrompt asserts v3-verb's,
// v4-specific's and v5-commit-to-a-candidate's shared shape: each extends
// v1's SystemPrompt with exactly one added sentence and touches nothing
// else - ListCapabilities, ProposePanel and None stay v1's. One
// table-driven test, not three near copies, so golangci's dupl linter
// (harness/quality/go/golangci.yml) sees one body.
func TestVerbAndSpecificDifferFromV1OnlyInSystemPrompt(t *testing.T) {
	v1 := mustWording(t, "v1")

	for _, name := range []string{"v3-verb", "v4-specific", "v5-commit-to-a-candidate"} {
		t.Run(name, func(t *testing.T) {
			w := mustWording(t, name)

			assert.NotEqual(t, v1.SystemPrompt, w.SystemPrompt, "%s must change SystemPrompt", name)
			require.True(t, strings.HasPrefix(w.SystemPrompt, v1.SystemPrompt),
				"%s must extend v1's SystemPrompt, not replace it", name)

			addition := strings.TrimPrefix(w.SystemPrompt, v1.SystemPrompt)
			assert.Equal(t, 1, strings.Count(addition, "。"), "%s's addition must be exactly one sentence", name)

			assert.Equal(t, v1.ListCapabilities, w.ListCapabilities, "%s must not touch ListCapabilities", name)
			assert.Equal(t, v1.ProposePanel, w.ProposePanel, "%s must not touch ProposePanel", name)
			assert.Equal(t, v1.None, w.None, "%s must not touch None", name)
		})
	}
}

// TestV3VerbAndV4SpecificNameNoResourceOrExample guards the task's own
// constraint: the added sentence states the rule only, never a resource,
// service or example question.
func TestV3VerbAndV4SpecificNameNoResourceOrExample(t *testing.T) {
	v1 := mustWording(t, "v1")

	for _, name := range []string{"v3-verb", "v4-specific", "v5-commit-to-a-candidate"} {
		w := mustWording(t, name)
		addition := strings.TrimPrefix(w.SystemPrompt, v1.SystemPrompt)

		assert.NotEmpty(t, strings.TrimSpace(addition), "%s: addition must not be empty", name)

		for _, forbidden := range []string{
			"在庫", "勤怠", "listInventoryItems", "遅刻", "有給",
			"見本", "壊れた", "常連", "特典", "定期", "サブスク", "契約",
		} {
			assert.NotContains(t, addition, forbidden, "%s: addition must name no resource or example", name)
		}
	}
}

// TestV5CommitToACandidateIsCompatibleWithTheBuiltInLines guards the task's
// own constraint that the new sentence must not contradict the three
// built-in candidate lines' own phrasing (PhraseListCapabilities,
// PhraseNone): it neither retires list_capabilities' own wording nor
// duplicates it, and it does not touch any of the three lines.
func TestV5CommitToACandidateIsCompatibleWithTheBuiltInLines(t *testing.T) {
	v1 := mustWording(t, "v1")
	v5 := mustWording(t, "v5-commit-to-a-candidate")

	addition := strings.TrimPrefix(v5.SystemPrompt, v1.SystemPrompt)

	assert.Contains(t, addition, "使える操作の一覧",
		"v5-commit-to-a-candidate must stay compatible with PhraseListCapabilities' own wording")
	assert.Equal(t, v1.ListCapabilities, v5.ListCapabilities, "v5-commit-to-a-candidate must not touch ListCapabilities")
	assert.Equal(t, v1.ProposePanel, v5.ProposePanel, "v5-commit-to-a-candidate must not touch ProposePanel")
	assert.Equal(t, v1.None, v5.None, "v5-commit-to-a-candidate must not touch None")
}

// TestV2StrictCapabilitiesNamesAnExplicitActionExclusion asserts the
// concrete fix the task asks for: v2's own ListCapabilities line must name
// the action-oriented exclusion (creating, recording, applying) that made
// bonsai2-27b answer list_capabilities for real-report-tardiness,
// real-apply-paid-leave and real-decrease-inventory.
func TestV2StrictCapabilitiesNamesAnExplicitActionExclusion(t *testing.T) {
	v2 := mustWording(t, "v2-strict-capabilities")

	assert.Contains(t, v2.ListCapabilities, "作成")
	assert.Contains(t, v2.ListCapabilities, "申請")
}
