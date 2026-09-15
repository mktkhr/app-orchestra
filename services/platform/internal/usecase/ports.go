// Package usecase holds the platform's ports and orchestration logic: the
// interfaces the adapters implement, and (in later tasks) the code that
// drives them. Nothing here performs I/O directly; it depends on domain and
// nothing else.
package usecase

import (
	"context"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// SpecSource fetches the OpenAPI contract of every configured service and
// converts it into the platform's catalogue. Implemented by
// internal/adapter/specsource/http, which reads each service's
// /openapi.yaml over HTTP; kept as a port here so the usecase layer depends
// on the shape of the fetch, not on HTTP.
//
// A service that cannot be reached must fail the whole fetch: a partial
// catalogue would silently hide endpoints that exist, which is worse than
// failing loudly at startup.
type SpecSource interface {
	Fetch(ctx context.Context) (domain.Catalog, error)
}

// Narrower cuts a catalogue down to a shortlist before Orchestrator.Plan
// offers it to the planner (docs/specs/shortlisting.md, H1/H2): the
// planner receives a shorter list and never knows why. Implemented by
// internal/adapter/narrowing/llamaswap, which embeds and reranks over
// llama-swap's HTTP endpoints - kept as a port here so the usecase layer
// depends on the shape of the cut, not on a vector or an HTTP call.
type Narrower interface {
	// Narrow returns at most k endpoints of catalog, best first, for
	// query. A Narrower may return catalog unchanged; the orchestrator
	// does not care.
	Narrow(ctx context.Context, catalog domain.Catalog, query string, k int) (domain.Catalog, error)
}

// PassThroughNarrower implements Narrower by returning catalog unchanged,
// ignoring query and k entirely. It is what NewOrchestrator defaults to
// (docs/specs/shortlisting.md, H7): with no narrowing configured, the
// platform behaves exactly as it did before this port existed, and every
// test that predates it keeps exercising this same Narrower without
// knowing it exists.
type PassThroughNarrower struct{}

var _ Narrower = PassThroughNarrower{}

// Narrow returns catalog unchanged.
func (PassThroughNarrower) Narrow(_ context.Context, catalog domain.Catalog, _ string, _ int) (domain.Catalog, error) {
	return catalog, nil
}
