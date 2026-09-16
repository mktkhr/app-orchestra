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
//
// thinking is Plan's own thinking parameter, carried to o.planner.Plan
// unchanged - the same question, so the same per-request thinking value it
// was asked with, whether it resolves through this preferred path or the
// ordinary one.
//
// fromPick is the staging subproject's own addition (docs/specs/staging.md,
// S1/S5): false for a chip the person has already chosen (Plan's own
// preferred request field), which keeps every behaviour above unchanged;
// true when this fill follows a pick (Orchestrator.planStaged,
// orchestrator_staging.go). Only when true does this function offer
// AskUserTool() and ListCapabilitiesTool() alongside the one operation's
// tool (never propose_panel - section 3's own single-call fallback already
// covers it), and only then is the model's answer honoured rather than
// degraded to a form: a DecisionAsk naming this same operation resolves
// through o.ask, DecisionNone and DecisionListCapabilities resolve exactly
// as planOrdinary resolves them. A pick has not already let the person
// choose between candidates the way a chip has, so the fill must still be
// able to end in a question, a "nothing here" refusal, or a capabilities
// table - not only in a call or a form. A call naming some other operation
// is the one case that still degrades to formFor: the pick, not the fill,
// is where the operation gets chosen, so a fill that reaches for a
// different one has not disagreed with the pick, it has misfired, and the
// form for the endpoint the person actually picked is still the safer
// answer than acting on an operation they never chose (dev-stack finding,
// docs/specs/staging.md section 5).
//
// catalog is the caller's already-scoped catalogue - the shortlist under a
// pick, the permission-filtered whole catalogue under a chip's preferred -
// used only for a fromPick DecisionListCapabilities (o.listCapabilities
// reads the whole scope, never the one endpoint alone); ignored otherwise,
// so passing it under fromPick == false changes nothing observable.
func (o *Orchestrator) planPreferred(
	ctx context.Context, catalog domain.Catalog, endpoint *domain.Endpoint, query string, answers []Answer,
	turns []Turn, thinking *bool, fromPick bool,
) (Result, error) {
	fallback := &Decision{Service: endpoint.Service, OperationID: endpoint.OperationID, Args: argsFromAnswers(answers)}

	// A required parameter the question (and any answers already given)
	// cannot supply is exactly what a form is for (D8 already renders one
	// for an unsafe operation; a safe operation missing an id it was never
	// given is the same situation) - so the model is never even asked.
	//
	// Only when fromPick is false: a chip's preferred carries no fresh
	// text (the person chose an operation and typed nothing new, 0840502),
	// so the model has nothing to extract and the shortcut is exact. A
	// pick's preferred comes from the question itself - 「在庫を登録して。
	// 名前はテスト品、数量は5、引当済で」 names CreateInventoryItem and
	// carries its arguments in the same sentence (regression, ORCHESTRA_
	// PLANNER_STAGES=2 make eval, create/create-attendance) - so under a
	// pick the model is always consulted; the fallback below still catches
	// anything it cannot use.
	if !fromPick && !requiredParamsKnown(endpoint, answers) {
		return formFor(endpoint, fallback, query, answers), nil
	}

	tools := []Tool{toolFor(endpoint)}
	if fromPick {
		tools = append(tools, AskUserTool(), ListCapabilitiesTool())
	}

	decision, err := o.planner.Plan(ctx, query, answers, truncateTurns(turns, o.contextWindow), tools, thinking)
	if err != nil {
		return Result{}, fmt.Errorf("planning: %w", err)
	}

	sameOperation := decision.Service == endpoint.Service && decision.OperationID == endpoint.OperationID

	if fromPick {
		return o.resolvePickedFill(ctx, catalog, endpoint, &decision, fallback, sameOperation, query, answers)
	}

	// fromPick == false (a chip's own preferred): unchanged from before this
	// subproject. Anything other than a call to the one tool offered -
	// list_capabilities, ask_user, none, or a call naming some other
	// operation entirely - is discarded in favour of the same form: the
	// person already chose this operation, so whatever the model said
	// instead is not the answer to show them (see the reproduction above).
	if decision.Kind != DecisionCall || !sameOperation {
		return formFor(endpoint, fallback, query, answers), nil
	}

	return o.call(ctx, domain.Catalog{Endpoints: []domain.Endpoint{*endpoint}}, &decision, query, answers)
}

// resolvePickedFill is planPreferred's fromPick == true dispatch, split out
// to keep planPreferred itself under the harness's cyclomatic-complexity
// guard (gocyclo, harness/quality/go/golangci.yml). See planPreferred's own
// doc comment for what each DecisionKind resolves to and why; decision is a
// pointer for the same gocritic hugeParam reason as planPreferred's own
// endpoint parameter.
func (o *Orchestrator) resolvePickedFill(
	ctx context.Context, catalog domain.Catalog, endpoint *domain.Endpoint, decision *Decision, fallback *Decision,
	sameOperation bool, query string, answers []Answer,
) (Result, error) {
	switch decision.Kind {
	case DecisionAsk:
		if sameOperation {
			return o.ask(domain.Catalog{Endpoints: []domain.Endpoint{*endpoint}}, decision, query, answers)
		}
	case DecisionNone:
		return Result{Kind: ResultKindNone, Message: messageNoEndpoint}, nil
	case DecisionListCapabilities:
		return o.listCapabilities(catalog, decision), nil
	case DecisionCall:
		if sameOperation {
			return o.call(ctx, domain.Catalog{Endpoints: []domain.Endpoint{*endpoint}}, decision, query, answers)
		}
	case DecisionProposal:
		// Not offered above (ProposePanelToolName is never in tools built by
		// planPreferred), so the model cannot legitimately return this -
		// falls through to the same formFor as any other unusable answer.
	}

	return formFor(endpoint, fallback, query, answers), nil
}

// requiredParamsKnown reports whether endpoint has no required parameter at
// all, or every one it does have is already named in answers - the same
// "required" list inputSchemaFor builds for the tool's own schema and for
// formFor's Schema (read via requiredParamNames, tools.go, so it can never
// drift from either, or from askDegrade's own use of the same list).
func requiredParamsKnown(endpoint *domain.Endpoint, answers []Answer) bool {
	required := requiredParamNames(endpoint)
	if len(required) == 0 {
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
