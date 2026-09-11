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

// inventoryCatalogWithStatusColumn is inventoryCatalog, but ListInventoryItems'
// row schema carries a "status" enum property, mirroring the real
// inventory service's Item schema - so a test can assert on the Japanese
// labels Fields carries for it.
func inventoryCatalogWithStatusColumn() domain.Catalog {
	statusSchema := domain.Schema{
		Type: domain.SchemaTypeString,
		Enum: []string{"allocated", "staged", "quarantined", "consigned"},
		EnumLabels: map[string]string{
			"allocated":   "引当済",
			"staged":      "出荷準備完了",
			"quarantined": "検品保留",
			"consigned":   "預託在庫",
		},
	}

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
					"items": {
						Type: domain.SchemaTypeArray,
						Items: &domain.Schema{
							Type: domain.SchemaTypeObject,
							Properties: map[string]domain.Schema{
								"status": statusSchema,
							},
						},
					},
				},
			},
		},
		{
			Service:     "inventory",
			OperationID: "GetInventoryItem",
			Method:      domain.MethodGet,
			Path:        "/api/inventory/items/{id}",
			Summary:     "在庫アイテムを取得する",
			Response: &domain.Schema{
				Type: domain.SchemaTypeObject,
				Properties: map[string]domain.Schema{
					"status": statusSchema,
				},
			},
		},
	}}
}

func TestPlanTableResultCarriesFieldsFromTheRowSchema(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionCall,
		Service:     "inventory",
		OperationID: "ListInventoryItems",
	}}
	invoker := &fakeInvoker{data: map[string]any{"items": []any{}}}

	orchestrator := usecase.NewOrchestrator(inventoryCatalogWithStatusColumn(), planner, invoker)

	result, err := orchestrator.Plan(t.Context(), "検品保留の在庫を見せて", nil)

	require.NoError(t, err)
	require.NotNil(t, result.Fields)

	status, ok := result.Fields["status"].(map[string]any)
	require.True(t, ok, "fields must carry a status property schema")

	enumLabels, ok := status["enumLabels"].(map[string]string)
	require.True(t, ok, "the status field must carry a structured enumLabels map")
	assert.Equal(t, "検品保留", enumLabels["quarantined"])
	assert.Equal(t, "引当済", enumLabels["allocated"])
}

func TestPlanDetailResultCarriesFieldsFromTheResponseSchema(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionCall,
		Service:     "inventory",
		OperationID: "GetInventoryItem",
	}}
	invoker := &fakeInvoker{data: map[string]any{"status": "quarantined"}}

	orchestrator := usecase.NewOrchestrator(inventoryCatalogWithStatusColumn(), planner, invoker)

	result, err := orchestrator.Plan(t.Context(), "この在庫の詳細を見せて", nil)

	require.NoError(t, err)
	require.NotNil(t, result.Fields)

	status, ok := result.Fields["status"].(map[string]any)
	require.True(t, ok, "fields must carry a status property schema")

	enumLabels, ok := status["enumLabels"].(map[string]string)
	require.True(t, ok, "the status field must carry a structured enumLabels map")
	assert.Equal(t, "検品保留", enumLabels["quarantined"])
}

func TestPlanResultWithNoColumnsHasNoFields(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionCall,
		Service:     "inventory",
		OperationID: "ListInventoryItems",
	}}
	invoker := &fakeInvoker{data: map[string]any{"items": []any{}}}

	// inventoryCatalog's rows have no properties at all, so there is
	// nothing a Fields map could usefully describe.
	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker)

	result, err := orchestrator.Plan(t.Context(), "在庫の一覧を見せて", nil)

	require.NoError(t, err)
	assert.Nil(t, result.Fields)
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

func TestPlanNoneMessageMentionsListCapabilities(t *testing.T) {
	// The "none" message must not be a dead end (docs/specs/orchestration.md,
	// D14): it points at asking what the platform can do.
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker)

	result, err := orchestrator.Plan(t.Context(), "今日の天気は？", nil)

	require.NoError(t, err)
	assert.Contains(t, result.Message, "何ができるの")
}

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

	orchestrator := usecase.NewOrchestrator(twoServiceCatalog(), planner, invoker)

	result, err := orchestrator.Plan(t.Context(), "何ができるの？", nil)

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

func TestPlanListCapabilitiesWithServiceFiltersToThatService(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:    usecase.DecisionListCapabilities,
		Service: "inventory",
	}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(twoServiceCatalog(), planner, invoker)

	result, err := orchestrator.Plan(t.Context(), "在庫について、どういう操作ができる？", nil)

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

	orchestrator := usecase.NewOrchestrator(twoServiceCatalog(), planner, invoker)

	result, err := orchestrator.Plan(t.Context(), "存在しないサービスについて何ができる？", nil)

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindResult, result.Kind, "an unmatched filter is still a result, just an empty one")

	data, ok := result.Data.(map[string]any)
	require.True(t, ok)

	items, ok := data["items"].([]map[string]any)
	require.True(t, ok)
	assert.Empty(t, items, "a service name that matches nothing must not silently fall back to the full catalogue")

	assert.Zero(t, invoker.calls)
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
	assert.Equal(t, map[string]any{"type": "object", "properties": map[string]any{}}, result.Schema)
	assert.Zero(t, invoker.calls, "an unsafe call must never reach the service")
}

