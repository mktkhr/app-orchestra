package usecase_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// TestPlanProposalCarriesTheModelsOwnValues is AC-N-101 and section 4's
// "the model's own values win where it gave them": a propose_panel
// decision that names its own component, chart, transform and title
// reaches the result exactly as given, and never touches the invoker
// (N1).
func TestPlanProposalCarriesTheModelsOwnValues(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionProposal,
		Service:     "inventory",
		OperationID: "ListInventoryItems",
		Args:        map[string]any{"status": "quarantined"},
		Component:   domain.ComponentChart,
		View: &domain.View{
			Chart:     &domain.Chart{Category: "status", Value: "count", Kind: domain.ChartKindBar},
			Transform: &domain.Transform{GroupBy: "status", Aggregate: domain.AggregateCount},
		},
		Title: "ステータス別の在庫",
	}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "在庫をステータス別に棒グラフで置いて", nil, nil, "ws-1")

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindProposal, result.Kind)
	assert.Equal(t, "inventory", result.Service)
	assert.Equal(t, "ListInventoryItems", result.OperationID)
	assert.Equal(t, map[string]any{"status": "quarantined"}, result.Args)
	assert.Equal(t, domain.ComponentChart, result.Component)
	require.NotNil(t, result.View)
	require.NotNil(t, result.View.Chart)
	assert.Equal(t, "status", result.View.Chart.Category)
	assert.Equal(t, "count", result.View.Chart.Value)
	assert.Equal(t, domain.ChartKindBar, result.View.Chart.Kind)
	require.NotNil(t, result.View.Transform)
	assert.Equal(t, "status", result.View.Transform.GroupBy)
	assert.Equal(t, domain.AggregateCount, result.View.Transform.Aggregate)
	assert.Equal(t, "ステータス別の在庫", result.Title)
	assert.Zero(t, invoker.calls, "a proposal must never call a service (N1)")
}

// inventoryCatalogWithDisplayNameAndChartHint mirrors
// inventoryCatalogWithChartHint but also names a display name on the
// endpoint, so a single fixture can prove every one of section 4's
// catalogue-sourced fallbacks at once: component from domain.Render, axes
// from x-ui-hint.chart, and title from the operation's display name.
func inventoryCatalogWithDisplayNameAndChartHint() domain.Catalog {
	catalog := inventoryCatalogWithChartHint()
	catalog.Endpoints[0].DisplayName = "在庫一覧"

	return catalog
}

// TestPlanProposalWithNoViewFillsInFromTheCatalogue is section 4's central
// claim: a propose_panel call that leaves component, chart/transform and
// title out gets exactly what the rendering rule and the contract would
// have drawn - the two-step flow this feature replaces, with the form
// already open.
func TestPlanProposalWithNoViewFillsInFromTheCatalogue(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionProposal,
		Service:     "inventory",
		OperationID: "ListInventoryItems",
	}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(inventoryCatalogWithDisplayNameAndChartHint(), planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "在庫をステータス別に置いて", nil, nil, "ws-1")

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindProposal, result.Kind)
	assert.Equal(t, domain.ComponentChart, result.Component, "component from domain.Render, which sees the endpoint's own ChartHint")
	require.NotNil(t, result.View)
	require.NotNil(t, result.View.Chart, "axes from x-ui-hint.chart when the contract declares them")
	assert.Equal(t, "status", result.View.Chart.Category)
	assert.Equal(t, "count", result.View.Chart.Value)
	assert.Equal(t, domain.ChartKindBar, result.View.Chart.Kind)
	assert.Nil(t, result.View.Transform, "the model gave no transform, and the catalogue has no default for one")
	assert.Equal(t, "在庫一覧", result.Title, "title from the operation's display name")
	assert.Zero(t, invoker.calls)
}

// TestPlanProposalComponentFallsBackToRenderResultShapeWithNoChartHint
// proves the fallback is domain.Render's own rule, not merely "chart when
// there is a hint": an endpoint with no chart hint and an array response
// still proposes as a table, the same component a plain answer would have
// drawn.
func TestPlanProposalComponentFallsBackToRenderResultShapeWithNoChartHint(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionProposal,
		Service:     "inventory",
		OperationID: "ListInventoryItems",
	}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "在庫の一覧を置いて", nil, nil, "ws-1")

	require.NoError(t, err)
	assert.Equal(t, domain.ComponentTable, result.Component)
	assert.Nil(t, result.View)
	assert.Equal(t, "ListInventoryItems", result.Title, "with no display name declared, the title falls back to the operation id")
	assert.Zero(t, invoker.calls)
}

// TestPlanProposalOnAnOperationTheUserMayNotCallFails is AC-N-105: a
// proposal naming an operation the person may not call cannot be
// produced, because the catalogue the planner was offered never held it
// (docs/specs/auth.md, A4) - the same ErrEndpointNotFound every other
// unknown-or-forbidden operation produces (see
// TestInvokeRefusesAnOperationTheUserMayNotCallWithTheSameErrorAsUnknown).
// "Cannot happen" is a claim about code somebody will change, so this
// test exists to keep that claim honest.
func TestPlanProposalOnAnOperationTheUserMayNotCallFails(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionProposal,
		Service:     "inventory",
		OperationID: "CreateInventoryItem",
	}}
	invoker := &fakeInvoker{}
	permissions := &fakePermissionStore{
		permissions: []domain.Permission{{Service: "inventory", OperationID: "ListInventoryItems"}},
	}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker, permissions)

	_, err := orchestrator.Plan(t.Context(), regularUser(), "在庫アイテムを作るパネルを置いて", nil, nil, "ws-1")

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrEndpointNotFound)
	assert.Zero(t, invoker.calls, "a proposal the person may not make must never reach the service")
}

// TestPlanProposalOnAnUnknownEndpointFails proves the refusal is not
// specific to permission narrowing: an operation that does not exist in
// the catalogue at all is refused the same way.
func TestPlanProposalOnAnUnknownEndpointFails(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionProposal,
		Service:     "inventory",
		OperationID: "NoSuchOperation",
	}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker, &fakePermissionStore{})

	_, err := orchestrator.Plan(t.Context(), adminUser(), "存在しない操作のパネルを置いて", nil, nil, "ws-1")

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrEndpointNotFound)
	assert.Zero(t, invoker.calls)
}

// TestPlanRefusesAProposalWithNoWorkspaceID is AC-O-104
// (docs/specs/offering.md): a propose_panel decision arriving with no
// workspace id is refused the way an unknown tool is, even though the
// planner itself named a real, permitted operation - the planner here is a
// fakePlanner that ignores the tools it was offered entirely (a stand-in
// for a model that calls propose_panel anyway), so this proves the
// platform itself, not the planner's own good behaviour, is what refuses
// it (O5: "the list is what the model is offered, not what the platform
// trusts"). Nothing is written either way (N1).
func TestPlanRefusesAProposalWithNoWorkspaceID(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionProposal,
		Service:     "inventory",
		OperationID: "ListInventoryItems",
	}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker, &fakePermissionStore{})

	_, err := orchestrator.Plan(t.Context(), adminUser(), "在庫の一覧を置いて", nil, nil, "")

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrToolNotOffered)
	assert.Zero(t, invoker.calls, "a refused proposal must never reach the service")
}
