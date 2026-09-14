package usecase_test

import (
	"context"
	"errors"
	"fmt"
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
	turns   []usecase.Turn
	tools   []usecase.Tool
}

func (f *fakePlanner) Plan(
	_ context.Context, query string, answers []usecase.Answer, turns []usecase.Turn, tools []usecase.Tool,
) (usecase.Decision, error) {
	f.query = query
	f.answers = answers
	f.turns = turns
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

// fakePermissionStore is a test double for usecase.PermissionStore. Most
// tests in this file drive the orchestrator as adminUser(), which never
// reaches it (see Orchestrator.catalogFor) - it exists for the tests that
// deliberately drive a non-admin user through permission narrowing.
type fakePermissionStore struct {
	permissions []domain.Permission
	err         error

	userID string
}

func (f *fakePermissionStore) For(_ context.Context, userID string) ([]domain.Permission, error) {
	f.userID = userID

	return f.permissions, f.err
}

func (f *fakePermissionStore) Set(context.Context, string, []domain.Permission) error {
	return nil
}

// adminUser is the *domain.User most tests in this file drive the
// orchestrator as: an admin holds every permission implicitly
// (docs/specs/auth.md, section 4), so these tests exercise Plan/Invoke's
// own behaviour without also depending on a fakePermissionStore's
// contents - the same reason nearly every test predating auth built a
// catalogue directly rather than through a permission filter.
func adminUser() *domain.User {
	return &domain.User{ID: "admin-1", Role: domain.RoleAdmin}
}

// regularUser is a *domain.User with no role privilege of its own: what
// they may call comes entirely from the permissions a fakePermissionStore
// hands back for their ID.
func regularUser() *domain.User {
	return &domain.User{ID: "user-1", Role: domain.RoleUser}
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

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "在庫の一覧を見せて", nil, nil, "")

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindResult, result.Kind)
	assert.Equal(t, domain.ComponentTable, result.Component)
	assert.Equal(t, map[string]any{"items": []any{}}, result.Data)
	assert.Equal(t, "inventory", result.Service)
	assert.Equal(t, "inventory", result.ServiceDisplayName,
		"inventoryCatalog declares no info.x-ui-hint.displayName, so this falls back to the identifier")
	assert.Equal(t, "ListInventoryItems", result.OperationID)
	assert.Equal(t, map[string]any{"status": "allocated"}, result.Args)

	assert.Equal(t, 1, invoker.calls)
	assert.Equal(t, "ListInventoryItems", invoker.endpoint.OperationID)
	assert.Equal(t, map[string]any{"status": "allocated"}, invoker.args)

	assert.Equal(t, "在庫の一覧を見せて", planner.query)
	assert.NotEmpty(t, planner.tools, "the orchestrator must offer the planner the catalogue's tools")
}

// TestPlanSafeCallPrefersTheServicesOwnDisplayName is DECISIONS.md's
// 2026-09-13 entry, one level up from an operation's own DisplayName: a
// service whose contract declares info.x-ui-hint.displayName carries it
// onto a safe call's result, for Provenance.tsx to read instead of the
// identifier.
func TestPlanSafeCallPrefersTheServicesOwnDisplayName(t *testing.T) {
	catalog := inventoryCatalog()
	for i := range catalog.Endpoints {
		catalog.Endpoints[i].ServiceDisplayName = "在庫管理"
	}

	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionCall,
		Service:     "inventory",
		OperationID: "ListInventoryItems",
	}}
	invoker := &fakeInvoker{data: map[string]any{"items": []any{}}}

	orchestrator := usecase.NewOrchestrator(catalog, planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "在庫の一覧を見せて", nil, nil, "")

	require.NoError(t, err)
	assert.Equal(t, "在庫管理", result.ServiceDisplayName)
}

// TestPlanWithNoTurnsBehavesExactlyAsBefore is AC-M-105: a question with no
// turns is answered exactly as it is today, and the planner sees no turns
// at all rather than an empty-but-non-nil slice standing in for "history".
func TestPlanWithNoTurnsBehavesExactlyAsBefore(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}
	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, &fakeInvoker{}, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "在庫の一覧を見せて", nil, nil, "")

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindNone, result.Kind)
	assert.Nil(t, planner.turns)
}