// catalogWithStatusEnum is inventoryCatalog plus the "status" query
// parameter ListInventoryItems actually declares (docs/specs section 8):
// an enum with Japanese labels, which is what optionsForParam is meant to
// read instead of trusting a Decision's own Options.
func catalogWithStatusEnum() domain.Catalog {
	c := inventoryCatalog()
	c.Endpoints[0].Parameters = []domain.Parameter{
		{
			Name:     "status",
			In:       "query",
			Required: false,
			Schema: domain.Schema{
				Type: domain.SchemaTypeString,
				Enum: []string{"allocated", "staged", "quarantined", "consigned"},
				EnumLabels: map[string]string{
					"allocated":   "引当済",
					"staged":      "出荷準備完了",
					"quarantined": "検品保留",
					"consigned":   "預託在庫",
				},
			},
		},
	}

	return c
}

func TestPlanAskDecisionReturnsOptionsFromTheCatalogue(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionAsk,
		Service:     "inventory",
		OperationID: "ListInventoryItems",
		Question:    "どのステータスですか？",
		Param:       "status",
		// The catalogue, not this fictitious option a model might have
		// invented, must win: see optionsForParam.
		Options: []domain.Option{{Value: "bogus", Label: "でたらめ"}},
	}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(catalogWithStatusEnum(), planner, invoker)

	result, err := orchestrator.Plan(t.Context(), "検品保留の在庫を見せて", nil)

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindAsk, result.Kind)
	assert.Equal(t, "どのステータスですか？", result.Question)
	assert.Equal(t, "status", result.Param)
	assert.ElementsMatch(t, []domain.Option{
		{Value: "allocated", Label: "引当済"},
		{Value: "staged", Label: "出荷準備完了"},
		{Value: "quarantined", Label: "検品保留"},
		{Value: "consigned", Label: "預託在庫"},
	}, result.Options)
	assert.Zero(t, invoker.calls, "an ask decision must never call a service")
}

func TestPlanAskDecisionFillsInAMissingLabelDefensively(t *testing.T) {
	c := inventoryCatalog()
	c.Endpoints[0].Parameters = []domain.Parameter{
		{
			Name:   "status",
			Schema: domain.Schema{Type: domain.SchemaTypeString, Enum: []string{"allocated"}, EnumLabels: map[string]string{}},
		},
	}

	planner := &fakePlanner{decision: usecase.Decision{
		Kind: usecase.DecisionAsk, Service: "inventory", OperationID: "ListInventoryItems", Param: "status",
	}}
	orchestrator := usecase.NewOrchestrator(c, planner, &fakeInvoker{})

	result, err := orchestrator.Plan(t.Context(), "在庫の一覧を見せて", nil)

	require.NoError(t, err)
	assert.Equal(t, []domain.Option{{Value: "allocated", Label: ""}}, result.Options)
}

// TestPlanAskDecisionForAFreeTextParamReturnsAForm drives the fix this
// repository shipped for a real 500: the model reaches for ask_user not
// only to disambiguate an enum, but also when it simply does not know what
// value to use for a required free-text field ("在庫を登録したい" names no
// item, so the model asks about CreateInventoryItem's "name"). "name" is a
// plain string with no declared enum, so optionsForParam cannot find
// anything to offer - and there is nothing to offer, since a free-text
// value cannot be chosen from a list. The only person who can supply it is
// the one asking, so the answer is the same form an unsafe call already
// produces, not an error.
func TestPlanAskDecisionForAFreeTextParamReturnsAForm(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionAsk,
		Service:     "inventory",
		OperationID: "CreateInventoryItem",
		Question:    "アイテム名を教えてください",
		Param:       "name",
		Args:        map[string]any{"status": "allocated"},
	}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker)

	result, err := orchestrator.Plan(t.Context(), "在庫を登録したい", nil)

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindForm, result.Kind)
	assert.Equal(t, "inventory", result.Service)
	assert.Equal(t, "CreateInventoryItem", result.OperationID)
	assert.Equal(t, map[string]any{"status": "allocated"}, result.Initial)
	assert.Equal(t, map[string]any{"type": "object", "properties": map[string]any{}}, result.Schema)
	assert.Zero(t, invoker.calls, "a degraded ask must never call a service")
}

