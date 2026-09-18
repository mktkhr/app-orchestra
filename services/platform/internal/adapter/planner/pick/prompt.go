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
// the id shown in the prompt can never drift apart.
const (
	lineListCapabilities = IDListCapabilities + "\tplatform\t" + PhraseListCapabilities
	lineProposePanel     = IDProposePanel + "\tplatform\t" + PhraseProposePanel
	lineNone             = IDNone + "\tplatform\t" + PhraseNone
)

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
// When both answers and turns are empty and offerProposePanel is true this
// must build byte-identical output to before either parameter existed:
// AC-S-103's own measurement, and the stages2 comparison it feeds, both
// depend on the pick seeing exactly the same prompt it always has whenever
// there is nothing new to tell it -
// TestUserMessageWithNoAnswersOrTurnsIsByteIdenticalToBeforeTheyExisted
// (prompt_internal_test.go) and TestPickByteIdenticalWithNoTurns
// (picker_test.go) are the regression guards for that. offerProposePanel is
// Picker.Pick's own O3 switch (docs/specs/offering.md - the same condition
// usecase.ToolsFor's own appliesFromWorkspace already applies to the
// built-in tool of the same name): lineProposePanel is left out of the
// candidate list entirely, not merely described as unavailable, whenever
// it is false.
func userMessage(
	query string, answers []usecase.Answer, turns []usecase.Turn, shortlist domain.Catalog, offerProposePanel bool,
) string {
	lines := make([]string, 0, len(shortlist.Endpoints)+builtinLineCount)

	for i := range shortlist.Endpoints {
		lines = append(lines, candidateLine(&shortlist.Endpoints[i]))
	}

	lines = append(lines, lineListCapabilities)

	if offerProposePanel {
		lines = append(lines, lineProposePanel)
	}

	lines = append(lines, lineNone)

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
