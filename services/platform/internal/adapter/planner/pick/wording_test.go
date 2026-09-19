package pick_test

import (
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
	assert.Equal(t, []string{"v1", "v2-strict-capabilities"}, pick.WordingNames())
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