// TestPlanAskDecisionForANameTheEndpointDoesNotDeclareAlsoReturnsAForm
// covers the defensive case optionsForParam already guarded: a param name
// the endpoint does not declare at all (a model's mistake, not just a
// free-text field). It degrades the same way a free-text field does, for
// the same reason - there is no list of values in the catalogue to offer,
// so there is nothing an ask can do that a form cannot.
//
// It also proves the form's schema now describes the endpoint's whole
// argument set, not just a request body: ListInventoryItems has no
// RequestBody at all, only the "status" query parameter catalogWithStatusEnum
// declares, and that parameter must still appear in the form's Schema -
// the gap formSchema (request-body-only) left open for a GET-shaped
// endpoint like GetInventoryItem.
func TestPlanAskDecisionForANameTheEndpointDoesNotDeclareAlsoReturnsAForm(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind: usecase.DecisionAsk, Service: "inventory", OperationID: "ListInventoryItems", Param: "no-such-param",
	}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(catalogWithStatusEnum(), planner, invoker)

	result, err := orchestrator.Plan(t.Context(), "検品保留の在庫を見せて", nil)

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindForm, result.Kind)
	assert.Equal(t, "inventory", result.Service)
	assert.Equal(t, "ListInventoryItems", result.OperationID)

	properties, ok := result.Schema["properties"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, properties, "status", "a parameter, not just a request body, must reach the form's schema")

	assert.Zero(t, invoker.calls)
}

// TestPlanAskDecisionForAnEnumParamStillAsks is the regression guard for
// the fix above: an ask naming a parameter the endpoint *does* declare as
// an enum must keep returning kind: "ask", not degrade to a form just
// because the degradation path now exists.
func TestPlanAskDecisionForAnEnumParamStillAsks(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind: usecase.DecisionAsk, Service: "inventory", OperationID: "ListInventoryItems", Param: "status",
	}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(catalogWithStatusEnum(), planner, invoker)

	result, err := orchestrator.Plan(t.Context(), "検品保留の在庫を見せて", nil)

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindAsk, result.Kind)
	assert.NotEmpty(t, result.Options)
	assert.Zero(t, invoker.calls)
}

func TestPlanAskDecisionForAnUnknownEndpointFails(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind: usecase.DecisionAsk, Service: "inventory", OperationID: "NoSuchOperation", Param: "status",
	}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker)

	_, err := orchestrator.Plan(t.Context(), "検品保留の在庫を見せて", nil)

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrEndpointNotFound)
	assert.Zero(t, invoker.calls)
}

// TestPlanAskDecisionResolvesTheSameParamNameOnItsOwnService drives the
// design hole this fix closes: "status" is not unique across a catalogue
// of many services. Two endpoints, on two different services, each
// declare a "status" enum parameter with disjoint values; an ask decision
// naming one service's operation must only ever offer that service's
// values, never the other service's, even though the parameter name alone
// cannot tell them apart.
func TestPlanAskDecisionResolvesTheSameParamNameOnItsOwnService(t *testing.T) {
	catalog := domain.Catalog{Endpoints: []domain.Endpoint{
		{
			Service:     "inventory",
			OperationID: "ListInventoryItems",
			Method:      domain.MethodGet,
			Path:        "/api/inventory/items",
			Response:    &domain.Schema{Type: domain.SchemaTypeObject},
			Parameters: []domain.Parameter{{
				Name: "status",
				Schema: domain.Schema{
					Type:       domain.SchemaTypeString,
					Enum:       []string{"allocated", "quarantined"},
					EnumLabels: map[string]string{"allocated": "引当済", "quarantined": "検品保留"},
				},
			}},
		},
		{
			Service:     "attendance",
			OperationID: "ListAttendanceRecords",
			Method:      domain.MethodGet,
			Path:        "/api/attendance/records",
			Response:    &domain.Schema{Type: domain.SchemaTypeObject},
			Parameters: []domain.Parameter{{
				Name: "status",
				Schema: domain.Schema{
					Type:       domain.SchemaTypeString,
					Enum:       []string{"present", "absent"},
					EnumLabels: map[string]string{"present": "出勤", "absent": "欠勤"},
				},
			}},
		},
	}}

	invoker := &fakeInvoker{}

	inventoryPlanner := &fakePlanner{decision: usecase.Decision{
		Kind: usecase.DecisionAsk, Service: "inventory", OperationID: "ListInventoryItems",
		Question: "どのステータスですか？", Param: "status",
	}}
	inventoryOrchestrator := usecase.NewOrchestrator(catalog, inventoryPlanner, invoker)

	inventoryResult, err := inventoryOrchestrator.Plan(t.Context(), "在庫のステータスは？", nil)
	require.NoError(t, err)
	assert.ElementsMatch(t, []domain.Option{
		{Value: "allocated", Label: "引当済"},
		{Value: "quarantined", Label: "検品保留"},
	}, inventoryResult.Options, "an ask naming the inventory operation must only offer inventory's own values")

	attendancePlanner := &fakePlanner{decision: usecase.Decision{
		Kind: usecase.DecisionAsk, Service: "attendance", OperationID: "ListAttendanceRecords",
		Question: "どのステータスですか？", Param: "status",
	}}
	attendanceOrchestrator := usecase.NewOrchestrator(catalog, attendancePlanner, invoker)

	attendanceResult, err := attendanceOrchestrator.Plan(t.Context(), "勤怠のステータスは？", nil)
	require.NoError(t, err)
	assert.ElementsMatch(t, []domain.Option{
		{Value: "present", Label: "出勤"},
		{Value: "absent", Label: "欠勤"},
	}, attendanceResult.Options, "an ask naming the attendance operation must only offer attendance's own values")

	assert.Zero(t, invoker.calls, "an ask decision must never call a service")
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
