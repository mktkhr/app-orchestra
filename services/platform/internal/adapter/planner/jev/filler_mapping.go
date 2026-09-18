package jev

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/toolcall"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// buildFillRequest builds the one wireRequest Filler.Fill sends: one Choice
// question per endpoint.Parameters entry (questionsForFill), keyed by the
// parameter's own name - unique within one endpoint's own parameter list,
// the same scope optionsForParam and guessedEnumArgs both search
// (internal/usecase). State is toolcall.BuildUserContent's own rendering -
// dateLine, the turns the local fill would have rendered, the question and
// any prior answers - so Jev is asked about exactly the user content the
// local fill would have seen for the same question, not a second, drifting
// copy of it.
func (f *Filler) buildFillRequest(
	endpoint *domain.Endpoint, query string, answers []usecase.Answer, turns []usecase.Turn, now time.Time,
) wireRequest {
	questions := make(map[string]wireQuestion, len(endpoint.Parameters))

	for i := range endpoint.Parameters {
		p := &endpoint.Parameters[i]
		questions[p.Name] = wireQuestion{
			Type:         choiceQuestionType,
			Instructions: instructionsForParam(p),
			Criteria:     f.criteriaForParam(p),
		}
	}

	if f.refusalMode == refusalModeSeparate {
		addSeparateRefusalQuestions(questions, endpoint)
	}

	return wireRequest{
		State:     toolcall.BuildUserContent(query, answers, turns, now),
		Model:     modelName,
		Questions: questions,
	}
}

// fillRefusalQuestionName and fillCapabilitiesQuestionName are the two
// whole-request questions addSeparateRefusalQuestions adds alongside every
// parameter's own Choice question, keyed the same "one extra key in the same
// request, not a second call" way picker.go's own impossibleQuestionName is
// (docs/measurements/jev-picker-v5.md, "fan-out"): Jev evaluates every
// question of one request in parallel and counts input tokens once.
const (
	fillRefusalQuestionName      = "refusal"
	fillCapabilitiesQuestionName = "capabilities"
)

// addSeparateRefusalQuestions adds fillRefusalQuestionName and
// fillCapabilitiesQuestionName to questions as two "noul" (yes/no-with-a-
// probability) questions - refusalModeSeparate's own answer to the category
// error refusalModeInOptions makes: "the picked operation cannot answer this
// question at all" and "the question is asking what the system can do, not
// asking to run an operation" are both claims about the whole request, not
// about any one parameter's value, so they are asked as their own questions
// here rather than as extra criteria competing inside a parameter's Choice
// question (criteriaForParam, called just above in buildFillRequest, never
// adds refusalChoice/capabilitiesChoice in this mode). The shape reused is
// gate.go's own standalone "noul" question's Instructions (a plain string),
// plus picker.go's own fan-out "impossible" question's Criteria (a
// true/false object, docs.typesafe.ai/primitives/noul) - unlike gate.go's
// own gateWireQuestion, which has no Criteria field at all. The plain
// Instructions carries refusalLabel/capabilitiesLabel byte-for-byte, as
// before; Criteria is new, and is where the API actually weighs evidence
// (docs/measurements: the first cut of this mode gave the judgement nothing
// but its own instruction sentence, and its confidence sat in a narrow band
// regardless of the picked operation because it had no way to know which
// operation was even picked). operationEvidenceCriteria builds both
// questions' own Criteria from endpoint, naming the picked operation
// concretely in every branch.
func addSeparateRefusalQuestions(questions map[string]wireQuestion, endpoint *domain.Endpoint) {
	criteria := operationEvidenceCriteria(endpoint)

	questions[fillRefusalQuestionName] = wireQuestion{
		Type: noulQuestionType, Instructions: refusalLabel, Criteria: criteria.refusal,
	}
	questions[fillCapabilitiesQuestionName] = wireQuestion{
		Type: noulQuestionType, Instructions: capabilitiesLabel, Criteria: criteria.capabilities,
	}
}

