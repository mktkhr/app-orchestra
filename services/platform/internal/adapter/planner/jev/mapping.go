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
// answers and shortlist.
func buildRequest(query string, answers []usecase.Answer, shortlist domain.Catalog) wireRequest {
	return wireRequest{
		State: stateFor(query, answers),
		Model: modelName,
		Questions: map[string]wireQuestion{
			questionName: {
				Type:         "choice",
				Instructions: instructions,
				Criteria:     criteriaFor(shortlist),
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
