package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// planOrdinary is today's single-call path (AC-S-101, byte for byte with
// STAGES unset or 1): the whole (narrowed) catalogue's tools, one planner
// call, and the switch over every DecisionKind the platform acts on. Kept
// here, in orchestrator_staging.go, rather than orchestrator.go
// (harness/quality/filelen.sh's 1000-line guard - orchestrator.go was
// already at 942 before this subproject), even though Plan (orchestrator.go)
// is its only other caller besides planStaged's own propose_panel
// fallback below.
func (o *Orchestrator) planOrdinary(
	ctx context.Context, catalog domain.Catalog, query string, answers []Answer, turns []Turn, workspaceID string,
	thinking *bool,
) (Result, error) {
	planCtx := PlanContext{WorkspaceID: workspaceID}
	tools := ToolsFor(catalog, planCtx)

	decision, err := o.planner.Plan(ctx, query, answers, truncateTurns(turns, o.contextWindow), tools, thinking)
	if err != nil {
		return Result{}, fmt.Errorf("planning: %w", err)
	}

	switch decision.Kind {
	case DecisionNone:
		return Result{Kind: ResultKindNone, Message: messageNoEndpoint}, nil
	case DecisionCall:
		return o.call(ctx, catalog, &decision)
	case DecisionAsk:
		return o.ask(catalog, &decision)
	case DecisionListCapabilities:
		return o.listCapabilities(catalog, &decision), nil
	case DecisionProposal:
		if !toolOffered(tools, ProposePanelToolName) {
			return Result{}, fmt.Errorf("%w: %s", ErrToolNotOffered, ProposePanelToolName)
		}

		return o.propose(catalog, &decision)
	default:
		return Result{}, fmt.Errorf("%w: unknown decision kind %q", ErrNotImplemented, decision.Kind)
	}
}

// staged is the value WithStages must be given for Plan to take the
// pick-then-fill path below, named so the comparison in orchestrator.go's
// Plan is not a bare magic number (mnd, harness/quality/go/golangci.yml).
const staged = 2

// planStaged resolves a question under WithStages(2): a pick over the
// narrowed shortlist (o.picker.Pick), then a fill against only the
// operation it named - the path docs/specs/staging.md's S1 describes.
// catalog is the shortlist Plan already narrowed before calling here (H1/
// H2, orchestrator.go) - the same value planOrdinary would have offered
// the planner whole, had staging been off, and what planPicked's own
// alternativesFor reads (H5).
//
// The pick's own outcome is logged at info - pick_operation_id,
// pick_ambiguous and pick_ms (S4: recorded, not acted on; snake_case keys
// per sloglint, part of the fixed harness policy,
// harness/quality/go/golangci.yml - the plan's own prose used
// "pick.operationId", which that guard rejects) - one line per request,
// through the same slog.Default() every other planner-adjacent log line in
// this codebase (toolcall.Planner's truncation warn) uses, rather than a
// logger field threaded onto Orchestrator: nothing else here needs one,
// and slog.Default() is what pkg/app.build already configures
// process-wide before an Orchestrator is ever built.
func (o *Orchestrator) planStaged(
	ctx context.Context, catalog domain.Catalog, query string, answers []Answer, turns []Turn, workspaceID string,
	thinking *bool,
) (Result, error) {
	start := time.Now()

	p, err := o.picker.Pick(ctx, query, answers, catalog)
	if err != nil {
		return Result{}, fmt.Errorf("picking: %w", err)
	}

	slog.Default().InfoContext(ctx, "pick completed",
		slog.String("pick_operation_id", p.OperationID),
		slog.Bool("pick_ambiguous", p.Ambiguous),
		slog.Int64("pick_ms", time.Since(start).Milliseconds()))

	switch p.Kind {
	case PickListCapabilities:
		return o.listCapabilities(catalog, &Decision{Kind: DecisionListCapabilities}), nil
	case PickNone:
		return Result{Kind: ResultKindNone, Message: messageNoEndpoint}, nil
	case PickProposePanel:
		// The one built-in that needs an operation and arguments of its
		// own, and rare enough (one make eval case) that a second, pick-
		// shaped format for it is not worth its own test surface
		// (docs/specs/staging.md, section 3) - so this falls back to
		// today's single call over the whole shortlist, exactly as
		// STAGES=1 would have handled it.
		return o.planOrdinary(ctx, catalog, query, answers, turns, workspaceID, thinking)
	case PickOperation:
		return o.planPicked(ctx, catalog, &p, query, answers, turns, thinking)
	default:
		return Result{}, fmt.Errorf("%w: unknown pick kind %q", ErrNotImplemented, p.Kind)
	}
}

// planPicked resolves a PickOperation: the fill (planPreferred, fromPick
// true) against the one endpoint the pick named, then - for a call result
// only - Alternatives from catalog's own shortlist positions after it
// (alternativesFor), never from the one-endpoint catalogue planPreferred's
// own call uses internally (H5, docs/specs/staging.md section 3: "pick
// then dispatches ... down the existing planPreferred path").
func (o *Orchestrator) planPicked(
	ctx context.Context, catalog domain.Catalog, p *Pick, query string, answers []Answer, turns []Turn, thinking *bool,
) (Result, error) {
	endpoint, ok := catalog.Find(p.Service, p.OperationID)
	if !ok {
		return Result{}, fmt.Errorf("%w: %s/%s", ErrEndpointNotFound, p.Service, p.OperationID)
	}

	result, err := o.planPreferred(ctx, &endpoint, query, answers, turns, thinking, true)
	if err != nil {
		return Result{}, err
	}

	if result.Kind == ResultKindResult {
		result.Alternatives = o.alternativesFor(catalog, endpoint.Service, endpoint.OperationID)
	}

	return result, nil
}

// hasEnumParameter reports whether endpoint declares any parameter with an
// enum: the fill (planPreferred) offers ask_user alongside the picked
// operation's own tool only when this holds (S5, docs/specs/staging.md,
// section 5), so a pick can still end in a question rather than a form
// when a restricting word does not match one of the enum's declared
// values (v6-unmatched-filter) - the same situation a chip-chosen
// preferred never needed to guard against, since the person had already
// disambiguated by choosing the chip.
func hasEnumParameter(e *domain.Endpoint) bool {
	for i := range e.Parameters {
		if len(e.Parameters[i].Schema.Enum) > 0 {
			return true
		}
	}

	return false
}
