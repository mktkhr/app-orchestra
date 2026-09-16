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

	query    string
	answers  []usecase.Answer
	turns    []usecase.Turn
	tools    []usecase.Tool
	thinking *bool
	calls    int
}

func (f *fakePlanner) Plan(
	_ context.Context, query string, answers []usecase.Answer, turns []usecase.Turn, tools []usecase.Tool,
	thinking *bool,
) (usecase.Decision, error) {
	f.query = query
	f.answers = answers
	f.turns = turns
	f.tools = tools
	f.thinking = thinking
	f.calls++

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

	result, err := orchestrator.Plan(t.Context(), adminUser(), "在庫の一覧を見せて", nil, nil, "", "", nil)

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

	result, err := orchestrator.Plan(t.Context(), adminUser(), "在庫の一覧を見せて", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, "在庫管理", result.ServiceDisplayName)
}

// TestPlanWithNoTurnsBehavesExactlyAsBefore is AC-M-105: a question with no
// turns is answered exactly as it is today, and the planner sees no turns
// at all rather than an empty-but-non-nil slice standing in for "history".
func TestPlanWithNoTurnsBehavesExactlyAsBefore(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}
	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, &fakeInvoker{}, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "在庫の一覧を見せて", nil, nil, "", "", nil)

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

	_, err := orchestrator.Plan(t.Context(), adminUser(), "4つ目", nil, turns, "", "", nil)

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

	_, err := orchestrator.Plan(t.Context(), adminUser(), "3つ目", nil, turns, "", "", nil)

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

	_, err := orchestrator.Plan(t.Context(), adminUser(), "最後の質問", nil, turns, "", "", nil)

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

	result, err := orchestrator.Plan(t.Context(), adminUser(), "検品保留の在庫を見せて", nil, nil, "", "", nil)

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

	result, err := orchestrator.Plan(t.Context(), adminUser(), "この在庫の詳細を見せて", nil, nil, "", "", nil)

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

	result, err := orchestrator.Plan(t.Context(), adminUser(), "在庫の一覧を見せて", nil, nil, "", "", nil)

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

	result, err := orchestrator.Plan(t.Context(), adminUser(), "ステータス別の件数を見せて", nil, nil, "", "", nil)

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

	result, err := orchestrator.Plan(t.Context(), adminUser(), "在庫の一覧を見せて", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Nil(t, result.View)
}

func TestPlanNoneCallsNothingAndReturnsAMessage(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "今日の天気は？", nil, nil, "", "", nil)

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

	result, err := orchestrator.Plan(t.Context(), adminUser(), "今日の天気は？", nil, nil, "", "", nil)

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

	result, err := orchestrator.Plan(t.Context(), adminUser(), "在庫を登録して", nil, nil, "", "", nil)

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

	result, err := orchestrator.Plan(t.Context(), adminUser(), "在庫を登録して", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, "在庫管理", result.ServiceDisplayName)
}

func TestPlanCallToUnknownEndpointFails(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionCall,
		Service:     "inventory",
		OperationID: "NoSuchOperation",
	}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker, &fakePermissionStore{})

	_, err := orchestrator.Plan(t.Context(), adminUser(), "存在しない操作", nil, nil, "", "", nil)

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrEndpointNotFound)
}

func TestPlanUnknownDecisionKindIsNotImplemented(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionKind("bogus")}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker, &fakePermissionStore{})

	_, err := orchestrator.Plan(t.Context(), adminUser(), "何か", nil, nil, "", "", nil)

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrNotImplemented)
}

func TestPlanWrapsAPlannerError(t *testing.T) {
	boom := errors.New("boom")
	planner := &fakePlanner{err: boom}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker, &fakePermissionStore{})

	_, err := orchestrator.Plan(t.Context(), adminUser(), "在庫の一覧を見せて", nil, nil, "", "", nil)

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

	_, err := orchestrator.Plan(t.Context(), adminUser(), "在庫の一覧を見せて", nil, nil, "", "", nil)

	require.Error(t, err)
	require.ErrorIs(t, err, boom)
}

// TestPlanTurnsA4xxServiceErrorIntoResultKindNone is the dev-stack defect
// (2026-09-16, /tmp/orchestra-platform.log 2026-09-16T20:52:15): the
// inventory service answering 404 "no item exists with this id" to a
// planner-chosen GetInventoryItem(id: att-002) must not turn /api/plan
// into a 500 - it is the service answering, not the platform failing.
func TestPlanTurnsA4xxServiceErrorIntoResultKindNone(t *testing.T) {
	catalog := inventoryCatalog()
	catalog.Endpoints[0].ServiceDisplayName = "在庫管理"
	catalog.Endpoints[0].DisplayName = "在庫アイテムの詳細"

	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionCall,
		Service:     "inventory",
		OperationID: "ListInventoryItems",
		Args:        map[string]any{"id": "att-002"},
	}}
	invoker := &fakeInvoker{err: usecase.ServiceError{Status: 404, Message: "no item exists with this id"}}

	orchestrator := usecase.NewOrchestrator(catalog, planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "att-002の内容", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindNone, result.Kind)
	assert.Equal(t, "在庫管理 の 在庫アイテムの詳細 は「no item exists with this id」と答えました。", result.Message)
	assert.Empty(t, result.Alternatives, "a none result has nothing chosen for alternativesFor to sit after")
}

// TestPlanTurnsA4xxServiceErrorWithNoMessageIntoTheBareStatus is the same
// shape, but for a service that answered 4xx with no JSON message at all
// (see the adapter's serviceMessage): the person still gets a sentence,
// just naming the status instead of quoting nothing.
func TestPlanTurnsA4xxServiceErrorWithNoMessageIntoTheBareStatus(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionCall,
		Service:     "inventory",
		OperationID: "ListInventoryItems",
	}}
	invoker := &fakeInvoker{err: usecase.ServiceError{Status: 400}}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "在庫の一覧を見せて", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindNone, result.Kind)
	assert.Equal(t, "inventory の ListInventoryItems は 400 を返しました。", result.Message)
}

// TestPlanKeepsA5xxServiceErrorAsAnError is the other half of the rule: a
// 5xx is the service's or the platform's own fault, not an answer, and
// stays exactly what TestPlanWrapsAnInvokerError already pins for a plain
// error - a ServiceError does not change that just because it is typed.
func TestPlanKeepsA5xxServiceErrorAsAnError(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionCall,
		Service:     "inventory",
		OperationID: "ListInventoryItems",
	}}
	svcErr := usecase.ServiceError{Status: 503, Message: "try again later"}
	invoker := &fakeInvoker{err: svcErr}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker, &fakePermissionStore{})

	_, err := orchestrator.Plan(t.Context(), adminUser(), "在庫の一覧を見せて", nil, nil, "", "", nil)

	require.Error(t, err)
	require.ErrorIs(t, err, svcErr)
}

func TestPlanPassesAnswersThrough(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker, &fakePermissionStore{})

	answers := []usecase.Answer{{Param: "status", Value: "allocated"}}
	_, err := orchestrator.Plan(t.Context(), adminUser(), "在庫の一覧を見せて", answers, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, answers, planner.answers)
}