// operationEvidenceFor renders the picked operation's own catalogue entry -
// the same fields the Filler already sends elsewhere in this request
// (questionsForFill's own instructionsForParam, criteriaForParam) - into one
// clause naming it concretely: its display name (falling back to its
// operation id, DisplayNameOr's own convention), its own operation id, its
// service's display name (ServiceDisplayNameOr), its summary (summaryFor,
// mapping.go's own Summary-or-first-Description-line fallback, reused
// rather than copied a third time - internal/adapter/planner/pick's own
// prompt.go keeps the first copy), and its parameters' own titles
// (questionParamTitle, the same fallback-to-name convention
// instructionsForParam already uses for the same parameters). Built once and
// shared by both refusalCriteria and capabilitiesCriteria below, so a
// deployment's two whole-request judgements always name the same operation
// the same way.
func operationEvidenceFor(e *domain.Endpoint) string {
	evidence := e.DisplayNameOr(e.OperationID) + "（操作id: " + e.OperationID +
		"、サービス: " + e.ServiceDisplayNameOr(e.Service) + "）"

	if summary := summaryFor(e); summary != "" {
		evidence += "。概要: " + summary
	}

	if len(e.Parameters) > 0 {
		titles := make([]string, len(e.Parameters))
		for i := range e.Parameters {
			titles[i] = questionParamTitle(&e.Parameters[i])
		}

		evidence += "。パラメータ: " + strings.Join(titles, "、")
	}

	return evidence
}

// separateRefusalCriteria is operationEvidenceCriteria's own result: the two
// Criteria objects addSeparateRefusalQuestions sends, named rather than
// returned as two same-typed values so golangci's gocritic (unnamedResult)
// and nonamedreturns rules (harness/quality/go/golangci.yml) do not
// disagree over how a two-map result should be spelled.
type separateRefusalCriteria struct {
	refusal      map[string]string
	capabilities map[string]string
}

// operationEvidenceCriteria builds addSeparateRefusalQuestions' own two
// Criteria objects (docs.typesafe.ai/primitives/noul's true/false form,
// the same shape mapping.go's own impossibleCriteria already sends for
// picker.go's fan-out question) - refusal's "true" and capabilities'
// "true" both describe the picked operation as unable to answer the
// question, "false" as able to, each naming it via operationEvidenceFor so
// the judgement has the evidence to weigh, not only its own instruction
// sentence.
func operationEvidenceCriteria(e *domain.Endpoint) separateRefusalCriteria {
	evidence := operationEvidenceFor(e)

	return separateRefusalCriteria{
		refusal: map[string]string{
			noulCriterionTrue:  "選ばれた操作" + evidence + "は、そもそもこの質問には答えられない（操作の対象や種類が質問と合っていない）",
			noulCriterionFalse: "選ばれた操作" + evidence + "は、この質問に答えられる",
		},
		capabilities: map[string]string{
			noulCriterionTrue: "質問は選ばれた操作" + evidence +
				"のような特定の操作の実行を求めているのではなく、そもそもこの仕組み全体で何ができるか、どんな操作があるかを尋ねている",
			noulCriterionFalse: "質問は選ばれた操作" + evidence + "のような特定の操作の実行を求めている",
		},
	}
}

// instructionsForParam builds one parameter's own Choice question
// instructions: its display title (questionParamTitle) and, when the
// contract declares one, its own schema Description - the same field
// optionsForParam's own scan reads nothing from, since Jev, unlike a local
// select, needs telling what the parameter even means.
func instructionsForParam(p *domain.Parameter) string {
	instructions := questionParamTitle(p) + "の値を、直前のやりとりから判定する。"
	if p.Schema.Description != "" {
		instructions += p.Schema.Description
	}

	return instructions
}