// TestPlanTruncatesTurnsToTheConfiguredWindow is AC-M-104: a conversation
// longer than the window sends the planner only the most recent turns,
// oldest dropped first (docs/specs/context.md, section 6).
func TestPlanTruncatesTurnsToTheConfiguredWindow(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}
	orchestrator := usecase.NewOrchestrator(
		inventoryCatalog(), planner, &fakeInvoker{}, &fakePermissionStore{}, usecase.WithContextWindow(2),
	)

	turns := []usecase.Turn{
		{Question: "1つ目", Kind: usecase.ResultKindResult, Service: "inventory", OperationID: "ListInventoryItems"},
		{Question: "2つ目", Kind: usecase.ResultKindResult, Service: "inventory", OperationID: "ListInventoryItems"},
		{Question: "3つ目", Kind: usecase.ResultKindResult, Service: "inventory", OperationID: "ListInventoryItems"},
	}

	_, err := orchestrator.Plan(t.Context(), adminUser(), "4つ目", nil, turns, "")

	require.NoError(t, err)
	require.Len(t, planner.turns, 2)
	assert.Equal(t, "2つ目", planner.turns[0].Question)
	assert.Equal(t, "3つ目", planner.turns[1].Question)
}

// TestPlanKeepsEveryTurnWhenFewerThanTheWindow shows the window is not a
// fixed padding: a conversation shorter than it passes through unchanged.
func TestPlanKeepsEveryTurnWhenFewerThanTheWindow(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}
	orchestrator := usecase.NewOrchestrator(
		inventoryCatalog(), planner, &fakeInvoker{}, &fakePermissionStore{}, usecase.WithContextWindow(5),
	)

	turns := []usecase.Turn{
		{Question: "1つ目", Kind: usecase.ResultKindResult, Service: "inventory", OperationID: "ListInventoryItems"},
		{Question: "2つ目", Kind: usecase.ResultKindResult, Service: "inventory", OperationID: "ListInventoryItems"},
	}

	_, err := orchestrator.Plan(t.Context(), adminUser(), "3つ目", nil, turns, "")

	require.NoError(t, err)
	assert.Equal(t, turns, planner.turns)
}

// TestPlanUsesTheDefaultWindowWhenNoneIsConfigured pins
// usecase.DefaultContextWindow's value against a NewOrchestrator built
// without WithContextWindow - every caller before this task, and every
// existing test in this file.
func TestPlanUsesTheDefaultWindowWhenNoneIsConfigured(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}
	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, &fakeInvoker{}, &fakePermissionStore{})

	turns := make([]usecase.Turn, usecase.DefaultContextWindow+3)
	for i := range turns {
		turns[i] = usecase.Turn{Question: fmt.Sprintf("質問%d", i), Kind: usecase.ResultKindNone}
	}

	_, err := orchestrator.Plan(t.Context(), adminUser(), "最後の質問", nil, turns, "")

	require.NoError(t, err)
	require.Len(t, planner.turns, usecase.DefaultContextWindow)
	assert.Equal(t, turns[len(turns)-usecase.DefaultContextWindow:], planner.turns)
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

	orchestrator := usecase.NewOrchestrator(inventoryCatalogWithStatusColumn(), planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "検品保留の在庫を見せて", nil, nil, "")

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

	orchestrator := usecase.NewOrchestrator(inventoryCatalogWithStatusColumn(), planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "この在庫の詳細を見せて", nil, nil, "")

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
	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "在庫の一覧を見せて", nil, nil, "")

	require.NoError(t, err)
	assert.Nil(t, result.Fields)
}

// inventoryCatalogWithChartHint is inventoryCatalog, but ListInventoryItems'
// contract declares x-ui-hint.chart, mirroring how a service would ask for
// its own answer to draw as a chart with nothing configured
// (docs/specs/dashboard.md, P2, AC-P-105).
func inventoryCatalogWithChartHint() domain.Catalog {
	catalog := inventoryCatalog()
	catalog.Endpoints[0].ChartHint = &domain.Chart{
		Category: "status", Value: "count", Kind: domain.ChartKindBar,
	}

	return catalog
}

