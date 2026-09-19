// Package pick implements usecase.Picker: it sends the shortlist in the
// measured stand-in picker's own format - the one e2e/narrowing/pick/client.ts
// used to produce the 83 in docs/specs/staging.md section 1 - over
// internal/adapter/planner/chat, and parses the one line it answers with
// back into a usecase.Pick.
package pick

import (
	"strings"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// SystemPrompt is byte-identical to e2e/narrowing/pick/client.ts's
// PICK_SYSTEM_PROMPT (S2, AC-S-103, docs/specs/staging.md): the pick's own
// number is only comparable to the 83 if the model is asked the same way.
// prompt_test.go asserts this against that file's own text; that file's
// own client-prompt.test.ts gains the mirror assertion the other way, so
// neither side is ever a hand-copied transcription of the other.
const SystemPrompt = `あなたは社内APIの振り分け役。質問に対して、候補一覧の中から呼ぶべきAPIを1つ選ぶ。

出力は次の形式の1行だけ。説明もタグも書かない。
listInventoryItems certain

1語目は候補一覧にある operationId をそのまま。2語目は certain か ambiguous。
ambiguous は「質問文だけでは候補を1つに決められない」場合。自信の有無ではなく、質問が足りていない場合。
ambiguous のときも、最も可能性の高い operationId を必ず1つ挙げること。`

// The three built-in ids the pick offers beside the shortlist (S3): the
// only text in this subproject that is new prose to the model, and
// exactly what section 7's measurement judges.
//
// The wording below is the first of at most three attempts the
// unanswerable regression allowed (docs/specs/staging.md section 7): the
// original list_capabilities line, "何ができるか知りたい" ("want to know
// what can be done"), reads as a paraphrase of any question the model
// cannot otherwise place, including 「今日の天気は？」 - so an
// off-topic question was picked as a request for the capabilities table
// instead of none. Renaming it to "使える操作の一覧を知りたい" ("want the
// list of operations available") ties the line to naming the built-in
// itself rather than to not knowing an answer, and giving none its own
// explicit exclusion, "どの候補も質問に合わない（業務と無関係な質問）"
// ("no candidate fits the question (a question unrelated to the
// business)"), gives the model a line to prefer for exactly that
// question instead of leaving it to fall through. This one wording fixed
// both `unanswerable` and kept `capability` at 10/10
// (ORCHESTRA_PLANNER_STAGES=2 make eval), so the other two attempts
// section 7 allowed were never needed.
//
// The second of those attempts was spent on 2026-09-18, when dropping
// `propose_panel` from a workspace-less pick (`eeb23fa`) reopened the
// same regression with one fewer built-in in the list: `unanswerable`
// (今日の天気は？) and `real-what-day` (今日は何曜日？) both moved from
// `none` to `list_capabilities` again (`DECISIONS.md`, 2026-09-18, the
// correction entry). The change is to this line alone - "使える操作の
// 一覧を知りたい" becomes "使える操作の一覧そのものを求めている", which
// asks for the list itself rather than merely wanting to know something -
// so the pull toward it from a question that names no operation is
// weaker, while `capability` (在庫で何ができる？), which does ask for the
// list, still reads it. `none`'s own line is deliberately left alone: the
// two rows move because list_capabilities attracts them, not because
// none is too weak to hold them.
//
// That attempt moved `real-what-day` back to `none` and held `capability`
// and `real-capability-inventory` at 10/10, but left `unanswerable`
// (今日の天気は？) still reading `list_capabilities` - so the third and
// last attempt section 7 allows widens none's own line instead:
// "（業務と無関係な質問）" becomes "（業務と無関係な質問、候補の操作では
// 答えられない質問）", naming the second case a question can fail in
// without naming any question. No word from an eval case appears here on
// purpose: a line listing 天気 or 日付 would fix the suite and nothing
// else.
//
// Exported (IDListCapabilities etc.) so internal/adapter/planner/jev's
// Picker - the second usecase.Picker implementation, behind
// ORCHESTRA_PICKER=jev - can name the same three built-ins in its own
// criteria map without inventing a second copy of the ids.
const (
	IDListCapabilities = "list_capabilities"
	IDProposePanel     = "propose_panel"
	IDNone             = "none"
)

// The three fixed built-ins' own Japanese phrasing, exported for the same
// reason the ids above are: internal/adapter/planner/jev's Picker shows
// them as criteria descriptions rather than as candidateLine's
// tab-separated column, but the wording itself - and the reasoning above
// for why it reads the way it does - must stay the one copy both pickers
// share.
const (
	PhraseListCapabilities = "使える操作の一覧そのものを求めている"
	PhraseProposePanel     = "画面に出したい"
	PhraseNone             = "どの候補も質問に合わない（業務と無関係な質問、候補の操作では答えられない質問）"
)

// The three fixed candidate lines, in S3's order, built from the ids and
// phrases above so the id a response is matched against (parse.go) and
// the id shown in the prompt can never drift apart. These are exactly
// Wording's v1 - DefaultWording()'s own values - kept as their own
// package-level consts (rather than only reachable through
// DefaultWording()) since prompt_internal_test.go's byte-identity
// fixtures are written against them directly.
const (
	lineListCapabilities = IDListCapabilities + "\tplatform\t" + PhraseListCapabilities
	lineProposePanel     = IDProposePanel + "\tplatform\t" + PhraseProposePanel
	lineNone             = IDNone + "\tplatform\t" + PhraseNone
)

// Wording is one named set of the pick's three built-in candidate lines'
// own Japanese phrasing (the PhraseListCapabilities/PhraseProposePanel/
// PhraseNone constants above, for v1) - added because that phrasing is
// model-specific (DECISIONS.md, 2026-09-19): tuned for qwen3.5-9b-q8 (34/34
// on ORCHESTRA_PLANNER_STAGES=2 make eval), the same text reads
// bonsai2-27b (a ternary 27B served through llama-swap) at 30/34, three of
// its four losses the same shape - list_capabilities answered where a
// create form, an ask or none was wanted (real-report-tardiness,
// real-apply-paid-leave, real-decrease-inventory).
//
// Named, selected by ORCHESTRA_PICK_WORDING (internal/infra/config),
// exactly the shape internal/adapter/planner/wording already gives the
// toolcall planner's own words: a named set, DefaultWording(), selection
// by env var, an unknown name failing startup naming every known set
// (WordingNames()). A separate type from that package's own Wording -
// the pick offers three fixed lines, not the toolcall planner's system
// prompt/tool descriptions, and jev (internal/adapter/planner/jev) reads
// PhraseListCapabilities et al. directly rather than through this type
// (see that package's own mapping.go), so there is nothing here for the
// two to share.
type Wording struct {
	// Name selects this set via ORCHESTRA_PICK_WORDING.
	Name string
	// SystemPrompt is the pick's own system message under this set - v1's
	// value is SystemPrompt (the package const) byte for byte; Pick sends
	// this field, never the package const directly, so a named set can
	// change it (v3-verb, v4-specific) without a second call site to keep
	// in sync.
	SystemPrompt string
	// ListCapabilities is lineListCapabilities' phrase column under this
	// set.
	ListCapabilities string
	// ProposePanel is lineProposePanel's phrase column under this set.
	ProposePanel string
	// None is lineNone's phrase column under this set.
	None string
}

// wordingV1Name is "v1".
const wordingV1Name = "v1"

// v1Wording is the text in the product today, byte for byte - built from
// the same PhraseListCapabilities/PhraseProposePanel/PhraseNone constants
// lineListCapabilities/lineProposePanel/lineNone above are, so the two can
// never drift apart. A function, not a package-level value, for the same
// gochecknoglobals reason internal/adapter/planner/wording.v1 is
// (harness/quality/go/golangci.yml).
func v1Wording() Wording {
	return Wording{
		Name:             wordingV1Name,
		SystemPrompt:     SystemPrompt,
		ListCapabilities: PhraseListCapabilities,
		ProposePanel:     PhraseProposePanel,
		None:             PhraseNone,
	}
}

// wordingV2StrictCapabilitiesName is "v2-strict-capabilities".
const wordingV2StrictCapabilitiesName = "v2-strict-capabilities"

// v2StrictCapabilitiesListCapabilities extends v1's own
// PhraseListCapabilities with an explicit exclusion, the same shape
// PhraseNone already uses for its own "not this" clause: a question that
// names a specific action - creating, recording, applying for something -
// should not read as a request for the capabilities list just because
// list_capabilities is the built-in the model reaches for when unsure.
// bonsai2-27b's three same-shaped losses (v1Wording's own doc comment)
// were exactly this: real-report-tardiness (遅刻を報告したい),
// real-apply-paid-leave (有給を申請したい) and real-decrease-inventory
// (在庫を減らしたい, which produced a form instead) all name an action,
// yet the model answered list_capabilities. A genuine capability question
// naming no action (在庫で何ができる？) still reads the first sentence
// unchanged.
const v2StrictCapabilitiesListCapabilities = PhraseListCapabilities +
	"。作成・登録・申請など、特定の操作を行いたい質問には使わない。"

// v2StrictCapabilities is a second, independent attempt at
// ListCapabilities' own phrasing, for a model other than the one v1 is
// tuned for (Wording's own doc comment) - ProposePanel and None are left
// exactly v1's, since bonsai2-27b's losses never touched those two lines.
func v2StrictCapabilities() Wording {
	return Wording{
		Name:             wordingV2StrictCapabilitiesName,
		SystemPrompt:     SystemPrompt,
		ListCapabilities: v2StrictCapabilitiesListCapabilities,
		ProposePanel:     PhraseProposePanel,
		None:             PhraseNone,
	}
}

// wordingV3VerbName is "v3-verb".
const wordingV3VerbName = "v3-verb"

// v3VerbSystemPromptAddition targets the same "wrong verb" shape
// internal/adapter/planner/wording's v7-verb (v7_verb.go) was written
// against, for the pick stage instead of the fill stage: a question that
// asks for something to be recorded or requested for the first time was
// answered with the operation that reads or edits an existing record of the
// same resource, because that regression is decided here - which operation
// gets chosen - not in the fill stage's own system prompt (DECISIONS.md,
// v7-verb and v8-specific producing byte-identical fill-stage answers on
// all 50 axis D/E questions because the pick stage's prompt was never
// touched). The sentence names no resource, service or example question -
// it states the general rule only, appended after SystemPrompt's own last
// instruction.
const v3VerbSystemPromptAddition = "\n\n操作を選ぶときは質問が求める動詞に合わせ、まだ存在しないものの記録や申請を求める質問には、" +
	"それを参照・更新する操作ではなく、新しく作成する操作を選ぶこと。"

// v3VerbSystemPrompt is SystemPrompt with v3VerbSystemPromptAddition
// appended - v1's value plus exactly this one sentence, nothing else.
const v3VerbSystemPrompt = SystemPrompt + v3VerbSystemPromptAddition

// v3Verb targets the "wrong verb" shape in the pick's own choice: the right
// resource, the wrong operation on it (a read or an update where the
// question asked for a creation). It differs from v1 by exactly one
// sentence appended to SystemPrompt - ListCapabilities, ProposePanel and
// None are v1's, unchanged.
func v3Verb() Wording {
	return Wording{
		Name:             wordingV3VerbName,
		SystemPrompt:     v3VerbSystemPrompt,
		ListCapabilities: PhraseListCapabilities,
		ProposePanel:     PhraseProposePanel,
		None:             PhraseNone,
	}
}

// wordingV4SpecificName is "v4-specific".
const wordingV4SpecificName = "v4-specific"

// v4SpecificSystemPromptAddition targets the same "neighbouring or more
// general resource" shape internal/adapter/planner/wording's v8-specific
// (v8_specific.go) was written against, for the pick stage instead of the
// fill stage (see v3VerbSystemPromptAddition's own doc comment for why the
// pick stage is where this choice is actually made): the candidate list
// holds both a general resource and a more specific one that would also
// fit, and the general one gets picked instead of the one the question
// actually names. The sentence names no resource, service or example
// question - it states the general rule only, appended after SystemPrompt's
// own last instruction.
const v4SpecificSystemPromptAddition = "\n\n候補に対象を広く扱う操作とより具体的な対象を扱う操作の両方があり、" +
	"どちらも条件に合いそうな場合は、それも該当するだけの広い操作ではなく、質問が名指ししている具体的な対象の操作を選ぶこと。"

// v4SpecificSystemPrompt is SystemPrompt with v4SpecificSystemPromptAddition
// appended - v1's value plus exactly this one sentence, nothing else.
const v4SpecificSystemPrompt = SystemPrompt + v4SpecificSystemPromptAddition

// v4Specific targets the "neighbouring or more general resource" shape in
// the pick's own choice: the right family, the wrong member, because a more
// general operation also technically fits. It differs from v1 by exactly
// one sentence appended to SystemPrompt - ListCapabilities, ProposePanel
// and None are v1's, unchanged.
func v4Specific() Wording {
	return Wording{
		Name:             wordingV4SpecificName,
		SystemPrompt:     v4SpecificSystemPrompt,
		ListCapabilities: PhraseListCapabilities,
		ProposePanel:     PhraseProposePanel,
		None:             PhraseNone,
	}
}

// allWordings lists every named Wording this package declares, in the
// order WordingNames reports them - v1 first, since it is DefaultWording
// and every candidate is written as a delta from it (v1Wording's,
// v2StrictCapabilities', v3Verb's and v4Specific's own doc comments).
func allWordings() []Wording {
	return []Wording{v1Wording(), v2StrictCapabilities(), v3Verb(), v4Specific()}
}

// DefaultWording is v1: today's text, byte for byte, selected whenever
// ORCHESTRA_PICK_WORDING is unset.
func DefaultWording() Wording {
	return v1Wording()
}

// WordingByName looks a set up by Wording.Name, reporting false when name
// is not one allWordings declares - the shape ORCHESTRA_PICK_WORDING's
// validation (internal/infra/config) and pkg/app both need for an unknown
// name to fail startup.
func WordingByName(name string) (Wording, bool) {
	for _, w := range allWordings() {
		if w.Name == name {
			return w, true
		}
	}

	return Wording{}, false
}

// WordingNames lists every set's Name, in allWordings' declared order -
// what an unknown ORCHESTRA_PICK_WORDING's startup error lists.
func WordingNames() []string {
	ws := allWordings()

	names := make([]string, len(ws))
	for i, w := range ws {
		names[i] = w.Name
	}

	return names
}

// builtinLine renders one built-in candidate's own "id\tplatform\tphrase"
// line - the same shape candidateLine gives a shortlist endpoint, except
// the service column is always platformService's own display value
// ("platform", candidatesFor's own constant) since none of the three
// built-ins belongs to a configured service.
func builtinLine(id, phrase string) string {
	return id + "\tplatform\t" + phrase
}

// summaryFor is an endpoint's summary column: its own Summary, or - when
// that is empty - the first line of its Description.
//
// Deviation from docs/plans/staging.md's Task 1: the plan says to "reuse
// the helper [toolFor] uses" for this fallback, but toolFor
// (internal/usecase/tools.go) has no such fallback - it reads e.Summary
// directly. There is nothing to reuse, so this is a new, small helper
// local to this package rather than a second, independently-invented copy
// placed in usecase.
func summaryFor(e *domain.Endpoint) string {
	if e.Summary != "" {
		return e.Summary
	}

	first, _, _ := strings.Cut(e.Description, "\n")

	return first
}

// candidateLine renders one shortlist endpoint as the pick's own
// "operationId\tserviceDisplayName\tsummary" line (docs/specs/staging.md,
// section 4) - byte for byte what candidateLine builds in
// e2e/narrowing/pick/client.ts, examples column omitted (that column is
// never shown to the pick - section 2, S2).
func candidateLine(e *domain.Endpoint) string {
	return e.OperationID + "\t" + e.ServiceDisplayNameOr(e.Service) + "\t" + summaryFor(e)
}

// builtinLineCount is how many fixed lines S3 appends after the
// shortlist - named so the capacity hint below isn't a bare magic number
// (mnd, harness/quality/go/golangci.yml), the same reason
// usecase.builtinToolCount is named.
const builtinLineCount = 3

// userMessage builds the pick's one user message: the framing line, then -
// only when answers is non-empty (added 2026-09-16 alongside the ask_user
// degradation fix, docs/specs/staging.md section 4) - one "回答:
// <param>=<value>" line per answer, then - only when turns is non-empty
// (added 2026-09-17, docs/measurements/jev-v5.md's isolation result: giving
// the pick stage the turns recovered follow-up-other-service from 0/10 to
// 10/10 for the Jev picker) - one "直前: ..." line per turn (turnLines),
// then the blank line, one candidate line per shortlist endpoint in
// shortlist order, then the three fixed lines of S3.
//
// When both answers and turns are empty, offerProposePanel is true and w is
// DefaultWording() this must build byte-identical output to before either
// parameter existed: AC-S-103's own measurement, and the stages2 comparison
// it feeds, both depend on the pick seeing exactly the same prompt it
// always has whenever there is nothing new to tell it -
// TestUserMessageWithNoAnswersOrTurnsIsByteIdenticalToBeforeTheyExisted
// (prompt_internal_test.go) and TestPickByteIdenticalWithNoTurns
// (picker_test.go) are the regression guards for that. offerProposePanel is
// Picker.Pick's own O3 switch (docs/specs/offering.md - the same condition
// usecase.ToolsFor's own appliesFromWorkspace already applies to the
// built-in tool of the same name): the propose_panel line is left out of
// the candidate list entirely, not merely described as unavailable,
// whenever it is false. w selects the three built-ins' own phrasing
// (Wording, ORCHESTRA_PICK_WORDING) - the ids themselves never change.
func userMessage(
	query string, answers []usecase.Answer, turns []usecase.Turn, shortlist domain.Catalog, offerProposePanel bool,
	w *Wording,
) string {
	lines := make([]string, 0, len(shortlist.Endpoints)+builtinLineCount)

	for i := range shortlist.Endpoints {
		lines = append(lines, candidateLine(&shortlist.Endpoints[i]))
	}

	lines = append(lines, builtinLine(IDListCapabilities, w.ListCapabilities))

	if offerProposePanel {
		lines = append(lines, builtinLine(IDProposePanel, w.ProposePanel))
	}

	lines = append(lines, builtinLine(IDNone, w.None))

	return "質問: " + query + "\n" + answerLines(answers) + turnLines(turns, shortlist) + "\n候補:\n" +
		strings.Join(lines, "\n")
}

// turnLine renders one turn in the pick's own terse style (the same style
// answerLines uses for 回答 lines): "直前: <serviceDisplayName> / <operation
// display name>（<the person's question>）". shortlist - the pick's own
// narrowed catalogue, not the full one - is searched for the turn's
// (Service, OperationID) to read its display names; when not found (a
// narrowing has since dropped the operation from the shortlist, or the turn
// has none at all, e.g. a past ask/none) turnLine falls back to the turn's
// own raw Service/OperationID, the same fallback-to-raw pattern
// internal/adapter/planner/jev/mapping.go's turnsFor uses for the same
// lookup.
func turnLine(t usecase.Turn, shortlist domain.Catalog) string {
	service, operation := t.Service, t.OperationID

	if e, ok := shortlist.Find(t.Service, t.OperationID); ok {
		service = e.ServiceDisplayNameOr(t.Service)
		operation = e.DisplayNameOr(t.OperationID)
	}

	return "直前: " + service + " / " + operation + "（" + t.Question + "）"
}

// turnLines renders turns as turnLine lines, oldest first (the order
// truncateTurns, internal/usecase/orchestrator.go, already leaves them in),
// each terminated by its own newline - the same shape answerLines gives
// answers, so userMessage's surrounding "\n" + ... + "\n候補:" keeps
// producing exactly one blank line before 候補: regardless of how many of
// answers and turns are non-empty.
func turnLines(turns []usecase.Turn, shortlist domain.Catalog) string {
	if len(turns) == 0 {
		return ""
	}

	lines := make([]string, len(turns))
	for i, t := range turns {
		lines[i] = turnLine(t, shortlist)
	}

	return strings.Join(lines, "\n") + "\n"
}

// answerLines renders answers as the pick's own "回答: <param>=<value>"
// lines, one per answer, each terminated by its own newline so
// userMessage's surrounding "\n" + ... + "\n候補:" produces exactly one
// blank line before 候補: whether or not there are any - empty answers
// yields "", collapsing the two adjacent "\n"s in userMessage into the same
// single blank line the byte-identical empty case has always had.
func answerLines(answers []usecase.Answer) string {
	if len(answers) == 0 {
		return ""
	}

	lines := make([]string, len(answers))
	for i, a := range answers {
		lines[i] = "回答: " + a.Param + "=" + a.Value
	}

	return strings.Join(lines, "\n") + "\n"
}
