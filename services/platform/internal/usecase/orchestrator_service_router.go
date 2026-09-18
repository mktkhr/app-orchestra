// orchestrator_service_router.go: routeService and catalogHasService,
// split out of orchestrator.go for guard-filelen (harness/quality/filelen.sh's
// 1000-line cap - orchestrator.go was already at its own cap before this
// subproject existed), the same split orchestrator_affinity.go and
// orchestrator_staging.go already describe for idAffinity/narrowToService
// and planStaged respectively. The package's own doc comment is
// orchestrator.go's.

package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// routeService is Plan's own call into o.serviceRouter, run before
// o.narrower.Narrow so the local embedding/reranker narrowing (and,
// under WithStages(2), the pick) both run inside whichever service the
// router names, rather than against the whole catalogue
// (docs/measurements/jev-full-catalogue.md; DECISIONS.md 2026-09-18).
// o.serviceRouter is nil by default (WithServiceRouter never given), in
// which case this returns catalog unchanged without ever calling
// anything - byte for byte the same Plan path as before this stage
// existed.
//
// Every failure path here falls open, by design: a router error is
// logged at warn and swallowed; an empty Service (the router's own "no
// opinion", including its catch-all answer), a Service the catalogue
// does not actually have, or a Confidence below
// o.serviceRouterThreshold are all logged at most at info and leave
// catalog exactly as it was given. This stage can only narrow the
// catalogue to one service - unlike Gate, it can never refuse a question
// on its own.
func (o *Orchestrator) routeService(
	ctx context.Context, catalog domain.Catalog, query string, answers []Answer, turns []Turn,
) domain.Catalog {
	if o.serviceRouter == nil {
		return catalog
	}

	start := time.Now()

	route, err := o.serviceRouter.Route(ctx, query, answers, truncateTurns(turns, o.contextWindow), catalog)
	if err != nil {
		slog.Default().WarnContext(ctx, "service router failed, proceeding without routing", slog.Any("error", err))

		return catalog
	}

	if route.Service == "" || route.Confidence < o.serviceRouterThreshold || !catalogHasService(catalog, route.Service) {
		return catalog
	}

	narrowed := narrowToService(catalog, route.Service)

	slog.Default().InfoContext(ctx, "service router narrowed catalogue to one service",
		slog.String("route_service", route.Service),
		slog.Float64("route_confidence", route.Confidence),
		slog.Int64("route_ms", time.Since(start).Milliseconds()))

	return narrowed
}

// catalogHasService reports whether catalog carries at least one
// endpoint belonging to service - routeService's own guard against a
// router naming a service the catalogue does not have (an unknown
// service, or one permissions/preferred has already filtered out),
// which must fail open exactly as an unrecognised choice does anywhere
// else in this codebase.
func catalogHasService(catalog domain.Catalog, service string) bool {
	for i := range catalog.Endpoints {
		if catalog.Endpoints[i].Service == service {
			return true
		}
	}

	return false
}
