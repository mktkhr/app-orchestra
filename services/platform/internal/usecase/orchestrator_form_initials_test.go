package usecase_test

// Tests for formFor's dropInventedInitials rule (2026-09-16, TODO.md
// "invented form values", DECISIONS.md "Thirty questions against the real
// dev services"): a form's initial value for a plain free-text string
// parameter is kept only when the question (or an answer already given)
// actually mentions it. fakePlanner, fakeInvoker, fakePermissionStore and
// adminUser are defined in orchestrator_test.go, same package.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// attendanceCreateCatalog is one unsafe operation, CreateAttendanceRecord,
// with three request-body fields of three different shapes the drop rule
// tells apart: employee (plain free-text string), date (string, format
// "date"), kind (string enum). Mirrors services/attendance/api/openapi.yaml's
// real NewRecord schema, format: date included (2026-09-16).
func attendanceCreateCatalog() domain.Catalog {
	return domain.Catalog{Endpoints: []domain.Endpoint{
		{
			Service:     "attendance",
			OperationID: "CreateAttendanceRecord",
			Method:      "POST",
			Path:        "/api/attendance/records",
			Summary:     "勤怠記録を作成する",
			RequestBody: &domain.Schema{
				Type:     domain.SchemaTypeObject,
				Required: []string{"employee", "date", "kind"},
				Properties: map[string]domain.Schema{
					"employee": {Type: domain.SchemaTypeString, Title: "従業員名"},
					"date":     {Type: domain.SchemaTypeString, Format: "date", Title: "対象日"},
					"kind": {
						Type: domain.SchemaTypeString, Title: "種別",
						Enum:       []string{"deemed", "substitute"},
						EnumLabels: map[string]string{"deemed": "みなし労働", "substitute": "振替休日"},
					},
				},
			},
			Response: &domain.Schema{Type: domain.SchemaTypeObject},
		},
	}}
}

// inventoryCreateWithFieldsCatalog is one unsafe operation,
// CreateInventoryItem, whose request body actually declares "name" (plain
// free-text string) and "quantity" (integer) - unlike orchestrator_test.go's
// own inventoryCatalog, whose CreateInventoryItem declares no properties at
// all.
func inventoryCreateWithFieldsCatalog() domain.Catalog {
	return domain.Catalog{Endpoints: []domain.Endpoint{
		{
			Service:     "inventory",
			OperationID: "CreateInventoryItem",
			Method:      "POST",
			Path:        "/api/inventory/items",
			Summary:     "在庫アイテムを作成する",
			RequestBody: &domain.Schema{
				Type:     domain.SchemaTypeObject,
				Required: []string{"name", "quantity"},
				Properties: map[string]domain.Schema{
					"name":     {Type: domain.SchemaTypeString, Title: "品名"},
					"quantity": {Type: domain.SchemaTypeInteger, Title: "数量"},
				},
			},
			Response: &domain.Schema{Type: domain.SchemaTypeObject},
		},
	}}
}

// attendanceListWithEmployeeParamCatalog is one safe operation,
// ListAttendanceRecords, with a required "employee" query parameter that
// declares no enum - the shape askDegrade's paramIsRequired case degrades
// straight to a form for (orchestrator_ask.go).
func attendanceListWithEmployeeParamCatalog() domain.Catalog {
	return domain.Catalog{Endpoints: []domain.Endpoint{
		{
			Service:     "attendance",
			OperationID: "ListAttendanceRecords",
			Method:      domain.MethodGet,
			Path:        "/api/attendance/records",
			Summary:     "勤怠記録の一覧を返す",
			Parameters: []domain.Parameter{
				{
					Name: "employee", In: "query", Required: true,
					Schema: domain.Schema{Type: domain.SchemaTypeString, Title: "従業員名"},
				},
			},
			Response: &domain.Schema{
				Type: domain.SchemaTypeObject,
				Properties: map[string]domain.Schema{
					"records": {Type: domain.SchemaTypeArray, Items: &domain.Schema{Type: domain.SchemaTypeObject}},
				},
			},
		},
	}}
}

// TestFormDropsAnInventedEmployeeNameButKeepsDateAndKind is the dev-stack
// reproduction itself: 「遅刻を記録したい」 filled employee with a name
// nobody said (田中太郎), date with an invented year (fixed separately by
// toolcall/jsonmode.WithClock, but the drop rule must not also discard a
// real date just because it is a plain string type at heart), and kind
// with a guessed enum value. Only employee is dropped: date has a declared
// format, kind is an enum, and neither is a plain free-text string.
func TestFormDropsAnInventedEmployeeNameButKeepsDateAndKind(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionCall,
		Service:     "attendance",
		OperationID: "CreateAttendanceRecord",
		Args:        map[string]any{"employee": "田中太郎", "date": "2026-09-16", "kind": "deemed"},
	}}
	orchestrator := usecase.NewOrchestrator(attendanceCreateCatalog(), planner, &fakeInvoker{}, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "遅刻を記録したい", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindForm, result.Kind)
	assert.Equal(t, map[string]any{"date": "2026-09-16", "kind": "deemed"}, result.Initial,
		"employee must be dropped - the question never said 田中太郎; date and kind must survive")
}

