package usecase_test

// Tests for the enum-guess guard (orchestrator_enum_guess.go), restoring
// AC-B-105 (PRODUCT.md) under two-stage planning: 「破損した在庫はある？」
// must ask about status rather than silently calling with a guessed
// quarantined, and a question that does name a status (even by a run of
// its label, not the whole word) must keep working exactly as it did.
// fakePlanner, fakeInvoker, fakePermissionStore and adminUser are defined
// in orchestrator_test.go, same package.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// enumGuessCatalog carries the two real enums the guard restores AC-B-105
// for, as query parameters on safe list operations (the shape a fill
// actually produces a guessed value for): inventory's status
// (services/inventory/api/openapi.yaml's ItemStatus) and attendance's kind
// (services/attendance/api/openapi.yaml's RecordKind), each with its own
// title and the real Japanese labels.
func enumGuessCatalog() domain.Catalog {
	return domain.Catalog{Endpoints: []domain.Endpoint{
		{
			Service:     "inventory",
			OperationID: "ListInventoryItems",
			Method:      domain.MethodGet,
			Path:        "/api/inventory/items",
			Response:    &domain.Schema{Type: domain.SchemaTypeObject},
			Parameters: []domain.Parameter{{
				Name: "status", In: "query",
				Schema: domain.Schema{
					Type: domain.SchemaTypeString, Title: "ステータス",
					Enum: []string{"allocated", "staged", "quarantined", "consigned"},
					EnumLabels: map[string]string{
						"allocated":   "引当済",
						"staged":      "出荷準備完了",
						"quarantined": "検品保留",
						"consigned":   "預託在庫",
					},
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
				Name: "kind", In: "query",
				Schema: domain.Schema{
					Type: domain.SchemaTypeString, Title: "種別",
					Enum: []string{"deemed", "substitute", "compensatory", "on_call"},
					EnumLabels: map[string]string{
						"deemed":       "みなし労働",
						"substitute":   "振替休日",
						"compensatory": "代休",
						"on_call":      "待機",
					},
				},
			}},
		},
	}}
}

// TestEnumGuessGuard is the six cases DECISIONS.md's 2026-09-17 restoration
// measured by hand: two guessed values (the fill filled a status/kind
// nobody named) that must now ask instead of silently calling, and four
// values a question does name - one by the value itself and three by a
// run of the label short of the whole word - that must keep calling
// exactly as before the guard existed.
func TestEnumGuessGuard(t *testing.T) {
	tests := []struct {
		name     string
		question string
		service  string
		op       string
		param    string
		value    string
	}{
		{"guess-status", "破損した在庫はある？", "inventory", "ListInventoryItems", "status", "quarantined"},
		{"guess-kind", "有給の勤怠はある？", "attendance", "ListAttendanceRecords", "kind", "compensatory"},
		{"named-staged", "出荷準備できてる在庫は？", "inventory", "ListInventoryItems", "status", "staged"},
		{"named-allocated", "引当済みの在庫だけ出して", "inventory", "ListInventoryItems", "status", "allocated"},
		{"named-consigned", "預託在庫ある？", "inventory", "ListInventoryItems", "status", "consigned"},
		{"named-quarantined", "検品中のやつ", "inventory", "ListInventoryItems", "status", "quarantined"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			planner := &fakePlanner{decision: usecase.Decision{
				Kind:        usecase.DecisionCall,
				Service:     tc.service,
				OperationID: tc.op,
				Args:        map[string]any{tc.param: tc.value},
			}}
			invoker := &fakeInvoker{data: map[string]any{"items": []any{}}}
			orchestrator := usecase.NewOrchestrator(enumGuessCatalog(), planner, invoker, &fakePermissionStore{})

			result, err := orchestrator.Plan(t.Context(), adminUser(), tc.question, nil, nil, "", "", nil)

			require.NoError(t, err)

			if tc.name == "guess-status" || tc.name == "guess-kind" {
				assert.Equal(t, usecase.ResultKindAsk, result.Kind, "a guessed value must be asked about, not called with")
				assert.Equal(t, tc.param, result.Param)
				assert.NotEmpty(t, result.Options, "the ask must offer the catalogue's own options")
				assert.Zero(t, invoker.calls, "a guess must never reach the service")

				return
			}

			assert.Equal(t, usecase.ResultKindResult, result.Kind, "a value the question names must still be called")
			assert.Equal(t, 1, invoker.calls)
			assert.Equal(t, map[string]any{tc.param: tc.value}, invoker.args)
		})
	}
}

