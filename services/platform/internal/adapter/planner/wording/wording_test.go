package wording_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/wording"
)

// The four constants below are copied, byte for byte, from
// internal/adapter/planner/toolcall/planner.go's systemPrompt and
// internal/usecase/tools.go's askUserDescription, listCapabilitiesDescription
// and proposePanelDescription, as they stood at commit 5bf5cf8 - the last
// commit before this subproject moved them into this package. AC-Q-101:
// TestByNameV1IsByteIdenticalToTheLiteralsAt5bf5cf8 below asserts
// wording.ByName("v1") equals these, not v1.go's own constants, so a
// refactor that quietly edits v1.go cannot pass by comparing itself to
// itself - if this test ever fails, the baseline every wording is
// measured against, and that Default() (v2-commit) was chosen over, is
// void.

// systemPromptAsOf5bf5cf8 as of 5bf5cf8.
const systemPromptAsOf5bf5cf8 = "You are given a set of tools, one per operation of a catalogue of " +
	"internal services, plus ask_user, list_capabilities and propose_panel. Read the user's " +
	"question, in Japanese, and either call exactly one tool that answers it, call " +
	"list_capabilities when the question asks what can be done rather than asking to do " +
	"something, call ask_user when a parameter's value cannot be told from the question, call " +
	"propose_panel when the question asks to put something on the workspace's screen rather than " +
	"asking to look something up, or call no tool at all when nothing in the catalogue answers " +
	"the question."

// askUserDescriptionAsOf5bf5cf8 as of 5bf5cf8.
const askUserDescriptionAsOf5bf5cf8 = "Call this ONLY when the question does not tell you which value to use for " +
	"a parameter that declares a fixed set of allowed values (an enum), and you need the person to pick " +
	"one from that set. Do NOT call this for a free-text parameter (for example a name) that has no " +
	"declared set of values - there is nothing to pick from, so this tool cannot help; leave that " +
	"parameter out of your call instead."

// listCapabilitiesDescriptionAsOf5bf5cf8 as of 5bf5cf8.
const listCapabilitiesDescriptionAsOf5bf5cf8 = "Call this when the question asks what operations are " +
	"available - in general (\"何ができるの？\") or for one named service (\"在庫について、どういう" +
	"操作ができる？\") - rather than asking to actually look something up or change something. " +
	"Returns the catalogue's own list of operations, so it never risks naming a capability that " +
	"does not exist."

// proposePanelDescriptionAsOf5bf5cf8 as of 5bf5cf8.
const proposePanelDescriptionAsOf5bf5cf8 = "Call this when the question asks to put something on the workspace's " +
	"screen - a panel, a chart, a table - rather than asking a question you should just answer. Name the " +
	"operation and arguments the panel's data should come from, exactly as you would for that operation's " +
	"own tool. component, chart, transform and title are all optional: leave any of them out and the " +
	"platform fills it in from the same rule it would have drawn the answer with. Never call this for a " +
	"question that only asks to look something up - call that operation's own tool instead."

// TestByNameV1IsByteIdenticalToTheLiteralsAt5bf5cf8 is AC-Q-101: v1 is the
// text in the product today, byte for byte, and a catalogue tool's
// description under v1 is exactly its summary - examples given to
// CatalogueTool change nothing. Asserted against wording.ByName("v1"), not
// wording.Default() - the default is v2-commit as of docs/plans/wording.md
// Task 3 (DECISIONS.md, 2026-09-15), so the baseline byte-identity check
// has to name v1 explicitly to keep meaning what it always meant.
func TestByNameV1IsByteIdenticalToTheLiteralsAt5bf5cf8(t *testing.T) {
	d, ok := wording.ByName("v1")
	require.True(t, ok)

	assert.Equal(t, "v1", d.Name)
	assert.Equal(t, systemPromptAsOf5bf5cf8, d.SystemPrompt)
	assert.Equal(t, askUserDescriptionAsOf5bf5cf8, d.AskUser)
	assert.Equal(t, listCapabilitiesDescriptionAsOf5bf5cf8, d.ListCapabilities)
	assert.Equal(t, proposePanelDescriptionAsOf5bf5cf8, d.ProposePanel)

	require.NotNil(t, d.CatalogueTool)
	assert.Equal(t, "list inventory items", d.CatalogueTool("list inventory items", nil))
	assert.Equal(t, "list inventory items",
		d.CatalogueTool("list inventory items", []string{"在庫を見せて", "在庫の状況を教えて"}),
		"v1.CatalogueTool must ignore examples - AC-Q-101 requires the wire unchanged once "+
			"usecase.Tool.Examples exists")
}

// TestDefaultIsV2Commit is docs/plans/wording.md Task 3's switch: the
// default changed from v1 to v2-commit by recorded decision (DECISIONS.md,
// 2026-09-15, "wording: v2-commit becomes the default") - v2-commit clears
// v1 on correct@1/correct@shown, none and list_capabilities without
// costing axis A or E, while v3/v4's ask-on-collision sentence and v5's
// in-tool examples are both negative results (same entry).
func TestDefaultIsV2Commit(t *testing.T) {
	assert.Equal(t, "v2-commit", wording.Default().Name)
}

// declaredNames is Names' expected declared order - not sorted: v1 first,
// since it is Default and every candidate is written as a delta from it,
// followed by the four candidates in the order docs/specs/wording.md
// section 4 lists them.
var declaredNames = []string{
	"v1", "v2-commit", "v3-ask-on-collision", "v4-commit-and-ask", "v5-examples-in-tools",
}

