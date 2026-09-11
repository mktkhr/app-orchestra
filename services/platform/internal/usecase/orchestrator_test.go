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

// fakePlanner is a test double for usecase.Planner: it always returns the
// fixed decision (or error) it was built with, and records the arguments
// it was called with so a test can assert on them.
type fakePlanner struct {
	decision usecase.Decision
	err      error

	query   string
	answers []usecase.Answer
	tools   []usecase.Tool
}

func (f *fakePlanner) Plan(_ context.Context, query string, answers []usecase.Answer, tools []usecase.Tool) (usecase.Decision, error) {
	f.query = query
	f.answers = answers
	f.tools = tools

	return f.decision, f.err
}

// fakeInvoker is a test double for usecase.Invoker: it always returns the
// fixed data (or error) it was built with, and records the endpoint and
// args it was called with.
type fakeInvoker struct {
	data any
	err  error

	endpoint domain.Endpoint
	args     map[string]any
	calls    int
}

func (f *fakeInvoker) Invoke(_ context.Context, e *domain.Endpoint, args map[string]any) (any, error) {
	f.endpoint = *e
	f.args = args
	f.calls++

	return f.data, f.err
}

func inventoryCatalog() domain.Catalog {
	return domain.Catalog{Endpoints: []domain.Endpoint{
		{
			Service:     "inventory",
			OperationID: "ListInventoryItems",
			Method:      domain.MethodGet,
			Path:        "/api/inventory/items",
			Summary:     "在庫アイテムの一覧を返す",
			Response: &domain.Schema{
				Type: domain.SchemaTypeObject,
				Properties: map[string]domain.Schema{
					"items": {Type: domain.SchemaTypeArray, Items: &domain.Schema{Type: domain.SchemaTypeObject}},
				},
			},
		},
		{
			Service:     "inventory",
			OperationID: "CreateInventoryItem",
			Method:      "POST",
			Path:        "/api/inventory/items",
			Summary:     "在庫アイテムを作成する",
			RequestBody: &domain.Schema{Type: domain.SchemaTypeObject},
			Response:    &domain.Schema{Type: domain.SchemaTypeObject},
		},
	}}
}

func TestPlanSafeCallInvokesAndRendersTheResult(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionCall,
		Service:     "inventory",
		OperationID: "ListInventoryItems",
		Args:        map[string]any{"status": "allocated"},
	}}
	invoker := &fakeInvoker{data: map[string]any{"items": []any{}}}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker)

	result, err := orchestrator.Plan(t.Context(), "在庫の一覧を見せて", nil)

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindResult, result.Kind)
	assert.Equal(t, domain.ComponentTable, result.Component)
	assert.Equal(t, map[string]any{"items": []any{}}, result.Data)
	assert.Equal(t, "inventory", result.Service)
	assert.Equal(t, "ListInventoryItems", result.OperationID)
	assert.Equal(t, map[string]any{"status": "allocated"}, result.Args)

	assert.Equal(t, 1, invoker.calls)
	assert.Equal(t, "ListInventoryItems", invoker.endpoint.OperationID)
	assert.Equal(t, map[string]any{"status": "allocated"}, invoker.args)

	assert.Equal(t, "在庫の一覧を見せて", planner.query)
	assert.NotEmpty(t, planner.tools, "the orchestrator must offer the planner the catalogue's tools")
}

func TestPlanNoneCallsNothingAndReturnsAMessage(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker)

	result, err := orchestrator.Plan(t.Context(), "今日の天気は？", nil)

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindNone, result.Kind)
	assert.NotEmpty(t, result.Message)
	assert.Zero(t, invoker.calls, "a none decision must not invoke anything")
}

func TestPlanUnsafeCallReturnsAFormWithoutInvoking(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionCall,
		Service:     "inventory",
		OperationID: "CreateInventoryItem",
		Args:        map[string]any{"name": "widget", "status": "allocated"},
	}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker)

	result, err := orchestrator.Plan(t.Context(), "在庫を登録して", nil)

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindForm, result.Kind)
	assert.Equal(t, "inventory", result.Service)
	assert.Equal(t, "CreateInventoryItem", result.OperationID)
	assert.Equal(t, map[string]any{"name": "widget", "status": "allocated"}, result.Initial)
	assert.Equal(t, map[string]any{"type": "object"}, result.Schema)
	assert.Zero(t, invoker.calls, "an unsafe call must never reach the service")
}

func TestPlanAskDecisionIsNotImplemented(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:     usecase.DecisionAsk,
		Question: "どのステータスですか？",
		Param:    "status",
	}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker)

	_, err := orchestrator.Plan(t.Context(), "検品保留の在庫を見せて", nil)

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrNotImplemented)
	assert.Zero(t, invoker.calls)
}

func TestPlanCallToUnknownEndpointFails(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionCall,
		Service:     "inventory",
		OperationID: "NoSuchOperation",
	}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker)

	_, err := orchestrator.Plan(t.Context(), "存在しない操作", nil)

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrEndpointNotFound)
}

func TestPlanUnknownDecisionKindIsNotImplemented(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionKind("bogus")}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker)

	_, err := orchestrator.Plan(t.Context(), "何か", nil)

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrNotImplemented)
}

func TestPlanWrapsAPlannerError(t *testing.T) {
	boom := errors.New("boom")
	planner := &fakePlanner{err: boom}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker)

	_, err := orchestrator.Plan(t.Context(), "在庫の一覧を見せて", nil)

	require.Error(t, err)
	require.ErrorIs(t, err, boom)
}

func TestPlanWrapsAnInvokerError(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionCall,
		Service:     "inventory",
		OperationID: "ListInventoryItems",
	}}
	boom := errors.New("service unreachable")
	invoker := &fakeInvoker{err: boom}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker)

	_, err := orchestrator.Plan(t.Context(), "在庫の一覧を見せて", nil)

	require.Error(t, err)
	require.ErrorIs(t, err, boom)
}

func TestPlanPassesAnswersThrough(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker)

	answers := []usecase.Answer{{Param: "status", Value: "allocated"}}
	_, err := orchestrator.Plan(t.Context(), "在庫の一覧を見せて", answers)

	require.NoError(t, err)
	assert.Equal(t, answers, planner.answers)
}