// TestPlanResultCarriesTheEndpointsChartHint is AC-P-105's platform half:
// an endpoint whose contract declares x-ui-hint.chart draws its result as
// a chart, with the contract's own axes, and with no transform - a
// contract declares axes, never a transform.
func TestPlanResultCarriesTheEndpointsChartHint(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionCall,
		Service:     "inventory",
		OperationID: "ListInventoryItems",
	}}
	invoker := &fakeInvoker{data: map[string]any{"items": []any{}}}

	orchestrator := usecase.NewOrchestrator(inventoryCatalogWithChartHint(), planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "ステータス別の件数を見せて", nil, nil, "")

	require.NoError(t, err)
	assert.Equal(t, domain.ComponentChart, result.Component)
	require.NotNil(t, result.View)
	assert.Nil(t, result.View.Transform, "a contract declares axes, never a transform")
	require.NotNil(t, result.View.Chart)
	assert.Equal(t, "status", result.View.Chart.Category)
	assert.Equal(t, "count", result.View.Chart.Value)
	assert.Equal(t, domain.ChartKindBar, result.View.Chart.Kind)
}

// TestPlanResultWithNoChartHintHasNoView shows the common case is
// unaffected: an endpoint declaring no x-ui-hint.chart carries no View at
// all.
func TestPlanResultWithNoChartHintHasNoView(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionCall,
		Service:     "inventory",
		OperationID: "ListInventoryItems",
	}}
	invoker := &fakeInvoker{data: map[string]any{"items": []any{}}}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "在庫の一覧を見せて", nil, nil, "")

	require.NoError(t, err)
	assert.Nil(t, result.View)
}

func TestPlanNoneCallsNothingAndReturnsAMessage(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "今日の天気は？", nil, nil, "")

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

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "今日の天気は？", nil, nil, "")

	require.NoError(t, err)
	assert.Contains(t, result.Message, "何ができるの")
}

func TestPlanUnsafeCallReturnsAFormWithoutInvoking(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionCall,
		Service:     "inventory",
		OperationID: "CreateInventoryItem",
		Args:        map[string]any{"name": "widget", "status": "allocated"},
	}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "在庫を登録して", nil, nil, "")

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindForm, result.Kind)
	assert.Equal(t, "inventory", result.Service)
	assert.Equal(t, "inventory", result.ServiceDisplayName,
		"inventoryCatalog declares no info.x-ui-hint.displayName, so this falls back to the identifier")
	assert.Equal(t, "CreateInventoryItem", result.OperationID)
	assert.Equal(t, map[string]any{"name": "widget", "status": "allocated"}, result.Initial)
	assert.Equal(t, map[string]any{"type": "object", "properties": map[string]any{}}, result.Schema)
	assert.Zero(t, invoker.calls, "an unsafe call must never reach the service")
}

// TestPlanUnsafeCallFormPrefersTheServicesOwnDisplayName is
// TestPlanSafeCallPrefersTheServicesOwnDisplayName's unsafe-call half:
// formFor carries the same service display name onto the confirmation
// form an unsafe call degrades to.
func TestPlanUnsafeCallFormPrefersTheServicesOwnDisplayName(t *testing.T) {
	catalog := inventoryCatalog()
	for i := range catalog.Endpoints {
		catalog.Endpoints[i].ServiceDisplayName = "在庫管理"
	}

	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionCall,
		Service:     "inventory",
		OperationID: "CreateInventoryItem",
	}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(catalog, planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "在庫を登録して", nil, nil, "")

	require.NoError(t, err)
	assert.Equal(t, "在庫管理", result.ServiceDisplayName)
}

