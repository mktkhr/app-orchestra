package usecase_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// assertDegradesToAPlainQuestion is the shared assertion for every path
// that ends in askDegrade's rule 3 (or the always-plain-question paths
// above it, in ask itself): kind ask, the question carried through
// unchanged, and neither Param nor Options set - factored out so the
// several tests that reach this same shape (an unknown endpoint, an
// undeclared param, too few model options) read as one assertion each
// rather than tripping golangci-lint's dupl (harness/quality/go/golangci.yml).
func assertDegradesToAPlainQuestion(t *testing.T, result *usecase.Result, question string) {
	t.Helper()

	assert.Equal(t, usecase.ResultKindAsk, result.Kind)
	assert.Equal(t, question, result.Question)
	assert.Empty(t, result.Param)
	assert.Empty(t, result.Options)
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

	result, err := orchestrator.Plan(t.Context(), adminUser(), "在庫を登録したい", nil, nil, "", "", nil)

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

	result, err := orchestrator.Plan(t.Context(), adminUser(), "検品保留の在庫を見せて", nil, nil, "", "", nil)

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

	result, err := orchestrator.Plan(t.Context(), adminUser(), "在庫の一覧を見せて", nil, nil, "", "", nil)

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

	result, err := orchestrator.Plan(t.Context(), adminUser(), "在庫を登録したい", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindForm, result.Kind)
	assert.Equal(t, "inventory", result.Service)
	assert.Equal(t, "CreateInventoryItem", result.OperationID)
	assert.Equal(t, map[string]any{"status": "allocated"}, result.Initial)
	assert.Equal(t, map[string]any{"type": "object", "properties": map[string]any{}}, result.Schema)
	assert.Zero(t, invoker.calls, "a degraded ask must never call a service")
}

// TestPlanAskDecisionForANameTheEndpointDoesNotDeclareReturnsAPlainQuestion
// covers the defensive case optionsForParam already guarded: a param name
// the endpoint does not declare at all (a model's mistake, not just a
// free-text field), with no model-supplied Options either. Since fixing
// TODO.md item 3 (2026-09-16), askDegrade's rule 1 only keeps today's form
// for a *required* param - "no-such-param" is not one (it is not declared
// at all, so it cannot appear in the endpoint's required list either) - and
// rule 2 needs two or more options, which this decision has none of. What
// is left is rule 3: a plain question, with no Param and no Options, rather
// than the empty form this used to render.
func TestPlanAskDecisionForANameTheEndpointDoesNotDeclareReturnsAPlainQuestion(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind: usecase.DecisionAsk, Service: "inventory", OperationID: "ListInventoryItems", Param: "no-such-param",
		Question: "どのステータスですか？",
	}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(catalogWithStatusEnum(), planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "検品保留の在庫を見せて", nil, nil, "", "", nil)

	require.NoError(t, err)
	assertDegradesToAPlainQuestion(t, &result, "どのステータスですか？")
	assert.Zero(t, invoker.calls)
}

