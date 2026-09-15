package usecase_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// fourEndpointShortlist stands in for a reranker's shortlist of four
// distinct operations, named a/b/c/d and each in its own service, so a
// test asserting "the two after the chosen one" cannot accidentally pass
// because the alphabet and the reranker's order happen to agree
// (docs/specs/shortlisting.md, section 4, AC-H-103).
func fourEndpointShortlist() domain.Catalog {
	endpoint := func(letter string) domain.Endpoint {
		return domain.Endpoint{
			Service:     "svc-" + letter,
			OperationID: "Op" + letter,
			Method:      domain.MethodGet,
			Path:        "/" + letter,
			Summary:     "operation " + letter,
			DisplayName: "操作" + letter,
			Response:    &domain.Schema{Type: domain.SchemaTypeObject},
		}
	}

	return domain.Catalog{Endpoints: []domain.Endpoint{
		endpoint("a"), endpoint("b"), endpoint("c"), endpoint("d"),
	}}
}

// TestPlanResultAlternativesAreTheShortlistPositionsAfterTheChoice is
// AC-H-103's central claim, checked at every position of the shortlist
// [a, b, c, d]: the result's Alternatives are always the shortlist's own
// next up-to-two entries after the one the planner chose, never the
// shortlist's own top two regardless of which position was chosen, and
// none at all when nothing follows the choice.
func TestPlanResultAlternativesAreTheShortlistPositionsAfterTheChoice(t *testing.T) {
	tests := map[string]struct {
		chosen string
		want   []usecase.Alternative
	}{
		"the first entry's alternatives are the next two": {
			chosen: "a",
			want: []usecase.Alternative{
				{OperationID: "Opb", DisplayName: "操作b", Service: "svc-b"},
				{OperationID: "Opc", DisplayName: "操作c", Service: "svc-c"},
			},
		},
		"a middle entry's alternatives are the two after it, not the top two": {
			chosen: "b",
			want: []usecase.Alternative{
				{OperationID: "Opc", DisplayName: "操作c", Service: "svc-c"},
				{OperationID: "Opd", DisplayName: "操作d", Service: "svc-d"},
			},
		},
		"the last entry has nothing after it": {
			chosen: "d",
			want:   nil,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			shortlist := fourEndpointShortlist()
			planner := &fakePlanner{decision: usecase.Decision{
				Kind: usecase.DecisionCall, Service: "svc-" + tt.chosen, OperationID: "Op" + tt.chosen,
			}}
			narrower := &fakeNarrower{catalog: shortlist}

			orchestrator := usecase.NewOrchestrator(
				fourEndpointShortlist(), planner, &fakeInvoker{data: map[string]any{}}, &fakePermissionStore{},
				usecase.WithNarrower(narrower, 20),
			)

			result, err := orchestrator.Plan(t.Context(), adminUser(), "質問", nil, nil, "", "", nil)

			require.NoError(t, err)
			assert.Equal(t, tt.want, result.Alternatives)
		})
	}
}

// TestPlanResultHasNoAlternativesWithPassThroughNarrower is H7: with no
// narrower configured, a result carries no alternatives at all, even
// though the chosen operation sits inside the (permission-filtered, not
// shortlisted) catalogue with two entries after it - "position" in an
// unranked catalogue is meaningless, and the wire must stay byte-identical
// to today (see plan_test.go's wire-level counterpart).
func TestPlanResultHasNoAlternativesWithPassThroughNarrower(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind: usecase.DecisionCall, Service: "svc-a", OperationID: "Opa",
	}}

	orchestrator := usecase.NewOrchestrator(
		fourEndpointShortlist(), planner, &fakeInvoker{data: map[string]any{}}, &fakePermissionStore{},
	)

	result, err := orchestrator.Plan(t.Context(), adminUser(), "質問", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Empty(t, result.Alternatives)
}

// TestPlanWithPreferredBypassesTheNarrowerAndOffersOnlyThatOperation is
// section 4's preferred behaviour: with Preferred set, the narrower (which
// would otherwise return the shortlist below) is never consulted at all,
// and - since Opc declares no required parameter - the planner is offered
// that operation's tool alone, with none of the built-ins beside it
// (docs/specs/shortlisting.md, H5; the 2026-09-15 reproduction in
// planPreferred's own doc comment in orchestrator.go).
func TestPlanWithPreferredBypassesTheNarrowerAndOffersOnlyThatOperation(t *testing.T) {
	shortlist := fourEndpointShortlist()
	planner := &fakePlanner{decision: usecase.Decision{
		Kind: usecase.DecisionCall, Service: "svc-c", OperationID: "Opc",
	}}
	narrower := &fakeNarrower{catalog: shortlist}

	orchestrator := usecase.NewOrchestrator(
		fourEndpointShortlist(), planner, &fakeInvoker{data: map[string]any{}}, &fakePermissionStore{},
		usecase.WithNarrower(narrower, 20),
	)

	result, err := orchestrator.Plan(t.Context(), adminUser(), "質問", nil, nil, "", "Opc", nil)

	require.NoError(t, err)
	require.Len(t, planner.tools, 1, "only the preferred operation's tool must be offered, no built-ins")
	assert.Equal(t, "Opc", planner.tools[0].Name)
	assert.Empty(t, narrower.query, "Narrow must never be called when preferred is set")
	assert.Empty(t, result.Alternatives, "a preferred result carries no alternatives")
}

// TestPlanWithUnknownPreferredIsErrEndpointNotFound is section 4: a
// preferred operation id the catalogue does not have at all is the same
// error an unknown operation from the planner itself would be, and the
// planner is never even asked - narrowing to nothing is refused before
// ToolsFor or Planner.Plan ever run.
func TestPlanWithUnknownPreferredIsErrEndpointNotFound(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}

	orchestrator := usecase.NewOrchestrator(
		fourEndpointShortlist(), planner, &fakeInvoker{}, &fakePermissionStore{},
	)

	_, err := orchestrator.Plan(t.Context(), adminUser(), "質問", nil, nil, "", "nope", nil)

	require.ErrorIs(t, err, usecase.ErrEndpointNotFound)
	assert.Empty(t, planner.query, "the planner must never be reached when preferred does not resolve")
}

// TestPlanWithPreferredOutsidePermissionIsErrEndpointNotFound is the same
// error as TestPlanWithUnknownPreferredIsErrEndpointNotFound for a
// different reason: the operation exists, but this person's permissions
// (docs/specs/auth.md, section 5) filtered it out of catalogFor's result
// before Preferred was even looked up - a preferred the user may not see
// must read exactly like one that does not exist, or the wire would leak
// which operations exist beyond what the person may call.
func TestPlanWithPreferredOutsidePermissionIsErrEndpointNotFound(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}
	permissions := &fakePermissionStore{permissions: []domain.Permission{
		{Service: "svc-a", OperationID: "Opa"},
	}}

	orchestrator := usecase.NewOrchestrator(fourEndpointShortlist(), planner, &fakeInvoker{}, permissions)

	_, err := orchestrator.Plan(t.Context(), regularUser(), "質問", nil, nil, "", "Opb", nil)

	require.ErrorIs(t, err, usecase.ErrEndpointNotFound)
	assert.Empty(t, planner.query, "the planner must never be reached when preferred is not permitted")
}
