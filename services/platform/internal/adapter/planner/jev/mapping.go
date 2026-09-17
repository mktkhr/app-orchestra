package jev

import (
	"strings"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/pick"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// instructions is Jev's "pick" question instructions: the same framing
// pick.SystemPrompt gives the local picker, plus one sentence spelling
// out what each of the three built-ins is for - Jev is asked to choose
// from a criteria map, not to read a fixed-format prompt, so the built-
// ins need their purpose stated instead of relying on pick's own
// candidate-line wording alone.
const instructions = "社内APIの振り分け役。質問に対して、候補の中から呼ぶべき操作を1つ選ぶ。" +
	"list_capabilitiesは「何ができるか」を尋ねる質問のとき、propose_panelは画面に何かを出したい質問のとき、" +
	"noneはどの候補も質問に合わない、または質問が業務と無関係なときに選ぶ。"

// instructionsV2 is instructions plus one line spelling out the two
// extra fields CriteriaV2's criterionV2 objects carry beyond v1's plain
// string, since Jev is never told what a criterion's own field names
// mean on its own (docs.typesafe.ai/primitives/choice: "the model
// receives both field names and values").
const instructionsV2 = instructions +
	"候補には examples（その操作に対して人がよく尋ねる質問）と" +
	"not_for（混同しやすい別の操作）がある。"

// questionName is the one key this adapter's wireRequest.Questions and
// wireResponse.Answers ever use.
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

// stateFor builds the "state" Jev is asked about: query alone, or -
// when answers is non-empty - query followed by one "回答:
// <param>=<value>" line per answer, the same lines
// internal/adapter/planner/pick/prompt.go's own answerLines appends to
// the local picker's own user message.
func stateFor(query string, answers []usecase.Answer) string {
	if len(answers) == 0 {
		return query
	}

	lines := make([]string, len(answers))
	for i, a := range answers {
		lines[i] = "回答: " + a.Param + "=" + a.Value
	}

	return query + "\n" + strings.Join(lines, "\n")
}

// buildRequest builds the one wireRequest Picker.Pick sends for query,
// answers and shortlist. criteria selects CriteriaV1 (criteriaFor,
// instructions) or CriteriaV2 (criteriaForV2, instructionsV2); anything
// other than CriteriaV2 - including "", Picker.criteria's zero value -
// is CriteriaV1, matching WithCriteria's own fallback.
func buildRequest(query string, answers []usecase.Answer, shortlist domain.Catalog, criteria string) wireRequest {
	wireInstructions := instructions

	var wireCriteria any = criteriaFor(shortlist)

	if criteria == CriteriaV2 {
		wireInstructions = instructionsV2
		wireCriteria = criteriaForV2(shortlist)
	}

	return wireRequest{
		State: stateFor(query, answers),
		Model: modelName,
		Questions: map[string]wireQuestion{
			questionName: {
				Type:         "choice",
				Instructions: wireInstructions,
				Criteria:     wireCriteria,
			},
		},
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