// criteriaForParam builds one parameter's own Choice question criteria:
// one entry per declared enum value, keyed by the value itself, described
// by its own x-enum-labels Japanese label (domain.Schema.EnumLabels) -
// falling back to the bare value for a spec-nonconformant service the same
// defensive way optionsFromSchema (internal/usecase/orchestrator_ask.go)
// already does - plus the two sentinels unsetChoice and mismatchChoice.
// refusalChoice and capabilitiesChoice are added only in refusalModeInOptions
// - never in refusalModeSeparate, where the same two judgements are asked as
// their own whole-request questions instead (addSeparateRefusalQuestions),
// so a parameter's own criteria here only ever describe that one field.
func (f *Filler) criteriaForParam(p *domain.Parameter) map[string]string {
	criteria := make(map[string]string, len(p.Schema.Enum)+sentinelCount)

	for _, value := range p.Schema.Enum {
		label := p.Schema.EnumLabels[value]
		if label == "" {
			label = value
		}

		criteria[value] = label
	}

	criteria[unsetChoice] = f.unsetLabel()
	criteria[mismatchChoice] = mismatchLabel

	if f.refusalMode == refusalModeInOptions {
		criteria[refusalChoice] = refusalLabel
		criteria[capabilitiesChoice] = capabilitiesLabel
	}

	return criteria
}

// unsetLabel is unsetChoice's own criterion text: unsetLabelWide when the
// Filler was built WithFillUnsetWordingWide, unsetLabelNarrow (the
// default) otherwise.
func (f *Filler) unsetLabel() string {
	if f.unsetWording == unsetWordingWide {
		return unsetLabelWide
	}

	return unsetLabelNarrow
}

// questionParamTitle answers one parameter's own display name the same way
// usecase.enumParamTitle does for the local fill's own guard: its schema
// Title, falling back to the bare parameter name.
func questionParamTitle(p *domain.Parameter) string {
	if p.Schema.Title != "" {
		return p.Schema.Title
	}

	return p.Name
}

// questionTitleFor searches endpoint's own parameters for name and returns
// its questionParamTitle - the same lookup-by-name scope
// usecase.enumParamTitle already applies for the local fill's own guard.
func questionTitleFor(endpoint *domain.Endpoint, name string) string {
	for i := range endpoint.Parameters {
		if endpoint.Parameters[i].Name == name {
			return questionParamTitle(&endpoint.Parameters[i])
		}
	}

	return name
}

// fillOutcome is mapFillAnswers' own result: args is the DecisionCall
// Filler.Fill returns when none of the whole-request judgements
// (refusalParam/refusalSeparate, capabilitiesParam/capabilitiesSeparate) nor
// mismatchParam fired (every parameter answered unsetChoice or a real enum
// value); mismatchParam, when non-empty, is the first parameter (in
// endpoint.Parameters order) whose answer was mismatchChoice.
// refusalParam/capabilitiesParam are refusalModeInOptions' own reading, when
// non-empty naming the first parameter whose answer was
// refusalChoice/capabilitiesChoice; refusalSeparate/capabilitiesSeparate are
// refusalModeSeparate's own reading of the two whole-request noul questions,
// true when that question's own probability was at or above threshold.
// Precedence among refusal/capabilities/mismatch, applied in decision below:
// refusal first, then capabilities, then mismatch - each of the first two is
// a claim about the whole request, not one field, and a claim about the
// whole request outranks a claim about a single parameter's value; refusal
// outranks capabilities because "this operation cannot answer the question
// at all" is the stronger of the two whole-request claims, and the ordering
// matches the local fill's own DecisionNone-over-DecisionListCapabilities
// precedent when both would apply - true regardless of which refusalMode
// produced the claim. chosen, confidence and probabilities carry every
// parameter's own raw answer; refusalProbability/capabilitiesProbability
// carry the two whole-request noul answers' own probability (0 when
// refusalMode is not refusalModeSeparate) - all read only by
// logFillCompleted.
type fillOutcome struct {
	args                    map[string]any
	mismatchParam           string
	refusalParam            string
	capabilitiesParam       string
	refusalSeparate         bool
	capabilitiesSeparate    bool
	refusalProbability      float64
	capabilitiesProbability float64
	chosen                  map[string]string
	confidence              map[string]float64
	probabilities           map[string]map[string]float64
}

