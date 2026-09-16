// Package pick implements usecase.Picker: it sends the shortlist in the
// measured stand-in picker's own format - the one e2e/narrowing/pick/client.ts
// used to produce the 83 in docs/specs/staging.md section 1 - over
// internal/adapter/planner/chat, and parses the one line it answers with
// back into a usecase.Pick.
package pick

import (
	"strings"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
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
const (
	idListCapabilities = "list_capabilities"
	idProposePanel     = "propose_panel"
	idNone             = "none"
)

// The three fixed candidate lines, in S3's order, built from the ids above
// so the id a response is matched against (parse.go) and the id shown in
// the prompt can never drift apart.
const (
	lineListCapabilities = idListCapabilities + "\tplatform\t使える操作の一覧を知りたい"
	lineProposePanel     = idProposePanel + "\tplatform\t画面に出したい"
	lineNone             = idNone + "\tplatform\tどの候補も質問に合わない（業務と無関係な質問）"
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

// userMessage builds the pick's one user message: the framing line, one
// candidate line per shortlist endpoint in shortlist order, then the three
// fixed lines of S3, in that order.
func userMessage(query string, shortlist domain.Catalog) string {
	lines := make([]string, 0, len(shortlist.Endpoints)+builtinLineCount)

	for i := range shortlist.Endpoints {
		lines = append(lines, candidateLine(&shortlist.Endpoints[i]))
	}

	lines = append(lines, lineListCapabilities, lineProposePanel, lineNone)

	return "質問: " + query + "\n\n候補:\n" + strings.Join(lines, "\n")
}
