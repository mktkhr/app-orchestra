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

// fakeNarrower is a test double for usecase.Narrower: it always returns
// the fixed catalogue (or error) it was built with, ignoring the one it
// is given, and records the query and k it was called with.
type fakeNarrower struct {
	catalog domain.Catalog
	err     error

	query string
	k     int
	// calledWith is the catalogue Narrow was actually given - distinct
	// from catalog above (what it returns) - so a test can assert on
	// what Plan offered the narrower, not just what the narrower handed
	// back (orchestrator_service_router_test.go's own
	// TestPlanRouteNarrowsAheadOfNarrowerNotJustAheadOfThePick).
	calledWith domain.Catalog
}

func (f *fakeNarrower) Narrow(_ context.Context, catalog domain.Catalog, query string, k int) (domain.Catalog, error) {
	f.query = query
	f.k = k
	f.calledWith = catalog

	return f.catalog, f.err
}

// shortlistCatalog is three endpoints in a deliberately non-alphabetical
// order, standing in for a reranker's shortlist (docs/specs/shortlisting.md,
// H1): a test asserting order must not accidentally pass because the order
// happened to match some other sort.
func shortlistCatalog() domain.Catalog {
	endpoint := func(service, operationID string) domain.Endpoint {
		return domain.Endpoint{
			Service:     service,
			OperationID: operationID,
			Method:      domain.MethodGet,
			Path:        "/" + operationID,
			Summary:     operationID,
			Response:    &domain.Schema{Type: domain.SchemaTypeObject},
		}
	}

	return domain.Catalog{Endpoints: []domain.Endpoint{
		endpoint("b", "Bravo"),
		endpoint("a", "Alpha"),
		endpoint("c", "Charlie"),
	}}
}

// TestPlanOffersExactlyTheNarrowedCatalogueInItsOwnOrder is AC-H-102: with
// a Narrower configured, the planner is offered exactly the endpoints it
// returned, plus the built-ins, in the order the Narrower gave them - not
// resorted by ToolsFor, which iterates a catalogue's endpoints in the
// order it holds them (internal/usecase/tools.go).
func TestPlanOffersExactlyTheNarrowedCatalogueInItsOwnOrder(t *testing.T) {
	shortlist := shortlistCatalog()
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}
	narrower := &fakeNarrower{catalog: shortlist}

	orchestrator := usecase.NewOrchestrator(
		inventoryCatalog(), planner, &fakeInvoker{}, &fakePermissionStore{}, usecase.WithNarrower(narrower, 20),
	)

	_, err := orchestrator.Plan(t.Context(), adminUser(), "在庫の一覧を見せて", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, usecase.ToolsFor(shortlist, usecase.PlanContext{}), planner.tools,
		"the planner must be offered exactly the narrowed catalogue's tools, in the narrower's own order, plus the built-ins")
	assert.Equal(t, "在庫の一覧を見せて", narrower.query, "Narrow must be called with the question")
	assert.Equal(t, 20, narrower.k, "Narrow must be called with the configured K")
}

// TestPlanWithPassThroughNarrowerOffersByteIdenticalTools is AC-H-101: with
// no Narrower configured - every existing caller of NewOrchestrator, and
// every test above this one - Plan offers exactly what ToolsFor(catalog,
// planCtx) alone built before this port existed. Comparing the two slices
// directly (not a wire re-encoding) already proves same content and same
// order, the same reasoning tools_test.go's own
// TestToolsForCatalogueToolsAreUnaffectedByPlanContext gives for why a
// plain slice comparison is enough at this layer, which may not import
// encoding/json at all (depguard, harness/quality/go/golangci.yml).
func TestPlanWithPassThroughNarrowerOffersByteIdenticalTools(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}
	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, &fakeInvoker{}, &fakePermissionStore{})

	_, err := orchestrator.Plan(t.Context(), adminUser(), "在庫の一覧を見せて", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, usecase.ToolsFor(inventoryCatalog(), usecase.PlanContext{}), planner.tools)
}

// TestPassThroughNarrowerReturnsCatalogUnchanged exercises
// usecase.PassThroughNarrower directly, beyond what driving it through
// Plan already proves.
func TestPassThroughNarrowerReturnsCatalogUnchanged(t *testing.T) {
	catalog := inventoryCatalog()

	got, err := (usecase.PassThroughNarrower{}).Narrow(t.Context(), catalog, "在庫の一覧を見せて", 5)

	require.NoError(t, err)
	assert.Equal(t, catalog, got)
}

// TestPlanWrapsANarrowerError proves a Narrower's error reaches the
// caller, wrapped, rather than being swallowed - the same treatment
// TestPlanWrapsAPlannerError and TestPlanWrapsAnInvokerError give their
// own collaborators' errors.
func TestPlanWrapsANarrowerError(t *testing.T) {
	boom := errors.New("embedding endpoint unreachable")
	narrower := &fakeNarrower{err: boom}
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}

	orchestrator := usecase.NewOrchestrator(
		inventoryCatalog(), planner, &fakeInvoker{}, &fakePermissionStore{}, usecase.WithNarrower(narrower, 20),
	)

	_, err := orchestrator.Plan(t.Context(), adminUser(), "在庫の一覧を見せて", nil, nil, "", "", nil)

	require.Error(t, err)
	require.ErrorIs(t, err, boom)
}