// catalogWithCreateStatusEnum is inventoryCatalog plus a "status" property
// on CreateInventoryItem's request body, declared as the same enum
// ListInventoryItems' query parameter carries: this is what makes
// optionsForParam able to find an enum for "status" on an unsafe
// operation, the exact situation D11 and section 8b (docs/specs/orchestration.md)
// were amended for - the ask must degrade to a form despite the enum
// being found, because CreateInventoryItem is unsafe.
func catalogWithCreateStatusEnum() domain.Catalog {
	c := inventoryCatalog()
	c.Endpoints[1].RequestBody = &domain.Schema{
		Type: domain.SchemaTypeObject,
		Properties: map[string]domain.Schema{
			"name": {Type: domain.SchemaTypeString},
			"status": {
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

// TestPlanAskDecisionForAnUnsafeOperationReturnsAFormEvenWithAnEnum drives
// the defect this task fixes: 在庫を登録したい asks CreateInventoryItem to
// be filled in, the model reaches for ask_user on "status" because it is
// the one required field it cannot decide on its own, and "status" is a
// real enum in the catalogue - so optionsForParam finds it and, before
// this fix, ask returned kind: "ask" over "ステータスを選んでください"
// instead of the create form. CreateInventoryItem is unsafe, so the
// question is beside the point: name and quantity were never asked about,
// and the form that follows would repeat the same select anyway
// (docs/specs/orchestration.md, section 8b). The fix is to degrade to a
// form before optionsForParam is ever consulted, whenever the endpoint is
// unsafe.
func TestPlanAskDecisionForAnUnsafeOperationReturnsAFormEvenWithAnEnum(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionAsk,
		Service:     "inventory",
		OperationID: "CreateInventoryItem",
		Question:    "ステータスを選んでください",
		Param:       "status",
		Args:        map[string]any{"name": "widget"},
	}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(catalogWithCreateStatusEnum(), planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "在庫を登録したい", nil, nil, "")

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindForm, result.Kind)
	assert.Equal(t, "inventory", result.Service)
	assert.Equal(t, "CreateInventoryItem", result.OperationID)
	assert.Equal(t, map[string]any{"name": "widget"}, result.Initial)

	properties, ok := result.Schema["properties"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, properties, "name")
	assert.Contains(t, properties, "status", "the whole input schema, including the enum field asked about, must reach the form")

	assert.Zero(t, invoker.calls, "an ask over an unsafe operation must never call a service")
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

	orchestrator := usecase.NewOrchestrator(catalogWithStatusEnum(), planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "検品保留の在庫を見せて", nil, nil, "")

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
	orchestrator := usecase.NewOrchestrator(c, planner, &fakeInvoker{}, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "在庫の一覧を見せて", nil, nil, "")

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

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "在庫を登録したい", nil, nil, "")

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

	orchestrator := usecase.NewOrchestrator(catalogWithStatusEnum(), planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "検品保留の在庫を見せて", nil, nil, "")

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

	orchestrator := usecase.NewOrchestrator(catalogWithStatusEnum(), planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "検品保留の在庫を見せて", nil, nil, "")

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

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker, &fakePermissionStore{})

	_, err := orchestrator.Plan(t.Context(), adminUser(), "検品保留の在庫を見せて", nil, nil, "")

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
	inventoryOrchestrator := usecase.NewOrchestrator(catalog, inventoryPlanner, invoker, &fakePermissionStore{})

	inventoryResult, err := inventoryOrchestrator.Plan(t.Context(), adminUser(), "在庫のステータスは？", nil, nil, "")
	require.NoError(t, err)
	assert.ElementsMatch(t, []domain.Option{
		{Value: "allocated", Label: "引当済"},
		{Value: "quarantined", Label: "検品保留"},
	}, inventoryResult.Options, "an ask naming the inventory operation must only offer inventory's own values")

	attendancePlanner := &fakePlanner{decision: usecase.Decision{
		Kind: usecase.DecisionAsk, Service: "attendance", OperationID: "ListAttendanceRecords",
		Question: "どのステータスですか？", Param: "status",
	}}
	attendanceOrchestrator := usecase.NewOrchestrator(catalog, attendancePlanner, invoker, &fakePermissionStore{})

	attendanceResult, err := attendanceOrchestrator.Plan(t.Context(), adminUser(), "勤怠のステータスは？", nil, nil, "")
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

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker, &fakePermissionStore{})

	_, err := orchestrator.Plan(t.Context(), adminUser(), "存在しない操作", nil, nil, "")

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrEndpointNotFound)
}

func TestPlanUnknownDecisionKindIsNotImplemented(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionKind("bogus")}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker, &fakePermissionStore{})

	_, err := orchestrator.Plan(t.Context(), adminUser(), "何か", nil, nil, "")

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrNotImplemented)
}

func TestPlanWrapsAPlannerError(t *testing.T) {
	boom := errors.New("boom")
	planner := &fakePlanner{err: boom}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker, &fakePermissionStore{})

	_, err := orchestrator.Plan(t.Context(), adminUser(), "在庫の一覧を見せて", nil, nil, "")

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

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker, &fakePermissionStore{})

	_, err := orchestrator.Plan(t.Context(), adminUser(), "在庫の一覧を見せて", nil, nil, "")

	require.Error(t, err)
	require.ErrorIs(t, err, boom)
}

func TestPlanPassesAnswersThrough(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker, &fakePermissionStore{})

	answers := []usecase.Answer{{Param: "status", Value: "allocated"}}
	_, err := orchestrator.Plan(t.Context(), adminUser(), "在庫の一覧を見せて", answers, nil, "")

	require.NoError(t, err)
	assert.Equal(t, answers, planner.answers)
}
