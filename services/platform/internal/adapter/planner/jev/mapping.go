package jev

import (
	"strings"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/pick"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// instructionsQuestion is the "pick" question's own framing - the same
// opening sentence this package's instructions carried before the v5
// trial (docs/measurements/jev-picker-v5.md), now one field of
// instructionsFor's object rather than the head of one concatenated
// paragraph.
const instructionsQuestion = "社内APIの振り分け役。候補の中から呼ぶべき操作を1つ選ぶ。"

// instructionsFocus is the v5 trial's own addition
// (docs/measurements/jev-picker-v5.md, "what the API offers that v1-v4
// did not use", point 3): v2 measured a new failure mode - Jev naming a
// get* (singular-record) operation where a list*/search*/summarize*/
// aggregate* one was right, or the reverse - that notForV2 was never
// built to catch, since it is not a cross-service collision at all. A
// second instructions field named for exactly this is the lever
// docs.typesafe.ai/primitives/choice's free-form instructions object
// invites for it.
const instructionsFocus = "質問が求めている操作そのものに注目すること。動詞（一覧/詳細/作成/更新/削除）と" +
	"対象resourceの両方を、選ぶ候補と一致させる。"

// instructionsBuiltins spells out what each of the three built-ins is
// for - byte for byte the sentence this package's pre-v5 instructions
// constant carried after its own framing sentence, moved into its own
// object field.
const instructionsBuiltins = "list_capabilitiesは「何ができるか」を尋ねる質問のとき、propose_panelは画面に何かを出したい質問のとき、" +
	"noneはどの候補も質問に合わない、または質問が業務と無関係なときに選ぶ。"

// instructionsNoteV2 is CriteriaV2's own addition, unchanged since v2
// (docs/measurements/jev-picker-v2.md, section 0): what a criterion's
// examples and not_for fields mean, since Jev is never told a field's own
// name carries meaning beyond its value
// (docs.typesafe.ai/primitives/choice).
const instructionsNoteV2 = "候補には examples（その操作に対して人がよく尋ねる質問）と" +
	"not_for（混同しやすい別の操作）がある。"

// instructionsContext is the v5 trial's own hypothesis
// (docs/measurements/jev-picker-v5.md), stated to the model rather than
// left implicit - point 4, "backtick state paths"
// (docs.typesafe.ai/primitives/choice documents instructions referencing
// structured state paths such as `state.turns[0].question`). Added only
// when turns is non-empty (instructionsFor), so a request with no turns
// never gains a field pointing at an array that would be empty.
const instructionsContext = "直前の会話は `state.turns` にある。そこで扱った操作の続きなら、" +
	"その操作か同じ資源の別操作を選ぶ。"

// wireInstructionsObject is the "pick" question's own instructions value: an
// object, not this package's pre-v5 plain string - field names are
// free-form and read by the model as data
// (docs.typesafe.ai/primitives/choice: "The field names ... are not part
// of the API, and none are reserved. You choose them"), chosen here for
// what each says.
type wireInstructionsObject struct {
	Question string `json:"question"`
	Focus    string `json:"focus"`
	Builtins string `json:"builtins"`
	Note     string `json:"note,omitempty"`
	Context  string `json:"context,omitempty"`
}

// legacyInstructions and legacyInstructionsV2 are byte-identical to this
// package's own pre-v5 "instructions"/"instructionsV2" constants
// (docs/measurements/jev-picker-v2.md) - kept only for
// WithLegacyInstructions, run B of the v5 trial's own isolation
// (docs/measurements/jev-v5.md): turns still reach "state" unchanged, but
// "pick"'s own instructions revert to the plain string v1/v2 always sent,
// to tell the v5 round's object instructions apart from its turns-in-state
// change as the cause of real-attendance-detail's new regression.
const legacyInstructions = "社内APIの振り分け役。質問に対して、候補の中から呼ぶべき操作を1つ選ぶ。" +
	"list_capabilitiesは「何ができるか」を尋ねる質問のとき、propose_panelは画面に何かを出したい質問のとき、" +
	"noneはどの候補も質問に合わない、または質問が業務と無関係なときに選ぶ。"

const legacyInstructionsV2 = legacyInstructions +
	"候補には examples（その操作に対して人がよく尋ねる質問）と" +
	"not_for（混同しやすい別の操作）がある。"

// legacyInstructionsFor returns legacyInstructions or legacyInstructionsV2
// depending on criteria - WithLegacyInstructions' own counterpart to
// instructionsFor.
func legacyInstructionsFor(criteria string) string {
	if criteria == CriteriaV2 {
		return legacyInstructionsV2
	}

	return legacyInstructions
}

// instructionsFor builds the "pick" question's instructions object:
// instructionsNoteV2 only under CriteriaV2, instructionsContext only when
// hasTurns (the request carries a non-empty turns list).
func instructionsFor(criteria string, hasTurns bool) wireInstructionsObject {
	wi := wireInstructionsObject{Question: instructionsQuestion, Focus: instructionsFocus, Builtins: instructionsBuiltins}

	if criteria == CriteriaV2 {
		wi.Note = instructionsNoteV2
	}

	if hasTurns {
		wi.Context = instructionsContext
	}

	return wi
}

// impossibleQuestionName is the fan-out question WithFanOutGate adds
// alongside questionName in the same request
// (docs/measurements/jev-picker-v5.md, "fan-out, not extra calls": Jev
// evaluates every question of one request in parallel, and counts input
// tokens once per request, so this is not a second call the way gate.go's
// own standalone Gate.Gate makes one).
const impossibleQuestionName = "impossible"

// impossibleInstructions is the fan-out "noul" question's own framing -
// the same judgment gate.go's gateInstructions asks, worded for its own
// true/false criteria (impossibleCriteria) rather than a bare yes/no.
const impossibleInstructions = "この質問は、列挙された候補のどれでも実現できないことを求めているか" +
	"（例: 一覧しかない資源の集計・承認・印刷、候補に無い資源、業務と無関係な話題）。" +
	"能力を尋ねる質問（何ができる？）はfalse。"

// impossibleCriteria is the fan-out "noul" question's own criteria object
// (docs.typesafe.ai/primitives/noul; point 2 of
// docs/measurements/jev-picker-v5.md's "what the API offers that v1-v4
// did not use" - v3's own gateInstructions gave noul only instructions,
// never criteria).
func impossibleCriteria() map[string]string {
	return map[string]string{
		"true":  "質問が求める操作が候補一覧に無い（一覧しかない資源の集計・承認・印刷、一覧に無い資源、業務と無関係）",
		"false": "候補のどれかで答えられる、または「何ができるか」を尋ねている",
	}
}

// questionName is the "pick" key this adapter's wireRequest.Questions and
// wireResponse.Answers use.
const questionName = "pick"

// builtinCriteriaCount is how many fixed entries criteriaFor appends
// after the shortlist, named so the capacity hint below isn't a bare
// magic number (mnd, harness/quality/go/golangci.yml).
const builtinCriteriaCount = 3

// summaryFor is an endpoint's summary column: its own Summary, or - when
// that is empty - the first line of its Description. Byte for byte the
// same fallback internal/adapter/planner/pick/prompt.go's own summaryFor
// uses; not exported there, so kept here as its own small copy rather
// than a cross-package dependency for one two-line helper.
func summaryFor(e *domain.Endpoint) string {
	if e.Summary != "" {
		return e.Summary
	}

	first, _, _ := strings.Cut(e.Description, "\n")

	return first
}

// criterionV2 is one CriteriaV2 entry: the object form of a criterion
// Jev's "choice" primitive accepts (docs.typesafe.ai/primitives/choice),
// carrying more than CriteriaV1's single descriptive line. Fields with
// nothing to say are left zero and omitted from the wire (omitempty) -
// Jev's own docs give every field as optional and free-form, so an empty
// "not_for" is left off rather than sent as "".
type criterionV2 struct {
	What     string   `json:"what"`
	Examples []string `json:"examples,omitempty"`
	NotFor   string   `json:"not_for,omitempty"`
}

// noneExamplesV2 returns CriteriaV2's "none" criterion's own examples:
// two questions with nothing to do with any service's business domain,
// plus one naming a verb this catalogue has no operation for at all (a
// restart is never something any of these services exposes, whatever
// the shortlist), so "none" reads as "out of scope", not merely "no
// close match". A function, not a package-level slice
// (gochecknoglobals, harness/quality/go/golangci.yml) - a slice literal
// would be a mutable global shared by every criteriaForV2 call.
func noneExamplesV2() []string {
	return []string{"今日の天気は？", "好きな食べ物は何？", "システムを再起動して"}
}

// whatForV2 is one shortlist endpoint's CriteriaV2 "what": the same
// "<serviceDisplayName> / <summary>" line criteriaFor sends for CriteriaV1,
// plus the operation's own Description's first line, appended only when
// it says something summaryFor's own line does not already - Description
// is OpenAPI free text an author can leave empty, or can write as a
// restatement of Summary, and either way a line that adds nothing is
// worth omitting rather than padding every criterion alike.
func whatForV2(e *domain.Endpoint) string {
	what := e.ServiceDisplayNameOr(e.Service) + " / " + summaryFor(e)

	descLine, _, _ := strings.Cut(e.Description, "\n")
	if descLine == "" || strings.Contains(what, descLine) {
		return what
	}

	return what + "。" + descLine
}

// noun keys criteriaForV2's collision grouping (see notForV2): an
// endpoint's own DisplayName when the contract declares one - already a
// short business term, e.g. "承認" - or otherwise its summary cut at the
// first Japanese particle among の/を/に/が/は/で, e.g. "在庫の一覧を返す"
// -> "在庫". Two endpoints across different services sharing this term
// are the mechanical definition of "easily confused" this package uses;
// nothing here reads meaning into near-synonyms (受注 and 発注 do not
// collide unless they are also given the same DisplayName) - a
// deliberately narrow, literal rule over a fuzzy one.
func noun(e *domain.Endpoint) string {
	if e.DisplayName != "" {
		return e.DisplayName
	}

	summary := summaryFor(e)
	if i := strings.IndexAny(summary, "のをにがはで"); i > 0 {
		return summary[:i]
	}

	return summary
}

// notForV2 builds one shortlist endpoint's CriteriaV2 "not_for": the
// service display names of every other shortlist entry that shares this
// endpoint's own noun (see noun) but belongs to a different service -
// the cross-service homonym collision v1's flat criteria line never
// named at all. Endpoints in the same service as e, or sharing no noun
// with it, contribute nothing; an endpoint with no collision at all gets
// "" (criterionV2's own omitempty drops the field entirely).
func notForV2(e *domain.Endpoint, shortlist domain.Catalog) string {
	own := noun(e)

	var siblings []string

	seen := map[string]bool{}

	for i := range shortlist.Endpoints {
		other := &shortlist.Endpoints[i]
		if other.Service == e.Service || noun(other) != own {
			continue
		}

		name := other.ServiceDisplayNameOr(other.Service)
		if seen[name] {
			continue
		}

		seen[name] = true

		siblings = append(siblings, name+"の"+own+"ではない")
	}

	return strings.Join(siblings, "、")
}

// criteriaForV2 builds the "pick" question's CriteriaV2 criteria: one
// object per shortlist endpoint (whatForV2, the endpoint's own
// x-orchestra-examples when it declares any, notForV2), then the three
// fixed built-ins with their v1 phrases as "what", "none" and
// "list_capabilities" given their own fixed examples (noneExamplesV2,
// one 「何ができるの？」) - the CriteriaV2 counterpart to criteriaFor.
func criteriaForV2(shortlist domain.Catalog) map[string]criterionV2 {
	criteria := make(map[string]criterionV2, len(shortlist.Endpoints)+builtinCriteriaCount)

	for i := range shortlist.Endpoints {
		e := &shortlist.Endpoints[i]
		criteria[e.OperationID] = criterionV2{
			What:     whatForV2(e),
			Examples: e.Examples,
			NotFor:   notForV2(e, shortlist),
		}
	}

	criteria[pick.IDListCapabilities] = criterionV2{
		What: pick.PhraseListCapabilities, Examples: []string{"何ができるの？"},
	}
	criteria[pick.IDProposePanel] = criterionV2{What: pick.PhraseProposePanel}
	criteria[pick.IDNone] = criterionV2{What: pick.PhraseNone, Examples: noneExamplesV2()}

	return criteria
}

// criteriaFor builds the "pick" question's criteria: one entry per
// shortlist endpoint, keyed by its own OperationID, valued
// "<serviceDisplayName> / <summary>" - the same two columns
// pick.candidateLine shows the local picker - then the three fixed
// built-ins, keyed and phrased exactly as pick.IDListCapabilities et al.
func criteriaFor(shortlist domain.Catalog) map[string]string {
	criteria := make(map[string]string, len(shortlist.Endpoints)+builtinCriteriaCount)

	for i := range shortlist.Endpoints {
		e := &shortlist.Endpoints[i]
		criteria[e.OperationID] = e.ServiceDisplayNameOr(e.Service) + " / " + summaryFor(e)
	}

	criteria[pick.IDListCapabilities] = pick.PhraseListCapabilities
	criteria[pick.IDProposePanel] = pick.PhraseProposePanel
	criteria[pick.IDNone] = pick.PhraseNone

	return criteria
}

// answerLinesFor renders answers as one "回答: <param>=<value>" line per
// answer, the same lines internal/adapter/planner/pick/prompt.go's own
// answerLines appends to the local picker's own user message. Always a
// non-nil slice (len(answers)==0 gives a non-nil, zero-length one, so
// wireStateWithTurns.Answers marshals as "[]", never "null") - stateFor
// and stateValue are its only two callers.
func answerLinesFor(answers []usecase.Answer) []string {
	lines := make([]string, len(answers))
	for i, a := range answers {
		lines[i] = "回答: " + a.Param + "=" + a.Value
	}

	return lines
}

// stateFor builds the "state" Jev is asked about when there are no turns
// to report: query alone, or - when answers is non-empty - query followed
// by one "回答: <param>=<value>" line per answer (answerLinesFor). Byte
// for byte what every request this adapter sent before the v5 trial built
// (docs/measurements/jev-picker-v5.md) - stateValue's own no-turns branch.
func stateFor(query string, answers []usecase.Answer) string {
	lines := answerLinesFor(answers)
	if len(lines) == 0 {
		return query
	}

	return query + "\n" + strings.Join(lines, "\n")
}

// wireTurn is one entry of wireStateWithTurns.Turns: a prior turn's own
// question, and - when the turn named an operation - the service and
// operation display names the shortlist's own criteria lines already use
// (whatForV2/criteriaFor's own "<serviceDisplayName> / ..."), so Jev reads
// the same vocabulary for a turn's operation as it does for a candidate's.
// Service and Operation are both omitted (omitempty) for a turn with no
// operation of its own (usecase.Turn.Service == "", e.g. a past "ask" or
// "none") - turnsFor never sets one without the other.
type wireTurn struct {
	Question  string `json:"question"`
	Service   string `json:"service,omitempty"`
	Operation string `json:"operation,omitempty"`
}

// turnsFor builds wireStateWithTurns.Turns: one wireTurn per turn, in
// order. shortlist is searched for each turn's own (Service, OperationID)
// to read its display names (Endpoint.ServiceDisplayNameOr/DisplayNameOr,
// the same fallback-to-raw-id pattern
// internal/usecase/orchestrator.go's own serviceDisplay/operationDisplay
// use); shortlist is the pick's own narrowed catalogue, not the full one,
// so a turn naming an operation idAffinity or narrowing has since dropped
// from it falls back to the turn's own raw Service/OperationID rather
// than losing the operation entirely - still enough for Jev to tell "the
// same operation" from "a different one".
func turnsFor(turns []usecase.Turn, shortlist domain.Catalog) []wireTurn {
	if len(turns) == 0 {
		return nil
	}

	wireTurns := make([]wireTurn, len(turns))

	for i, t := range turns {
		wt := wireTurn{Question: t.Question}

		if t.Service != "" {
			wt.Service, wt.Operation = t.Service, t.OperationID

			if e, ok := shortlist.Find(t.Service, t.OperationID); ok {
				wt.Service = e.ServiceDisplayNameOr(t.Service)
				wt.Operation = e.DisplayNameOr(t.OperationID)
			}
		}

		wireTurns[i] = wt
	}

	return wireTurns
}

// wireStateWithTurns is the "state" Jev is asked about when turns is
// non-empty: an object, not stateFor's plain string - Question and
// Answers carry exactly what stateFor would have folded into one string
// (Answers as answerLinesFor's own lines, not stateFor's joined text, so
// Turns can sit beside them rather than after them in one blob), plus
// Turns (turnsFor). instructionsContext's own `state.turns[0].question`
// reference (docs.typesafe.ai/primitives/choice) is this field.
type wireStateWithTurns struct {
	Question string     `json:"question"`
	Answers  []string   `json:"answers"`
	Turns    []wireTurn `json:"turns"`
}

// stateValue builds wireRequest.State: stateFor's plain string when turns
// is empty (byte-identical to every pre-v5 request,
// docs/measurements/jev-picker-v5.md), or a wireStateWithTurns object
// otherwise.
func stateValue(query string, answers []usecase.Answer, turns []usecase.Turn, shortlist domain.Catalog) any {
	if len(turns) == 0 {
		return stateFor(query, answers)
	}

	return wireStateWithTurns{Question: query, Answers: answerLinesFor(answers), Turns: turnsFor(turns, shortlist)}
}

// buildRequest builds the one wireRequest Picker.Pick sends for query,
// answers, turns and shortlist. criteria selects CriteriaV1 (criteriaFor)
// or CriteriaV2 (criteriaForV2); anything other than CriteriaV2 -
// including "", Picker.criteria's zero value - is CriteriaV1, matching
// WithCriteria's own fallback. instructionsFor folds criteria and
// len(turns)>0 into the "pick" question's own instructions object, unless
// legacyInstructions is true (WithLegacyInstructions), in which case
// legacyInstructionsFor's plain string is sent instead - state still
// carries turns exactly as it does when legacyInstructions is false; only
// the "pick" question's own instructions value changes.
func buildRequest(
	query string, answers []usecase.Answer, turns []usecase.Turn, shortlist domain.Catalog,
	criteria string, legacyInstructions bool,
) wireRequest {
	var wireCriteria any = criteriaFor(shortlist)

	if criteria == CriteriaV2 {
		wireCriteria = criteriaForV2(shortlist)
	}

	var wireQuestionInstructions any = instructionsFor(criteria, len(turns) > 0)
	if legacyInstructions {
		wireQuestionInstructions = legacyInstructionsFor(criteria)
	}

	return wireRequest{
		State: stateValue(query, answers, turns, shortlist),
		Model: modelName,
		Questions: map[string]wireQuestion{
			questionName: {
				Type:         "choice",
				Instructions: wireQuestionInstructions,
				Criteria:     wireCriteria,
			},
		},
	}
}

// addImpossibleQuestion adds the fan-out "impossible" question
// (impossibleQuestionName, impossibleInstructions, impossibleCriteria) to
// req.Questions - Picker.Pick's own WithFanOutGate path, called after
// buildRequest rather than folded into it, so buildRequest's own golden
// tests (the "pick" question alone) stay unaffected by whether fan-out is
// on.
func addImpossibleQuestion(req *wireRequest) {
	req.Questions[impossibleQuestionName] = wireQuestion{
		Type: "noul", Instructions: impossibleInstructions, Criteria: impossibleCriteria(),
	}
}

// serviceFor looks up which service a shortlist endpoint's operationID
// belongs to.
func serviceFor(shortlist domain.Catalog, operationID string) (string, bool) {
	for i := range shortlist.Endpoints {
		if shortlist.Endpoints[i].OperationID == operationID {
			return shortlist.Endpoints[i].Service, true
		}
	}

	return "", false
}

// mapAnswer turns Jev's "pick" answer into a usecase.Pick: choice maps to
// one of the three built-ins, or to a shortlist endpoint's own
// (Service, OperationID), exactly as
// internal/adapter/planner/pick/parse.go's own parse resolves the local
// picker's answer. Ambiguous is confidence below threshold (S4).
// matched is false only when choice named neither a built-in nor any
// shortlist endpoint - the caller logs that case; this function stays a
// pure mapping with nothing to log to.
func mapAnswer(answer wireAnswer, shortlist domain.Catalog, threshold float64) (usecase.Pick, bool) {
	ambiguous := answer.Confidence < threshold

	switch answer.Choice {
	case pick.IDListCapabilities:
		return usecase.Pick{Kind: usecase.PickListCapabilities, Ambiguous: ambiguous}, true
	case pick.IDProposePanel:
		return usecase.Pick{Kind: usecase.PickProposePanel, Ambiguous: ambiguous}, true
	case pick.IDNone:
		return usecase.Pick{Kind: usecase.PickNone, Ambiguous: ambiguous}, true
	default:
		if service, ok := serviceFor(shortlist, answer.Choice); ok {
			return usecase.Pick{
				Kind: usecase.PickOperation, Service: service, OperationID: answer.Choice, Ambiguous: ambiguous,
			}, true
		}

		return usecase.Pick{Kind: usecase.PickNone, Ambiguous: ambiguous}, false
	}
}