// TestFormKeepsANameMentionedInTheQuestion is the other dev-stack case,
// which must keep working exactly as it did: 「ネジを100個入庫」 already
// filled name and quantity directly from the question, and that must not
// regress into always dropping name.
func TestFormKeepsANameMentionedInTheQuestion(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionCall,
		Service:     "inventory",
		OperationID: "CreateInventoryItem",
		Args:        map[string]any{"name": "ネジ", "quantity": 100},
	}}
	orchestrator := usecase.NewOrchestrator(
		inventoryCreateWithFieldsCatalog(), planner, &fakeInvoker{}, &fakePermissionStore{},
	)

	result, err := orchestrator.Plan(t.Context(), adminUser(), "ネジを100個入庫して", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindForm, result.Kind)
	assert.Equal(t, map[string]any{"name": "ネジ", "quantity": 100}, result.Initial)
}

// TestFormKeepsAValueAlreadyConfirmedByAnAnswer covers the other half of
// the rule: a value the question itself never mentions is still kept when
// it is exactly the value a previous ask_user answer already confirmed -
// the person did say it, just in an earlier turn of the same exchange.
func TestFormKeepsAValueAlreadyConfirmedByAnAnswer(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionCall,
		Service:     "attendance",
		OperationID: "CreateAttendanceRecord",
		Args:        map[string]any{"employee": "田中太郎", "date": "2026-09-16", "kind": "deemed"},
	}}
	orchestrator := usecase.NewOrchestrator(attendanceCreateCatalog(), planner, &fakeInvoker{}, &fakePermissionStore{})

	answers := []usecase.Answer{{Param: "employee", Value: "田中太郎"}}

	result, err := orchestrator.Plan(t.Context(), adminUser(), "遅刻を記録して", answers, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindForm, result.Kind)
	assert.Equal(t,
		map[string]any{"employee": "田中太郎", "date": "2026-09-16", "kind": "deemed"}, result.Initial,
		"employee is kept: it never appears in the question, but it does appear in a previous answer",
	)
}

// TestFormDropRuleMatchesASCIICaseInsensitively covers the rule's second
// comparison: an ASCII name typed in a different case from the question
// still counts as mentioned.
func TestFormDropRuleMatchesASCIICaseInsensitively(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionCall,
		Service:     "inventory",
		OperationID: "CreateInventoryItem",
		Args:        map[string]any{"name": "NAIL", "quantity": 10},
	}}
	orchestrator := usecase.NewOrchestrator(
		inventoryCreateWithFieldsCatalog(), planner, &fakeInvoker{}, &fakePermissionStore{},
	)

	result, err := orchestrator.Plan(t.Context(), adminUser(), "nailを10個入庫して", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, map[string]any{"name": "NAIL", "quantity": 10}, result.Initial,
		"NAIL must count as mentioned even though the question said nail, lowercase")
}

// TestAskDegradeFormDropsAnInventedValue drives the rule through
// askDegrade's own formFor call (orchestrator_ask.go): a safe operation's
// required, enum-less parameter degrades straight to a form, and that
// form is subject to the same drop rule as call's.
func TestAskDegradeFormDropsAnInventedValue(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionAsk,
		Service:     "attendance",
		OperationID: "ListAttendanceRecords",
		Question:    "誰の勤怠ですか？",
		Param:       "employee",
		Args:        map[string]any{"employee": "田中太郎"},
	}}
	orchestrator := usecase.NewOrchestrator(
		attendanceListWithEmployeeParamCatalog(), planner, &fakeInvoker{}, &fakePermissionStore{},
	)

	result, err := orchestrator.Plan(t.Context(), adminUser(), "勤怠を見せて", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindForm, result.Kind)
	assert.Nil(t, result.Initial, "田中太郎 never appears in 勤怠を見せて, so it must be dropped")
}

// TestPlanPreferredFallbackFormKeepsAnswerSuppliedValues drives the rule
// through planPreferred's own early formFor call (orchestrator_preferred.go):
// a preferred operation whose required parameters are not all known yet
// from answers alone never even reaches the planner - the fallback form's
// Initial is argsFromAnswers(answers), which the drop rule must pass
// through unchanged, since every value in it is, by construction, exactly
// what a previous answer said.
func TestPlanPreferredFallbackFormKeepsAnswerSuppliedValues(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}
	orchestrator := usecase.NewOrchestrator(attendanceCreateCatalog(), planner, &fakeInvoker{}, &fakePermissionStore{})

	answers := []usecase.Answer{{Param: "employee", Value: "田中太郎"}}

	result, err := orchestrator.Plan(
		t.Context(), adminUser(), "続けて", answers, nil, "", "CreateAttendanceRecord", nil,
	)

	require.NoError(t, err)
	assert.Zero(t, planner.calls, "date and kind are still unknown, so the model is never consulted")
	assert.Equal(t, usecase.ResultKindForm, result.Kind)
	assert.Equal(t, map[string]any{"employee": "田中太郎"}, result.Initial)
}
