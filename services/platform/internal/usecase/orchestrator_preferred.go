package usecase

import (
	"context"
	"fmt"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// planPreferred resolves PlanRequest.preferred (docs/specs/shortlisting.md,
// section 4) into a Result: unlike the ordinary path, the model is never
// offered list_capabilities, ask_user or propose_panel alongside the named
// operation, and for an operation with a required parameter the model is
// not consulted at all. Split out of orchestrator.go to keep that file
// under the harness's file-length guard.
//
// Reproduction (2026-09-15): asking 「勤怠記録の詳細」 (GetAttendanceRecord,
// which requires id) from a chat that only ever said 「勤怠記録を見せて」
// posted preferred: "GetAttendanceRecord". The old code narrowed the
// catalogue to that one operation but still handed the planner that tool
// plus the built-ins (Plan's old comment: "the planner is offered that tool
// alone plus the built-ins"); GetAttendanceRecord needs an id the question
// never gave, so the model could not call it and reached for
// list_capabilities instead - which was still on the list - and the person
// who chose "the detail of this record" got a capabilities table back.
// Under preferred the answer IS the named operation, so list_capabilities
// (and every other built-in) has no business being offered beside it: a
// person who cannot supply id gets a form to fill it in, never a table
// about the operation they already chose.
//
// endpoint is a pointer for the same gocritic hugeParam reason as call's
// and ask's decision parameter (domain.Endpoint is 136 bytes; see
// harness/quality/go/golangci.yml).
func (o *Orchestrator) planPreferred(
	ctx context.Context, endpoint *domain.Endpoint, query string, answers []Answer, turns []Turn,
) (Result, error) {
	fallback := &Decision{Service: endpoint.Service, OperationID: endpoint.OperationID, Args: argsFromAnswers(answers)}

	// A required parameter the question (and any answers already given)
	// cannot supply is exactly what a form is for (D8 already renders one
	// for an unsafe operation; a safe operation missing an id it was never
	// given is the same situation) - so the model is never even asked.
	if !requiredParamsKnown(endpoint, answers) {
		return formFor(endpoint, fallback), nil
	}

	decision, err := o.planner.Plan(ctx, query, answers, truncateTurns(turns, o.contextWindow), []Tool{toolFor(endpoint)})
	if err != nil {
		return Result{}, fmt.Errorf("planning: %w", err)
	}

	// Anything other than a call to the one tool offered - list_capabilities,
	// ask_user, none, or a call naming some other operation entirely - is
	// discarded in favour of the same form: the person already chose this
	// operation, so whatever the model said instead is not the answer to
	// show them (see the reproduction above).
	if decision.Kind != DecisionCall || decision.Service != endpoint.Service || decision.OperationID != endpoint.OperationID {
		return formFor(endpoint, fallback), nil
	}

	return o.call(ctx, domain.Catalog{Endpoints: []domain.Endpoint{*endpoint}}, &decision)
}

// requiredParamsKnown reports whether endpoint has no required parameter at
// all, or every one it does have is already named in answers - the same
// "required" list inputSchemaFor builds for the tool's own schema and for
// formFor's Schema, read once here rather than walked a second time, so
// what counts as required can never drift between the three.
func requiredParamsKnown(endpoint *domain.Endpoint, answers []Answer) bool {
	schema := inputSchemaFor(endpoint)

	requiredAny, ok := schema[keyRequired]
	if !ok {
		return true
	}

	required, ok := requiredAny.([]string)
	if !ok || len(required) == 0 {
		return true
	}

	known := make(map[string]bool, len(answers))
	for _, a := range answers {
		known[a.Param] = true
	}

	for _, name := range required {
		if !known[name] {
			return false
		}
	}

	return true
}

// argsFromAnswers turns answers into the map planPreferred's fallback form
// carries as Initial: whatever the person already answered (a previous
// ask_user's reply, most often) pre-fills the form instead of being asked
// for a second time. nil, not an empty map, when there is nothing to carry
// - the same "absent, not empty" convention Result's own doc comments use
// elsewhere.
func argsFromAnswers(answers []Answer) map[string]any {
	if len(answers) == 0 {
		return nil
	}

	args := make(map[string]any, len(answers))
	for _, a := range answers {
		args[a.Param] = a.Value
	}

	return args
}
