package usecase_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// TestPlanCarriesThinkingToThePlannerOnTheOrdinaryPath is the usecase half
// of the effective-value rule toolcall.Planner.Plan implements: whatever
// value Plan was asked with reaches the planner's own Plan call unchanged,
// on the ordinary (non-preferred) path.
func TestPlanCarriesThinkingToThePlannerOnTheOrdinaryPath(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}
	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, &fakeInvoker{}, &fakePermissionStore{})

	_, err := orchestrator.Plan(t.Context(), adminUser(), "在庫の一覧を見せて", nil, nil, "", "", new(bool))

	require.NoError(t, err)
	require.NotNil(t, planner.thinking)
	assert.False(t, *planner.thinking)
}

// TestPlanCarriesThinkingTrueToThePlannerOnTheOrdinaryPath is the other
// value, on the same path - proving the value is carried through, not
// merely defaulted to false by coincidence.
func TestPlanCarriesThinkingTrueToThePlannerOnTheOrdinaryPath(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}
	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, &fakeInvoker{}, &fakePermissionStore{})

	_, err := orchestrator.Plan(t.Context(), adminUser(), "在庫の一覧を見せて", nil, nil, "", "", new(true))

	require.NoError(t, err)
	require.NotNil(t, planner.thinking)
	assert.True(t, *planner.thinking)
}

// TestPlanCarriesNoThinkingOverrideWhenTheRequestOmitsIt is the "absent"
// case: a request that never set PlanRequest.thinking reaches the planner
// as nil, exactly as it did before this override existed - the toolcall
// planner's own configured default decides, not this layer.
func TestPlanCarriesNoThinkingOverrideWhenTheRequestOmitsIt(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}
	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, &fakeInvoker{}, &fakePermissionStore{})

	_, err := orchestrator.Plan(t.Context(), adminUser(), "在庫の一覧を見せて", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Nil(t, planner.thinking)
}

// TestPlanCarriesThinkingToThePlannerOnThePreferredPath is the same rule
// on planPreferred's own call to o.planner.Plan (orchestrator_preferred.go)
// - a question re-planned with PlanRequest.preferred set must carry the
// same thinking value it was asked with, exactly as the ordinary path
// does.
func TestPlanCarriesThinkingToThePlannerOnThePreferredPath(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind: usecase.DecisionCall, Service: "svc-c", OperationID: "Opc",
	}}

	orchestrator := usecase.NewOrchestrator(
		fourEndpointShortlist(), planner, &fakeInvoker{data: map[string]any{}}, &fakePermissionStore{},
	)

	_, err := orchestrator.Plan(t.Context(), adminUser(), "質問", nil, nil, "", "Opc", new(bool))

	require.NoError(t, err)
	require.NotNil(t, planner.thinking)
	assert.False(t, *planner.thinking)
}