// TestPlanAskDecisionForARequiredFreeTextParamStillReturnsAForm is rule 1 of
// askDegrade: a *required* param the catalogue does not enumerate - id-
// shaped, most often - still degrades to the form, exactly as
// TestPlanAskDecisionForAFreeTextParamReturnsAForm already covers for
// CreateInventoryItem's "name". This test exercises the same rule through a
// GET-shaped endpoint's own required parameter instead of a request body
// field, and doubles as the proof that the form's schema still describes
// the endpoint's whole argument set - ListInventoryItems has no RequestBody
// at all, only the "status" query parameter catalogWithRequiredStatus
// declares as required, and that parameter must still appear in the form's
// Schema, the gap formSchema (request-body-only) left open for a GET-shaped
// endpoint like GetInventoryItem.
func TestPlanAskDecisionForARequiredFreeTextParamStillReturnsAForm(t *testing.T) {
	c := inventoryCatalog()
	c.Endpoints[0].Parameters = []domain.Parameter{
		{Name: "status", Required: true, Schema: domain.Schema{Type: domain.SchemaTypeString}},
	}

	planner := &fakePlanner{decision: usecase.Decision{
		Kind: usecase.DecisionAsk, Service: "inventory", OperationID: "ListInventoryItems", Param: "status",
		Question: "どのステータスですか？",
	}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(c, planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "検品保留の在庫を見せて", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindForm, result.Kind)
	assert.Equal(t, "inventory", result.Service)
	assert.Equal(t, "ListInventoryItems", result.OperationID)

	properties, ok := result.Schema["properties"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, properties, "status", "a parameter, not just a request body, must reach the form's schema")

	assert.Zero(t, invoker.calls)
}

// TestPlanAskDecisionWithTwoOrMoreModelOptionsAsksUsingThem is askDegrade's
// rule 2: 「注文を見たい」-shaped questions, where the model reaches for
// ask_user about a param the catalogue does not enumerate but itself
// supplies two or more concrete candidates (受注 or 発注) - not the
// catalogue's options (there are none to have, unlike
// TestPlanAskDecisionReturnsOptionsFromTheCatalogue's enum case), but the
// model's own Decision.Options, trusted verbatim because there is nothing
// catalogue-authoritative to prefer instead. "kind" is not required, so
// rule 1 does not apply.
func TestPlanAskDecisionWithTwoOrMoreModelOptionsAsksUsingThem(t *testing.T) {
	c := inventoryCatalog()
	c.Endpoints[0].Parameters = []domain.Parameter{
		{Name: "kind", Required: false, Schema: domain.Schema{Type: domain.SchemaTypeString}},
	}

	planner := &fakePlanner{decision: usecase.Decision{
		Kind: usecase.DecisionAsk, Service: "inventory", OperationID: "ListInventoryItems", Param: "kind",
		Question: "受注ですか、発注ですか？",
		Options:  []domain.Option{{Value: "sales", Label: "受注"}, {Value: "purchase", Label: "発注"}},
	}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(c, planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "注文を見たい", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindAsk, result.Kind)
	assert.Equal(t, "受注ですか、発注ですか？", result.Question)
	assert.Equal(t, "kind", result.Param)
	assert.Equal(t, []domain.Option{{Value: "sales", Label: "受注"}, {Value: "purchase", Label: "発注"}}, result.Options)
	assert.Zero(t, invoker.calls)
}

// TestPlanAskDecisionWithTwoOrMoreModelOptionsFillsInAnEmptyLabel is the
// same rule 2 path, proving the same defensive label-falls-back-to-value
// TestPlanAskDecisionFillsInAMissingLabelDefensively already covers for a
// catalogue enum - here for the model's own Options instead
// (optionsWithLabelFallback).
func TestPlanAskDecisionWithTwoOrMoreModelOptionsFillsInAnEmptyLabel(t *testing.T) {
	c := inventoryCatalog()
	c.Endpoints[0].Parameters = []domain.Parameter{
		{Name: "kind", Required: false, Schema: domain.Schema{Type: domain.SchemaTypeString}},
	}

	planner := &fakePlanner{decision: usecase.Decision{
		Kind: usecase.DecisionAsk, Service: "inventory", OperationID: "ListInventoryItems", Param: "kind",
		Options: []domain.Option{{Value: "sales", Label: "受注"}, {Value: "purchase"}},
	}}

	orchestrator := usecase.NewOrchestrator(c, planner, &fakeInvoker{}, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "注文を見たい", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, []domain.Option{{Value: "sales", Label: "受注"}, {Value: "purchase", Label: "purchase"}}, result.Options)
}

// TestPlanAskDecisionWithOneModelOptionDegradesToAPlainQuestion is
// askDegrade's rule 3 boundary: a single model-supplied option is not a
// choice worth showing as a select (minAskOptions), so it is discarded the
// same way zero options are, rather than rule 2 offering a one-item list.
func TestPlanAskDecisionWithOneModelOptionDegradesToAPlainQuestion(t *testing.T) {
	c := inventoryCatalog()
	c.Endpoints[0].Parameters = []domain.Parameter{
		{Name: "kind", Required: false, Schema: domain.Schema{Type: domain.SchemaTypeString}},
	}

	planner := &fakePlanner{decision: usecase.Decision{
		Kind: usecase.DecisionAsk, Service: "inventory", OperationID: "ListInventoryItems", Param: "kind",
		Question: "受注ですか？",
		Options:  []domain.Option{{Value: "sales", Label: "受注"}},
	}}

	orchestrator := usecase.NewOrchestrator(c, planner, &fakeInvoker{}, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "注文を見たい", nil, nil, "", "", nil)

	require.NoError(t, err)
	assertDegradesToAPlainQuestion(t, &result, "受注ですか？")
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

	result, err := orchestrator.Plan(t.Context(), adminUser(), "検品保留の在庫を見せて", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindAsk, result.Kind)
	assert.NotEmpty(t, result.Options)
	assert.Zero(t, invoker.calls)
}

// TestPlanAskDecisionForAnUnknownEndpointDegradesToAPlainQuestion is
// defect 1 (docs/specs/shortlisting.md, measured 2026-09-15): a five-service
// catalogue tempts the model into fabricating a service/operationId ask_user
// never had ("approval/createApproval", "expense/ask_user" naming its own
// tool). An ask naming an operation the catalogue does not have - however it
// got there - is never a 500: it degrades to a plain question, with no
// param, no options and nothing invoked.
func TestPlanAskDecisionForAnUnknownEndpointDegradesToAPlainQuestion(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind: usecase.DecisionAsk, Service: "inventory", OperationID: "NoSuchOperation", Param: "status",
		Question: "どのステータス？",
	}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "検品保留の在庫を見せて", nil, nil, "", "", nil)

	require.NoError(t, err)
	assertDegradesToAPlainQuestion(t, &result, "どのステータス？")
	assert.Zero(t, invoker.calls)
}

// TestPlanAskDecisionWithNoOperationAtAllDegradesToAPlainQuestion is the
// same rule for the empty case: ask_user's schema no longer requires -
// or even offers - a "service" argument (defect 1), and a model that
// leaves operationId out entirely (or the JSON planner, which has no
// tool-calling schema to fall back on) gets the same plain question.
func TestPlanAskDecisionWithNoOperationAtAllDegradesToAPlainQuestion(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionAsk, Question: "何が知りたいですか？"}}
	invoker := &fakeInvoker{}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "何かある？", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindAsk, result.Kind)
	assert.Equal(t, "何が知りたいですか？", result.Question)
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

	inventoryResult, err := inventoryOrchestrator.Plan(t.Context(), adminUser(), "在庫のステータスは？", nil, nil, "", "", nil)
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

	attendanceResult, err := attendanceOrchestrator.Plan(t.Context(), adminUser(), "勤怠のステータスは？", nil, nil, "", "", nil)
	require.NoError(t, err)
	assert.ElementsMatch(t, []domain.Option{
		{Value: "present", Label: "出勤"},
		{Value: "absent", Label: "欠勤"},
	}, attendanceResult.Options, "an ask naming the attendance operation must only offer attendance's own values")

	assert.Zero(t, invoker.calls, "an ask decision must never call a service")
}