// mapFillAnswers reads resp.Answers against endpoint's own parameters, and -
// in refusalModeSeparate - the two whole-request noul answers
// addSeparateRefusalQuestions asked for: false whenever any parameter's
// answer is missing entirely, or its confidence is below threshold - Fill's
// own fail-open cases - or its choice is neither a sentinel nor one of the
// parameter's own declared enum values (a shape Jev's own criteria never
// offered, so this is defensive, not an expected path); also false when
// refusalMode is refusalModeSeparate and either whole-request question's own
// answer is missing, the same "a response that does not answer what was
// asked fails open" rule applied to the extra questions this mode adds. True
// otherwise, with outcome fully built: every parameter is read once, in
// endpoint.Parameters order, so a deployment with more than one
// mismatchChoice answer always names the same one first.
func mapFillAnswers(endpoint *domain.Endpoint, resp wireResponse, threshold float64, refusalMode string) (fillOutcome, bool) {
	outcome := fillOutcome{
		args:          map[string]any{},
		chosen:        make(map[string]string, len(endpoint.Parameters)),
		confidence:    make(map[string]float64, len(endpoint.Parameters)),
		probabilities: make(map[string]map[string]float64, len(endpoint.Parameters)),
	}

	for i := range endpoint.Parameters {
		p := &endpoint.Parameters[i]

		answer, ok := resp.Answers[p.Name]
		if !ok || answer.Confidence < threshold {
			return fillOutcome{}, false
		}

		outcome.chosen[p.Name] = answer.Choice
		outcome.confidence[p.Name] = answer.Confidence
		outcome.probabilities[p.Name] = answer.Probabilities

		switch {
		case answer.Choice == unsetChoice:
			continue
		case answer.Choice == refusalChoice:
			if outcome.refusalParam == "" {
				outcome.refusalParam = p.Name
			}
		case answer.Choice == capabilitiesChoice:
			if outcome.capabilitiesParam == "" {
				outcome.capabilitiesParam = p.Name
			}
		case answer.Choice == mismatchChoice:
			if outcome.mismatchParam == "" {
				outcome.mismatchParam = p.Name
			}
		case slices.Contains(p.Schema.Enum, answer.Choice):
			outcome.args[p.Name] = answer.Choice
		default:
			return fillOutcome{}, false
		}
	}

	if refusalMode == refusalModeSeparate {
		if !applySeparateRefusalAnswers(resp, threshold, &outcome) {
			return fillOutcome{}, false
		}
	}

	if len(outcome.args) == 0 {
		outcome.args = nil
	}

	return outcome, true
}

// applySeparateRefusalAnswers reads the two whole-request noul answers
// addSeparateRefusalQuestions asked for (fillRefusalQuestionName,
// fillCapabilitiesQuestionName) into outcome, returning false when either is
// missing entirely - split out of mapFillAnswers to keep its own cognitive
// complexity under golangci's gocognit/gocyclo caps
// (harness/quality/go/golangci.yml), the same reason mapFillAnswers' own
// per-parameter loop is not further inlined into Fill.
func applySeparateRefusalAnswers(resp wireResponse, threshold float64, outcome *fillOutcome) bool {
	refusal, ok := resp.Answers[fillRefusalQuestionName]
	if !ok {
		return false
	}

	capabilities, ok := resp.Answers[fillCapabilitiesQuestionName]
	if !ok {
		return false
	}

	outcome.refusalProbability = refusal.Noul
	outcome.refusalSeparate = refusal.Noul >= threshold
	outcome.capabilitiesProbability = capabilities.Noul
	outcome.capabilitiesSeparate = capabilities.Noul >= threshold

	return true
}

