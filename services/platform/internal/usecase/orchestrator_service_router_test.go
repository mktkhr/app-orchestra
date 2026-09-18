package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// fakeServiceRouter is a test double for usecase.ServiceRouter: it always
// returns the fixed route (or error) it was built with, ignoring the
// catalogue it is given, and records the query/catalog it was called
// with - the same shape fakeNarrower (orchestrator_narrowing_test.go) and
// fakeGate (orchestrator_staging_test.go) give their own ports.
type fakeServiceRouter struct {
	route usecase.ServiceRoute
	err   error

	query   string
	catalog domain.Catalog
	calls   int
}

func (f *fakeServiceRouter) Route(
	_ context.Context, query string, _ []usecase.Answer, _ []usecase.Turn, catalog domain.Catalog,
) (usecase.ServiceRoute, error) {
	f.query = query
	f.catalog = catalog
	f.calls++

	return f.route, f.err
}

// TestPlanWithNoServiceRouterOffersByteIdenticalTools is the default-off
// guarantee this subproject's own spec asks for: a nil router (never
// WithServiceRouter) leaves Plan byte for byte the same as before this
// port existed - the same shape
// TestPlanWithPassThroughNarrowerOffersByteIdenticalTools already proves
// for Narrower.
func TestPlanWithNoServiceRouterOffersByteIdenticalTools(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}
	orchestrator := usecase.NewOrchestrator(twoServiceCatalog(), planner, &fakeInvoker{}, &fakePermissionStore{})

	_, err := orchestrator.Plan(t.Context(), adminUser(), "在庫の一覧を見せて", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, usecase.ToolsFor(twoServiceCatalog(), usecase.PlanContext{}), planner.tools)
}

// TestPlanConfidentRouteNarrowsToOneServiceBeforeNarrowing proves a
// confident ServiceRoute (Service set, Confidence at or above threshold)
// narrows the catalogue to that one service before the narrower ever
// sees it - the narrower's own fakeNarrower.catalog is ignored (it always
// returns its own fixed catalogue), but fakeServiceRouter.catalog (what
// the router itself was offered) and the planner's own tools (what the
// narrower - here PassThroughNarrower - forwarded) both prove the
// narrowing already happened by the time each is called.
func TestPlanConfidentRouteNarrowsToOneServiceBeforeNarrowing(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}
	router := &fakeServiceRouter{route: usecase.ServiceRoute{Service: "attendance", Confidence: 0.9}}

	orchestrator := usecase.NewOrchestrator(
		twoServiceCatalog(), planner, &fakeInvoker{}, &fakePermissionStore{}, usecase.WithServiceRouter(router, 0.5),
	)

	_, err := orchestrator.Plan(t.Context(), adminUser(), "勤怠を見せて", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, 1, router.calls)

	wantCatalog := domain.Catalog{Endpoints: []domain.Endpoint{twoServiceCatalog().Endpoints[2]}}
	assert.Equal(t, usecase.ToolsFor(wantCatalog, usecase.PlanContext{}), planner.tools,
		"the planner must be offered only attendance's own endpoint, narrowed before it ever reached the planner")
}

// TestPlanRouteFailsOpenLeavesCatalogueUntouched proves every failure
// path routeService has falls open the same way: a Service named below
// o.serviceRouterThreshold, a Service the catalogue does not actually
// carry (the same treatment an unrecognised choice anywhere else in this
// codebase gets), the router's own "no opinion" (Service == "", the
// catch-all - jev.ServiceRouter's own mapRouteAnswer), and a
// transport-level router error (never surfaced to the caller as a Plan
// error, the same "consulted, never required" treatment planStaged's own
// o.gate already gets) all leave the catalogue exactly as it would reach
// the planner with no router configured at all.
func TestPlanRouteFailsOpenLeavesCatalogueUntouched(t *testing.T) {
	tests := map[string]*fakeServiceRouter{
		"below threshold":        {route: usecase.ServiceRoute{Service: "attendance", Confidence: 0.49}},
		"unknown service":        {route: usecase.ServiceRoute{Service: "payroll", Confidence: 0.99}},
		"catch-all (no opinion)": {route: usecase.ServiceRoute{}},
		"transport error":        {err: errors.New("jev unreachable")},
	}

	for name, router := range tests {
		t.Run(name, func(t *testing.T) {
			planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}

			orchestrator := usecase.NewOrchestrator(
				twoServiceCatalog(), planner, &fakeInvoker{}, &fakePermissionStore{}, usecase.WithServiceRouter(router, 0.5),
			)

			_, err := orchestrator.Plan(t.Context(), adminUser(), "在庫を見せて", nil, nil, "", "", nil)

			require.NoError(t, err)
			assert.Equal(t, usecase.ToolsFor(twoServiceCatalog(), usecase.PlanContext{}), planner.tools)
		})
	}
}

// TestPlanRouteNarrowsAheadOfNarrowerNotJustAheadOfThePick proves the
// router's own narrowing is what the narrower is offered - the narrower
// itself records the catalogue it is given, so this is a direct
// assertion on the argument order Plan calls the two collaborators in
// (routeService before o.narrower.Narrow), not merely an inference from
// the planner's own final tools.
func TestPlanRouteNarrowsAheadOfNarrowerNotJustAheadOfThePick(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}
	router := &fakeServiceRouter{route: usecase.ServiceRoute{Service: "attendance", Confidence: 0.9}}
	narrower := &fakeNarrower{catalog: twoServiceCatalog()}

	orchestrator := usecase.NewOrchestrator(
		twoServiceCatalog(), planner, &fakeInvoker{}, &fakePermissionStore{},
		usecase.WithServiceRouter(router, 0.5), usecase.WithNarrower(narrower, 5),
	)

	_, err := orchestrator.Plan(t.Context(), adminUser(), "勤怠を見せて", nil, nil, "", "", nil)

	require.NoError(t, err)

	wantCatalog := domain.Catalog{Endpoints: []domain.Endpoint{twoServiceCatalog().Endpoints[2]}}
	assert.Equal(t, wantCatalog, narrower.calledWith,
		"the narrower must be offered the catalogue already narrowed to the routed service")
}

// TestPlanWithPreferredNeverCallsTheServiceRouter proves preferred still
// bypasses routing entirely, the same way it already bypasses the
// narrower (Plan's own doc comment): a request naming one operation has
// nothing left for the router to decide.
func TestPlanWithPreferredNeverCallsTheServiceRouter(t *testing.T) {
	catalog := getAttendanceRecordCatalog()
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}
	router := &fakeServiceRouter{route: usecase.ServiceRoute{Service: "inventory", Confidence: 0.9}}

	orchestrator := usecase.NewOrchestrator(
		catalog, planner, &fakeInvoker{}, &fakePermissionStore{}, usecase.WithServiceRouter(router, 0.5),
	)

	_, err := orchestrator.Plan(t.Context(), adminUser(), "勤怠を見せて", nil, nil, "", "GetAttendanceRecord", nil)

	require.NoError(t, err)
	assert.Equal(t, 0, router.calls)
}
