package usecase

import (
	"context"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// ServiceRoute is what a ServiceRouter returns for one question: which
// service, of the ones catalog carries, the router judges the question
// belongs to, and its own confidence in that judgment. Service is "" for
// "no opinion" - the router named the catch-all, an unknown service, or
// otherwise has nothing useful to say - which Plan treats exactly like a
// router error or a below-threshold confidence: fail open, catalogue
// left untouched (see Plan's own doc comment).
type ServiceRoute struct {
	Service    string
	Confidence float64
}

// ServiceRouter names, before narrowing, which single service of the
// catalogue a question belongs to - a service-level analogue of Gate,
// asked for the opposite reason: not to refuse a question, only to
// narrow where the rest of the pipeline (o.narrower, then o.picker or
// the ordinary planner) goes looking for the answer.
//
// This exists because giving Jev the whole catalogue at once picks the
// wrong *operation* more often than the local picker does, but names the
// right *service* about as reliably as anything in this codebase
// measures (docs/measurements/jev-full-catalogue.md; DECISIONS.md,
// 2026-09-18): 90 of 98 rows right on service, 25 of 25 on the
// cross-service homonym axis, against 70 correct@1 on the operation
// itself. So Jev is asked only the coarser, better-answered question,
// and the existing local narrowing/pick run inside whichever service it
// names.
//
// Orchestrator.Plan calls Route before o.narrower.Narrow, and only when
// the request carries no preferred (see Plan's own doc comment) - a
// router error, an unknown service, a catch-all answer or a confidence
// below the configured threshold must all fall through to today's
// behaviour: this stage can only narrow the catalogue, never refuse a
// question the way Gate can.
type ServiceRouter interface {
	Route(
		ctx context.Context, query string, answers []Answer, turns []Turn, catalog domain.Catalog,
	) (ServiceRoute, error)
}
