package usecase

import (
	"context"
	"log/slog"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// tryFill is planPreferred's own fromPick == true hook into the fill-stage
// experiment's two arms (docs/measurements/jev-conditions.md): arm 1
// (o.fillSkipEmpty, ORCHESTRA_FILL_SKIP_EMPTY) skips the fill's model call
// outright for an EligibleForFillSkip endpoint; arm 2 (o.fillEnum,
// ORCHESTRA_FILL_ENUM=jev) replaces it with one Filler.Fill call for an
// EligibleForFillEnum endpoint. handled is false whenever neither arm
// applies, or arm 2 fails open - the caller then falls through to its own
// o.planner.Plan call exactly as if this function did not exist; handled
// is true only when this function itself already produced the Result (or
// error) to return.
//
// Both arms produce a Decision naming this same endpoint (Service/
// OperationID always equal endpoint's own) and dispatch it through
// o.resolvePickedFill exactly as o.planner.Plan's own decision would have
// - sameOperation is always true here, so every existing safe/unsafe, ask
// and form rule there applies unchanged.
func (o *Orchestrator) tryFill(
	ctx context.Context, catalog domain.Catalog, endpoint *domain.Endpoint, fallback *Decision, query string,
	answers []Answer, turns []Turn, workspaceID string,
) (Result, bool, error) {
	if o.fillSkipEmpty && EligibleForFillSkip(endpoint) {
		slog.Default().InfoContext(ctx, "fill completed",
			slog.String("fill_provider", "skipped"),
			slog.String("service", endpoint.Service),
			slog.String("operation_id", endpoint.OperationID))

		decision := &Decision{Kind: DecisionCall, Service: endpoint.Service, OperationID: endpoint.OperationID}

		result, err := o.resolvePickedFill(ctx, catalog, endpoint, decision, fallback, true, query, answers, workspaceID)

		return result, true, err
	}

	if o.fillEnum != nil && EligibleForFillEnum(endpoint) {
		planCtx := PlanContext{WorkspaceID: workspaceID}

		decision, ok, err := o.fillEnum.Fill(ctx, endpoint, query, answers, truncateTurns(turns, o.contextWindow), planCtx)
		if err != nil {
			slog.Default().WarnContext(ctx, "fill_enum failed, falling back to local fill",
				slog.String("fill_provider", "local"), slog.Any("error", err))

			return Result{}, false, nil
		}

		if !ok {
			// The adapter itself already logged why (missing answer or a
			// below-threshold confidence) - nothing further to log here,
			// only the same fall-through a transport error takes above.
			return Result{}, false, nil
		}

		result, err := o.resolvePickedFill(ctx, catalog, endpoint, &decision, fallback, true, query, answers, workspaceID)

		return result, true, err
	}

	return Result{}, false, nil
}
