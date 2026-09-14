package usecase_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// list_capabilities' own tests, split out of orchestrator_test.go only
// because that file is already at harness/quality's max-lines-per-file
// budget (1000) - the same reasoning source.go's own header comments give
// for splitting on that limit elsewhere in this codebase.

// twoServiceCatalog carries endpoints from two services, so tests can tell
// an unfiltered listing apart from a service-filtered one.
func twoServiceCatalog() domain.Catalog {
	return domain.Catalog{Endpoints: []domain.Endpoint{
		{
			Service:     "inventory",
			OperationID: "ListInventoryItems",
			Method:      domain.MethodGet,
			Path:        "/api/inventory/items",
			Summary:     "在庫アイテムの一覧を返す",
			Response:    &domain.Schema{Type: domain.SchemaTypeArray, Items: &domain.Schema{Type: domain.SchemaTypeObject}},
		},
		{
			Service:     "inventory",
			OperationID: "CreateInventoryItem",
			Method:      "POST",
			Path:        "/api/inventory/items",
			Summary:     "在庫アイテムを作成する",
			RequestBody: &domain.Schema{Type: domain.SchemaTypeObject},
		},
		{
			Service:     "attendance",
			OperationID: "ListAttendanceRecords",
			Method:      domain.MethodGet,
			Path:        "/api/attendance/records",
			Summary:     "勤怠記録の一覧を返す",
			Response:    &domain.Schema{Type: domain.SchemaTypeArray, Items: &domain.Schema{Type: domain.SchemaTypeObject}},
		},
	}}
}

func TestPlanListCapabilitiesWithNoServiceListsEveryEndpoint(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionListCapabilities}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(twoServiceCatalog(), planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "何ができるの？", nil, nil, "")

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindResult, result.Kind)
	assert.Equal(t, domain.ComponentTable, result.Component)
	assert.Equal(t, "platform", result.Service)
	assert.Equal(t, "list_capabilities", result.OperationID)

	data, ok := result.Data.(map[string]any)
	require.True(t, ok, "data must be a map carrying an items array")

	items, ok := data["items"].([]map[string]any)
	require.True(t, ok, "data.items must be a []map[string]any so rowsFromData can render it")
	require.Len(t, items, 3)

	assert.Equal(t, []map[string]any{
		{"service": "attendance", "operation": "ListAttendanceRecords", "summary": "勤怠記録の一覧を返す"},
		{"service": "inventory", "operation": "CreateInventoryItem", "summary": "在庫アイテムを作成する"},
		{"service": "inventory", "operation": "ListInventoryItems", "summary": "在庫アイテムの一覧を返す"},
	}, items, "rows must be sorted deterministically by (service, operation)")

	require.NotNil(t, result.Fields)

	serviceField, ok := result.Fields["service"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "サービス", serviceField["title"])

	operationField, ok := result.Fields["operation"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "操作", operationField["title"])

	summaryField, ok := result.Fields["summary"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "できること", summaryField["title"])

	assert.Zero(t, invoker.calls, "list_capabilities must never call a service")
}

// TestPlanListCapabilitiesColumnsPreferDisplayNames is DECISIONS.md's
// 2026-09-13 entries' list_capabilities half, both levels at once: the
// 操作 column shows x-ui-hint.displayName and the サービス column shows
// its service's own info.x-ui-hint.displayName, when the contract
// declares them, rather than the operation id/identifier every row showed
// before either field existed. The filter itself still matches the
// identifier decision.Service names, not the label -
// capabilitiesItems' own doc comment on that.
func TestPlanListCapabilitiesColumnsPreferDisplayNames(t *testing.T) {
	catalog := domain.Catalog{Endpoints: []domain.Endpoint{
		{
			Service:            "inventory",
			ServiceDisplayName: "在庫管理",
			OperationID:        "ListInventoryItems",
			Summary:            "List stock items, optionally filtered by status.",
			DisplayName:        "在庫一覧",
			Response:           &domain.Schema{Type: domain.SchemaTypeArray, Items: &domain.Schema{Type: domain.SchemaTypeObject}},
		},
	}}

	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionListCapabilities}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(catalog, planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "何ができるの？", nil, nil, "")
	require.NoError(t, err)

	data, ok := result.Data.(map[string]any)
	require.True(t, ok)
	items, ok := data["items"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, items, 1)

	assert.Equal(t, "在庫一覧", items[0]["operation"])
	assert.Equal(t, "在庫管理", items[0]["service"])
}

func TestPlanListCapabilitiesWithServiceFiltersToThatService(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:    usecase.DecisionListCapabilities,
		Service: "inventory",
	}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(twoServiceCatalog(), planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "在庫について、どういう操作ができる？", nil, nil, "")

	require.NoError(t, err)

	data, ok := result.Data.(map[string]any)
	require.True(t, ok)

	items, ok := data["items"].([]map[string]any)
	require.True(t, ok)

	for _, item := range items {
		assert.Equal(t, "inventory", item["service"])
	}

	assert.Equal(t, []map[string]any{
		{"service": "inventory", "operation": "CreateInventoryItem", "summary": "在庫アイテムを作成する"},
		{"service": "inventory", "operation": "ListInventoryItems", "summary": "在庫アイテムの一覧を返す"},
	}, items)

	assert.Zero(t, invoker.calls)
}

func TestPlanListCapabilitiesWithUnknownServiceReturnsNoRows(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:    usecase.DecisionListCapabilities,
		Service: "no-such-service",
	}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(twoServiceCatalog(), planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "存在しないサービスについて何ができる？", nil, nil, "")

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindResult, result.Kind, "an unmatched filter is still a result, just an empty one")

	data, ok := result.Data.(map[string]any)
	require.True(t, ok)

	items, ok := data["items"].([]map[string]any)
	require.True(t, ok)
	assert.Empty(t, items, "a service name that matches nothing must not silently fall back to the full catalogue")

	assert.Zero(t, invoker.calls)
}
