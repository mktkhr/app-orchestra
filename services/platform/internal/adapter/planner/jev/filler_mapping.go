package jev

import (
	"context"
	"log/slog"
	"slices"
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
func buildFillRequest(
	endpoint *domain.Endpoint, query string, answers []usecase.Answer, turns []usecase.Turn, now time.Time,
) wireRequest {
	questions := make(map[string]wireQuestion, len(endpoint.Parameters))

	for i := range endpoint.Parameters {
		p := &endpoint.Parameters[i]
		questions[p.Name] = wireQuestion{
			Type:         choiceQuestionType,
			Instructions: instructionsForParam(p),
			Criteria:     criteriaForParam(p),
		}
	}

	return wireRequest{
		State:     toolcall.BuildUserContent(query, answers, turns, now),
		Model:     modelName,
		Questions: questions,
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
func criteriaForParam(p *domain.Parameter) map[string]string {
	criteria := make(map[string]string, len(p.Schema.Enum)+sentinelCount)

	for _, value := range p.Schema.Enum {
		label := p.Schema.EnumLabels[value]
		if label == "" {
			label = value
		}

		criteria[value] = label
	}

	criteria[unsetChoice] = unsetLabel
	criteria[mismatchChoice] = mismatchLabel

	return criteria
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
// Filler.Fill returns when mismatchParam is "" (every parameter answered
// unsetChoice or a real enum value); mismatchParam, when non-empty, is the
// first parameter (in endpoint.Parameters order) whose answer was
// mismatchChoice - decision (below) turns either shape into the
// usecase.Decision Fill returns. chosen, confidence and probabilities
// carry every parameter's own raw answer, read only by logFillCompleted.
type fillOutcome struct {
	args          map[string]any
	mismatchParam string
	chosen        map[string]string
	confidence    map[string]float64
	probabilities map[string]map[string]float64
}

// mapFillAnswers reads resp.Answers against endpoint's own parameters:
// false whenever any parameter's answer is missing entirely, or its
// confidence is below threshold - Fill's own fail-open cases - or its
// choice is neither a sentinel nor one of the parameter's own declared
// enum values (a shape Jev's own criteria never offered, so this is
// defensive, not an expected path). True otherwise, with outcome fully
// built: every parameter is read once, in endpoint.Parameters order, so a
// deployment with more than one mismatchChoice answer always names the
// same one first.
func mapFillAnswers(endpoint *domain.Endpoint, resp wireResponse, threshold float64) (fillOutcome, bool) {
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

	if len(outcome.args) == 0 {
		outcome.args = nil
	}

	return outcome, true
}

// decision turns outcome into the usecase.Decision Filler.Fill returns:
// a DecisionAsk over mismatchParam (Question built the same way
// usecase.askForEnumGuess's own does, "<title>はどれですか？", so
// Orchestrator.ask produces the identical ResultKindAsk shape either way)
// when one was found, otherwise a DecisionCall naming every answered
// parameter's own value - nil Args, not an empty map, when every
// parameter answered unsetChoice, the same "absent, not empty" convention
// usecase.argsFromAnswers already keeps.
func (o fillOutcome) decision(endpoint *domain.Endpoint) usecase.Decision {
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
// convention jev.Picker.Pick already logs pick_probabilities under.
func logFillCompleted(ctx context.Context, endpoint *domain.Endpoint, outcome fillOutcome, fillMs int64, usage wireUsage) {
	slog.Default().InfoContext(ctx, "fill completed",
		slog.String("fill_provider", "jev"),
		slog.String("service", endpoint.Service),
		slog.String("operation_id", endpoint.OperationID),
		slog.Any("fill_choices", outcome.chosen),
		slog.Any("fill_confidences", outcome.confidence),
		slog.Int64("fill_ms", fillMs),
		slog.Int("fill_input_tokens", usage.InputTokens),
		slog.Int("fill_output_tokens", usage.OutputTokens),
		slog.Any("fill_probabilities", outcome.probabilities))
}
