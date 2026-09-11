package usecase_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// invokeCatalog extends inventoryCatalog with the extra endpoints these
// tests need: one with a required request body field and an enum-valued
// body property, and one with an enum-valued query parameter. Kept
// separate from inventoryCatalog (orchestrator_test.go) so those tests'
// exact-schema assertions do not have to change.
func invokeCatalog() domain.Catalog {
	catalog := inventoryCatalog()
	catalog.Endpoints = append(catalog.Endpoints,
		domain.Endpoint{
			Service:     "inventory",
			OperationID: "CreateInventoryItemStrict",
			Method:      "POST",
			Path:        "/api/inventory/items",
			RequestBody: &domain.Schema{
				Type:     domain.SchemaTypeObject,
				Required: []string{"name", "status"},
				Properties: map[string]domain.Schema{
					"name":   {Type: domain.SchemaTypeString},
					"status": {Type: domain.SchemaTypeString, Enum: []string{"allocated", "staged"}},
				},
			},
			Response: &domain.Schema{Type: domain.SchemaTypeObject},
		},
		domain.Endpoint{
			Service:     "inventory",
			OperationID: "ListInventoryItemsByStatus",
			Method:      domain.MethodGet,
			Path:        "/api/inventory/items",
			Parameters: []domain.Parameter{
				{Name: "status", In: "query", Schema: domain.Schema{Type: domain.SchemaTypeString, Enum: []string{"allocated", "staged"}}},
			},
			Response: &domain.Schema{
				Type: domain.SchemaTypeObject,
				Properties: map[string]domain.Schema{
					"items": {Type: domain.SchemaTypeArray, Items: &domain.Schema{Type: domain.SchemaTypeObject}},
				},
			},
		},
	)

	return catalog
}

func TestInvokeCallsTheServiceAndRendersDetailEvenThoughTheEndpointHasARequestBody(t *testing.T) {
	invoker := &fakeInvoker{data: map[string]any{"id": "1", "name": "新しい棚", "status": "allocated"}}
	orchestrator := usecase.NewOrchestrator(invokeCatalog(), &fakePlanner{}, invoker)

	result, err := orchestrator.Invoke(t.Context(), "inventory", "CreateInventoryItem",
		map[string]any{"name": "新しい棚", "status": "allocated"})

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindResult, result.Kind)
	// The endpoint has a request body (it is unsafe), but it has already
	// been invoked - unlike Plan, Invoke does not turn it into a form.
	// domain.Render would say "form" for an endpoint with a request body;
	// the response here is a single object, so the result must render as
	// "detail".
	assert.Equal(t, domain.ComponentDetail, result.Component)
	assert.Equal(t, "inventory", result.Service)
	assert.Equal(t, "CreateInventoryItem", result.OperationID)
	assert.Equal(t, map[string]any{"name": "新しい棚", "status": "allocated"}, result.Args)
	assert.Equal(t, map[string]any{"id": "1", "name": "新しい棚", "status": "allocated"}, result.Data)

	assert.Equal(t, 1, invoker.calls)
	assert.Equal(t, "CreateInventoryItem", invoker.endpoint.OperationID)
}

func TestInvokeNeverCallsThePlanner(t *testing.T) {
	planner := &fakePlanner{}
	invoker := &fakeInvoker{data: map[string]any{"items": []any{}}}
	orchestrator := usecase.NewOrchestrator(invokeCatalog(), planner, invoker)

	_, err := orchestrator.Invoke(t.Context(), "inventory", "ListInventoryItems", nil)

	require.NoError(t, err)
	assert.Empty(t, planner.query, "Invoke must never consult the planner")
}

func TestInvokeUnknownEndpointFailsAndCallsNothing(t *testing.T) {
	invoker := &fakeInvoker{}
	orchestrator := usecase.NewOrchestrator(invokeCatalog(), &fakePlanner{}, invoker)

	_, err := orchestrator.Invoke(t.Context(), "inventory", "NoSuchOperation", map[string]any{})

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrEndpointNotFound)
	assert.Zero(t, invoker.calls, "an unknown operation must never reach the service")
}

func TestInvokeRejectsAMissingRequiredBodyField(t *testing.T) {
	invoker := &fakeInvoker{}
	orchestrator := usecase.NewOrchestrator(invokeCatalog(), &fakePlanner{}, invoker)

	_, err := orchestrator.Invoke(t.Context(), "inventory", "CreateInventoryItemStrict",
		map[string]any{"name": "棚"})

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrInvalidArguments)
	assert.Zero(t, invoker.calls, "invalid arguments must never reach the service")
}

func TestInvokeRejectsAnOutOfEnumBodyValue(t *testing.T) {
	invoker := &fakeInvoker{}
	orchestrator := usecase.NewOrchestrator(invokeCatalog(), &fakePlanner{}, invoker)

	_, err := orchestrator.Invoke(t.Context(), "inventory", "CreateInventoryItemStrict",
		map[string]any{"name": "棚", "status": "nonexistent"})

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrInvalidArguments)
	assert.Zero(t, invoker.calls, "invalid arguments must never reach the service")
}

func TestInvokeRejectsAnOutOfEnumQueryParameter(t *testing.T) {
	invoker := &fakeInvoker{}
	orchestrator := usecase.NewOrchestrator(invokeCatalog(), &fakePlanner{}, invoker)

	_, err := orchestrator.Invoke(t.Context(), "inventory", "ListInventoryItemsByStatus",
		map[string]any{"status": "nonexistent"})

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrInvalidArguments)
	assert.Zero(t, invoker.calls, "invalid arguments must never reach the service")
}

func TestInvokeAcceptsAValidEnumQueryParameter(t *testing.T) {
	invoker := &fakeInvoker{data: map[string]any{"items": []any{}}}
	orchestrator := usecase.NewOrchestrator(invokeCatalog(), &fakePlanner{}, invoker)

	_, err := orchestrator.Invoke(t.Context(), "inventory", "ListInventoryItemsByStatus",
		map[string]any{"status": "allocated"})

	require.NoError(t, err)
	assert.Equal(t, 1, invoker.calls)
}

func TestInvokeWrapsAnInvokerError(t *testing.T) {
	boom := errors.New("service unreachable")
	invoker := &fakeInvoker{err: boom}
	orchestrator := usecase.NewOrchestrator(invokeCatalog(), &fakePlanner{}, invoker)

	_, err := orchestrator.Invoke(t.Context(), "inventory", "ListInventoryItems", nil)

	require.Error(t, err)
	require.ErrorIs(t, err, boom)
	require.NotErrorIs(t, err, usecase.ErrEndpointNotFound)
	require.NotErrorIs(t, err, usecase.ErrInvalidArguments)
}
