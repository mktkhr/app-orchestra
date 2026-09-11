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
	orchestrator := usecase.NewOrchestrator(invokeCatalog(), &fakePlanner{}, invoker, &fakePermissionStore{})

	result, err := orchestrator.Invoke(t.Context(), adminUser(), "inventory", "CreateInventoryItem",
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
	orchestrator := usecase.NewOrchestrator(invokeCatalog(), planner, invoker, &fakePermissionStore{})

	_, err := orchestrator.Invoke(t.Context(), adminUser(), "inventory", "ListInventoryItems", nil)

	require.NoError(t, err)
	assert.Empty(t, planner.query, "Invoke must never consult the planner")
}

func TestInvokeUnknownEndpointFailsAndCallsNothing(t *testing.T) {
	invoker := &fakeInvoker{}
	orchestrator := usecase.NewOrchestrator(invokeCatalog(), &fakePlanner{}, invoker, &fakePermissionStore{})

	_, err := orchestrator.Invoke(t.Context(), adminUser(), "inventory", "NoSuchOperation", map[string]any{})

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrEndpointNotFound)
	assert.Zero(t, invoker.calls, "an unknown operation must never reach the service")
}

func TestInvokeRejectsAMissingRequiredBodyField(t *testing.T) {
	invoker := &fakeInvoker{}
	orchestrator := usecase.NewOrchestrator(invokeCatalog(), &fakePlanner{}, invoker, &fakePermissionStore{})

	_, err := orchestrator.Invoke(t.Context(), adminUser(), "inventory", "CreateInventoryItemStrict",
		map[string]any{"name": "棚"})

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrInvalidArguments)
	assert.Zero(t, invoker.calls, "invalid arguments must never reach the service")
}

func TestInvokeRejectsAnOutOfEnumBodyValue(t *testing.T) {
	invoker := &fakeInvoker{}
	orchestrator := usecase.NewOrchestrator(invokeCatalog(), &fakePlanner{}, invoker, &fakePermissionStore{})

	_, err := orchestrator.Invoke(t.Context(), adminUser(), "inventory", "CreateInventoryItemStrict",
		map[string]any{"name": "棚", "status": "nonexistent"})

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrInvalidArguments)
	assert.Zero(t, invoker.calls, "invalid arguments must never reach the service")
}

func TestInvokeRejectsAnOutOfEnumQueryParameter(t *testing.T) {
	invoker := &fakeInvoker{}
	orchestrator := usecase.NewOrchestrator(invokeCatalog(), &fakePlanner{}, invoker, &fakePermissionStore{})

	_, err := orchestrator.Invoke(t.Context(), adminUser(), "inventory", "ListInventoryItemsByStatus",
		map[string]any{"status": "nonexistent"})

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrInvalidArguments)
	assert.Zero(t, invoker.calls, "invalid arguments must never reach the service")
}

func TestInvokeAcceptsAValidEnumQueryParameter(t *testing.T) {
	invoker := &fakeInvoker{data: map[string]any{"items": []any{}}}
	orchestrator := usecase.NewOrchestrator(invokeCatalog(), &fakePlanner{}, invoker, &fakePermissionStore{})

	_, err := orchestrator.Invoke(t.Context(), adminUser(), "inventory", "ListInventoryItemsByStatus",
		map[string]any{"status": "allocated"})

	require.NoError(t, err)
	assert.Equal(t, 1, invoker.calls)
}

func TestInvokeWrapsAnInvokerError(t *testing.T) {
	boom := errors.New("service unreachable")
	invoker := &fakeInvoker{err: boom}
	orchestrator := usecase.NewOrchestrator(invokeCatalog(), &fakePlanner{}, invoker, &fakePermissionStore{})

	_, err := orchestrator.Invoke(t.Context(), adminUser(), "inventory", "ListInventoryItems", nil)

	require.Error(t, err)
	require.ErrorIs(t, err, boom)
	require.NotErrorIs(t, err, usecase.ErrEndpointNotFound)
	require.NotErrorIs(t, err, usecase.ErrInvalidArguments)
}

// TestInvokeRefusesAnOperationTheUserMayNotCallWithTheSameErrorAsUnknown
// is AC-A-104: a person who may call ListInventoryItems but not
// CreateInventoryItem gets exactly the error an unknown operation would -
// ErrEndpointNotFound, not a distinct "forbidden" sentinel - because
// telling the two apart would let a 403 answer "does this exist?" for an
// operation the person was never offered in the first place
// (docs/specs/auth.md, section 5).
func TestInvokeRefusesAnOperationTheUserMayNotCallWithTheSameErrorAsUnknown(t *testing.T) {
	invoker := &fakeInvoker{data: map[string]any{"items": []any{}}}
	permissions := &fakePermissionStore{
		permissions: []domain.Permission{{Service: "inventory", OperationID: "ListInventoryItems"}},
	}
	orchestrator := usecase.NewOrchestrator(invokeCatalog(), &fakePlanner{}, invoker, permissions)

	_, err := orchestrator.Invoke(t.Context(), regularUser(), "inventory", "CreateInventoryItem",
		map[string]any{"name": "棚", "status": "allocated"})

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrEndpointNotFound)
	assert.Zero(t, invoker.calls, "an operation the person may not call must never reach the service")
	assert.Equal(t, "user-1", permissions.userID, "Invoke must read the calling user's own permissions")
}

// TestInvokeAllowsAnOperationTheUserMayCall proves the narrowing is not
// simply "deny everything for a non-admin": the one operation
// fakePermissionStore actually names for regularUser still goes through.
func TestInvokeAllowsAnOperationTheUserMayCall(t *testing.T) {
	invoker := &fakeInvoker{data: map[string]any{"items": []any{}}}
	permissions := &fakePermissionStore{
		permissions: []domain.Permission{{Service: "inventory", OperationID: "ListInventoryItems"}},
	}
	orchestrator := usecase.NewOrchestrator(invokeCatalog(), &fakePlanner{}, invoker, permissions)

	_, err := orchestrator.Invoke(t.Context(), regularUser(), "inventory", "ListInventoryItems", nil)

	require.NoError(t, err)
	assert.Equal(t, 1, invoker.calls)
}

// TestInvokeAsAdminNeverConsultsThePermissionStore proves admin's access
// comes from the role, not from rows (docs/specs/auth.md, section 4): an
// admin can call an operation even though the permission store behind
// them holds nothing.
func TestInvokeAsAdminNeverConsultsThePermissionStore(t *testing.T) {
	invoker := &fakeInvoker{data: map[string]any{"items": []any{}}}
	permissions := &fakePermissionStore{}
	orchestrator := usecase.NewOrchestrator(invokeCatalog(), &fakePlanner{}, invoker, permissions)

	_, err := orchestrator.Invoke(t.Context(), adminUser(), "inventory", "ListInventoryItems", nil)

	require.NoError(t, err)
	assert.Empty(t, permissions.userID, "an admin's access must never depend on a call to PermissionStore.For")
}