// TestNamesIsDeclaredOrderV1First asserts Names' exact order, declared,
// not sorted (see declaredNames' own doc comment for why).
func TestNamesIsDeclaredOrderV1First(t *testing.T) {
	assert.Equal(t, declaredNames, wording.Names())
}

// TestNamesAreUnique guards against a copy-pasted Name colliding with an
// existing one.
func TestNamesAreUnique(t *testing.T) {
	seen := make(map[string]bool, len(declaredNames))

	for _, name := range wording.Names() {
		assert.False(t, seen[name], "duplicate name %q", name)
		seen[name] = true
	}
}

// TestEveryDeclaredWordingHasEveryFieldNonEmpty is the shape test
// docs/plans/wording.md Task 1 Step 4 asks for: every field non-empty for
// every set this package declares.
func TestEveryDeclaredWordingHasEveryFieldNonEmpty(t *testing.T) {
	for _, name := range wording.Names() {
		w, ok := wording.ByName(name)
		require.True(t, ok, "ByName must round-trip every declared name")

		assert.NotEmpty(t, w.Name, "%s: Name", name)
		assert.NotEmpty(t, w.SystemPrompt, "%s: SystemPrompt", name)
		assert.NotEmpty(t, w.AskUser, "%s: AskUser", name)
		assert.NotEmpty(t, w.ListCapabilities, "%s: ListCapabilities", name)
		assert.NotEmpty(t, w.ProposePanel, "%s: ProposePanel", name)
		require.NotNil(t, w.CatalogueTool, "%s: CatalogueTool", name)
		assert.NotEmpty(t, w.CatalogueTool("s", nil), "%s: CatalogueTool(\"s\", nil)", name)
	}
}

// TestByNameRoundTripsEveryDeclaredName asserts ByName finds every name
// Names reports, and that the Wording it returns names itself the same
// way.
func TestByNameRoundTripsEveryDeclaredName(t *testing.T) {
	for _, name := range wording.Names() {
		w, ok := wording.ByName(name)
		require.True(t, ok)
		assert.Equal(t, name, w.Name)
	}
}

// TestByNameOnUnknownNameReturnsFalse is the other half of ByName's
// contract: internal/infra/config's ORCHESTRA_PLANNER_WORDING validation
// depends on this returning false, not a zero Wording nobody checked.
func TestByNameOnUnknownNameReturnsFalse(t *testing.T) {
	_, ok := wording.ByName("v99-does-not-exist")
	assert.False(t, ok)
}

// TestV5CatalogueToolAppendsQuotedExamplesInOrder is the literal assertion
// docs/plans/wording.md Task 1 Step 4 asks for: v5's own delta, spelled
// out for a fake summary and two examples.
func TestV5CatalogueToolAppendsQuotedExamplesInOrder(t *testing.T) {
	w, ok := wording.ByName("v5-examples-in-tools")
	require.True(t, ok)

	got := w.CatalogueTool("在庫を一覧表示する", []string{"在庫を見せて", "在庫の状況を教えて"})

	assert.Equal(t, "在庫を一覧表示する\n例: 「在庫を見せて」「在庫の状況を教えて」", got)
}

// TestV5CatalogueToolWithNoExamplesReturnsTheSummaryUnchanged is v5's
// other declared case: an endpoint with no x-orchestra-examples reads
// exactly as it does under v1.
func TestV5CatalogueToolWithNoExamplesReturnsTheSummaryUnchanged(t *testing.T) {
	w, ok := wording.ByName("v5-examples-in-tools")
	require.True(t, ok)

	assert.Equal(t, "list inventory items", w.CatalogueTool("list inventory items", nil))
}

// TestV2ThroughV4AddSentencesRatherThanReplacingV1sWords asserts the
// "small delta" shape docs/specs/wording.md section 4 asks for: every
// candidate's SystemPrompt still contains v1's own text, and only
// v2-commit/v4-commit-and-ask change ListCapabilities while only
// v3-ask-on-collision/v4-commit-and-ask change AskUser.
func TestV2ThroughV4AddSentencesRatherThanReplacingV1sWords(t *testing.T) {
	v1, foundV1 := wording.ByName("v1")
	require.True(t, foundV1)

	for _, name := range []string{"v2-commit", "v3-ask-on-collision", "v4-commit-and-ask"} {
		w, ok := wording.ByName(name)
		require.True(t, ok)

		assert.Contains(t, w.SystemPrompt, v1.SystemPrompt, "%s: SystemPrompt must extend v1's, not replace it", name)
		assert.NotEqual(t, v1.SystemPrompt, w.SystemPrompt, "%s: SystemPrompt must add something", name)
	}

	v2, ok := wording.ByName("v2-commit")
	require.True(t, ok)
	assert.Equal(t, v1.AskUser, v2.AskUser, "v2-commit must not touch AskUser")
	assert.NotEqual(t, v1.ListCapabilities, v2.ListCapabilities)

	v3, ok := wording.ByName("v3-ask-on-collision")
	require.True(t, ok)
	assert.Equal(t, v1.ListCapabilities, v3.ListCapabilities, "v3-ask-on-collision must not touch ListCapabilities")
	assert.NotEqual(t, v1.AskUser, v3.AskUser)

	v4, ok := wording.ByName("v4-commit-and-ask")
	require.True(t, ok)
	assert.NotEqual(t, v1.AskUser, v4.AskUser)
	assert.NotEqual(t, v1.ListCapabilities, v4.ListCapabilities)
}