// decision turns outcome into the usecase.Decision Filler.Fill returns, in
// the precedence fillOutcome's own doc comment explains (refusal, then
// capabilities, then mismatchParam, then the plain call): a DecisionNone
// when refusalParam was found or refusalSeparate is true (usecase.Decision{
// Kind: usecase.DecisionNone}, no Service/OperationID/Message - the same
// bare shape resolvePickedFill's own DecisionNone case already expects and
// turns into ResultKindNone with its usual message, exactly as it does
// for a DecisionNone the local fill produced); otherwise a
// DecisionListCapabilities when capabilitiesParam was found or
// capabilitiesSeparate is true (usecase.Decision{Kind:
// usecase.DecisionListCapabilities}, Service left empty so
// resolvePickedFill's own o.listCapabilities call lists every service, not
// one - the same bare shape planStaged's own PickListCapabilities case
// already builds); otherwise a DecisionAsk over mismatchParam (Question
// built the same way usecase.askForEnumGuess's own does, "<title>はどれです
// か？", so Orchestrator.ask produces the identical ResultKindAsk shape
// either way) when one was found, otherwise a DecisionCall naming every
// answered parameter's own value - nil Args, not an empty map, when every
// parameter answered unsetChoice, the same "absent, not empty" convention
// usecase.argsFromAnswers already keeps.
func (o *fillOutcome) decision(endpoint *domain.Endpoint) usecase.Decision {
	if o.refusalParam != "" || o.refusalSeparate {
		return usecase.Decision{Kind: usecase.DecisionNone}
	}

	if o.capabilitiesParam != "" || o.capabilitiesSeparate {
		return usecase.Decision{Kind: usecase.DecisionListCapabilities}
	}

	if o.mismatchParam != "" {
		return usecase.Decision{
			Kind:        usecase.DecisionAsk,
			Service:     endpoint.Service,
			OperationID: endpoint.OperationID,
			Param:       o.mismatchParam,
			Question:    questionTitleFor(endpoint, o.mismatchParam) + askQuestionText,
		}
	}

	return usecase.Decision{Kind: usecase.DecisionCall, Service: endpoint.Service, OperationID: endpoint.OperationID, Args: o.args}
}

// logFillCompleted logs fill_provider "jev", one chosen option and
// confidence per parameter, fill_ms, the token usage Jev's own response
// reports, and the full probability distribution per question - the same
// convention jev.Picker.Pick already logs pick_probabilities under. In
// refusalModeSeparate, it additionally logs fill_refusal_noul and
// fill_capabilities_noul - the two whole-request judgements' own
// probabilities, beside the per-parameter distributions already logged, so a
// run's platform log alone is enough to re-analyse both together (the
// measurement this whole option exists for: docs/measurements' own captured
// distributions were read from exactly this kind of log line).
func logFillCompleted(
	ctx context.Context, endpoint *domain.Endpoint, outcome *fillOutcome, fillMs int64, usage wireUsage, refusalMode string,
) {
	attrs := []any{
		slog.String("fill_provider", "jev"),
		slog.String("service", endpoint.Service),
		slog.String("operation_id", endpoint.OperationID),
		slog.Any("fill_choices", outcome.chosen),
		slog.Any("fill_confidences", outcome.confidence),
		slog.Int64("fill_ms", fillMs),
		slog.Int("fill_input_tokens", usage.InputTokens),
		slog.Int("fill_output_tokens", usage.OutputTokens),
		slog.Any("fill_probabilities", outcome.probabilities),
	}

	if refusalMode == refusalModeSeparate {
		attrs = append(attrs,
			slog.Float64("fill_refusal_noul", outcome.refusalProbability),
			slog.Float64("fill_capabilities_noul", outcome.capabilitiesProbability))
	}

	slog.Default().InfoContext(ctx, "fill completed", attrs...)
}