// TestEnumGuessGuardDropsAGuessFromAnUnsafeCallsForm covers the unsafe
// half of D8: an unsafe operation's own enum query parameter (Parameters,
// the same scope guessedEnumArgs searches for a safe list) is guessed
// (the question never named 検品保留 or quarantined), so it must be
// dropped from the form's initial values, while name - a plain string the
// question does say - survives dropInventedInitials's own separate rule.
func TestEnumGuessGuardDropsAGuessFromAnUnsafeCallsForm(t *testing.T) {
	statusParam := domain.Parameter{
		Name: "status", In: "query",
		Schema: domain.Schema{
			Type: domain.SchemaTypeString, Title: "ステータス",
			Enum:       []string{"allocated", "staged", "quarantined", "consigned"},
			EnumLabels: map[string]string{"allocated": "引当済", "quarantined": "検品保留"},
		},
	}
	catalog := domain.Catalog{Endpoints: []domain.Endpoint{{
		Service:     "inventory",
		OperationID: "UpdateInventoryItemsBulk",
		Method:      "PATCH",
		Path:        "/api/inventory/items",
		Parameters:  []domain.Parameter{{Name: "name", In: "query", Schema: domain.Schema{Type: domain.SchemaTypeString}}, statusParam},
		RequestBody: &domain.Schema{Type: domain.SchemaTypeObject},
		Response:    &domain.Schema{Type: domain.SchemaTypeObject},
	}}}

	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionCall,
		Service:     "inventory",
		OperationID: "UpdateInventoryItemsBulk",
		Args:        map[string]any{"name": "テスト品", "status": "quarantined"},
	}}
	invoker := &fakeInvoker{}
	orchestrator := usecase.NewOrchestrator(catalog, planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "テスト品をまとめて更新して", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindForm, result.Kind)
	assert.Equal(t, map[string]any{"name": "テスト品"}, result.Initial,
		"status must be dropped - the question never named 検品保留 or quarantined - but name survives")
	assert.Zero(t, invoker.calls)
}

// TestEnumGuessGuardPassesAValueLiteralInTheQuestion covers the value
// itself, not just its label, appearing in the question - mentionedInQuestion's
// first comparison.
func TestEnumGuessGuardPassesAValueLiteralInTheQuestion(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionCall,
		Service:     "inventory",
		OperationID: "ListInventoryItems",
		Args:        map[string]any{"status": "quarantined"},
	}}
	invoker := &fakeInvoker{data: map[string]any{"items": []any{}}}
	orchestrator := usecase.NewOrchestrator(enumGuessCatalog(), planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "quarantined のアイテムを見せて", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindResult, result.Kind)
	assert.Equal(t, 1, invoker.calls)
}

// TestEnumGuessGuardPassesAValueConfirmedByAnAnswer covers a value that
// never appears in the question at all, but was already confirmed by a
// previous ask_user answer in the same conversation.
func TestEnumGuessGuardPassesAValueConfirmedByAnAnswer(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionCall,
		Service:     "inventory",
		OperationID: "ListInventoryItems",
		Args:        map[string]any{"status": "quarantined"},
	}}
	invoker := &fakeInvoker{data: map[string]any{"items": []any{}}}
	orchestrator := usecase.NewOrchestrator(enumGuessCatalog(), planner, invoker, &fakePermissionStore{})

	answers := []usecase.Answer{{Param: "status", Value: "quarantined"}}

	result, err := orchestrator.Plan(t.Context(), adminUser(), "それの在庫は？", answers, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindResult, result.Kind)
	assert.Equal(t, 1, invoker.calls)
}

// TestEnumGuessGuardLeavesANonEnumParameterUntouched covers a parameter
// the guard has no business looking at: no Enum declared, so any
// value - named in the question or not - passes through exactly as it did
// before this guard existed.
func TestEnumGuessGuardLeavesANonEnumParameterUntouched(t *testing.T) {
	catalog := domain.Catalog{Endpoints: []domain.Endpoint{{
		Service:     "inventory",
		OperationID: "ListInventoryItems",
		Method:      domain.MethodGet,
		Path:        "/api/inventory/items",
		Response:    &domain.Schema{Type: domain.SchemaTypeObject},
		Parameters: []domain.Parameter{{
			Name: "name", In: "query",
			Schema: domain.Schema{Type: domain.SchemaTypeString, Title: "品名"},
		}},
	}}}

	planner := &fakePlanner{decision: usecase.Decision{
		Kind:        usecase.DecisionCall,
		Service:     "inventory",
		OperationID: "ListInventoryItems",
		Args:        map[string]any{"name": "何かのアイテム"},
	}}
	invoker := &fakeInvoker{data: map[string]any{"items": []any{}}}
	orchestrator := usecase.NewOrchestrator(catalog, planner, invoker, &fakePermissionStore{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "アイテムを見せて", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindResult, result.Kind, "no enum on name - the guard must never touch it")
	assert.Equal(t, 1, invoker.calls)
	assert.Equal(t, map[string]any{"name": "何かのアイテム"}, invoker.args)
}
