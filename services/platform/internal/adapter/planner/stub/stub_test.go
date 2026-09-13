package stub_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/stub"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

func TestPlanReturnsTheDecisionForAMatchingQuery(t *testing.T) {
	want := usecase.Decision{Kind: usecase.DecisionCall, Service: "inventory", OperationID: "ListInventoryItems"}
	p := stub.New(map[stub.Key]usecase.Decision{{Query: "list items"}: want}, &usecase.Decision{Kind: usecase.DecisionNone})

	got, err := p.Plan(t.Context(), "list items", nil, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestPlanReturnsNotFoundForAnUnknownQuery(t *testing.T) {
	notFound := usecase.Decision{Kind: usecase.DecisionNone}
	p := stub.New(map[stub.Key]usecase.Decision{{Query: "list items"}: {Kind: usecase.DecisionCall}}, &notFound)

	got, err := p.Plan(t.Context(), "something else entirely", nil, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, notFound, got)
}

func TestNewCopiesTheTableSoLaterMutationDoesNotLeak(t *testing.T) {
	table := map[stub.Key]usecase.Decision{{Query: "list items"}: {Kind: usecase.DecisionCall, OperationID: "A"}}
	p := stub.New(table, &usecase.Decision{Kind: usecase.DecisionNone})

	table[stub.Key{Query: "list items"}] = usecase.Decision{Kind: usecase.DecisionCall, OperationID: "B"}

	got, err := p.Plan(t.Context(), "list items", nil, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, "A", got.OperationID)
}

// TestPlanReturnsAProposeDecisionForAMatchingQuery is AC-N-106: the stub
// planner answers a propose_panel question with whichever DecisionProposal
// its fixture names, the same table lookup every other DecisionKind
// already goes through - no model involved.
func TestPlanReturnsAProposeDecisionForAMatchingQuery(t *testing.T) {
	want := usecase.Decision{
		Kind:        usecase.DecisionProposal,
		Service:     "inventory",
		OperationID: "ListInventoryItems",
		Component:   domain.ComponentChart,
		View:        &domain.View{Chart: &domain.Chart{Category: "status", Value: "count", Kind: domain.ChartKindBar}},
		Title:       "ステータス別の在庫",
	}
	p := stub.New(map[stub.Key]usecase.Decision{
		{Query: "在庫をステータス別に棒グラフで置いて"}: want,
	}, &usecase.Decision{Kind: usecase.DecisionNone})

	got, err := p.Plan(t.Context(), "在庫をステータス別に棒グラフで置いて", nil, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestPlanIgnoresTools(t *testing.T) {
	want := usecase.Decision{Kind: usecase.DecisionAsk, Question: "どちら？"}
	p := stub.New(map[stub.Key]usecase.Decision{{Query: "ambiguous"}: want}, &usecase.Decision{Kind: usecase.DecisionNone})

	got, err := p.Plan(
		t.Context(),
		"ambiguous",
		nil,
		nil,
		[]usecase.Tool{{Name: "ListInventoryItems"}},
	)

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

// TestPlanRoutesOnAnswersToADifferentDecision drives Task 9's re-posting
// path: the same query, first with no answers and then resubmitted with
// one, must reach two different table entries - an ask the first time, the
// call it resolved to once answered.
func TestPlanRoutesOnAnswersToADifferentDecision(t *testing.T) {
	ask := usecase.Decision{Kind: usecase.DecisionAsk, Question: "どのステータスですか？", Param: "status"}
	call := usecase.Decision{
		Kind:        usecase.DecisionCall,
		Service:     "inventory",
		OperationID: "ListInventoryItems",
		Args:        map[string]any{"status": "quarantined"},
	}
	answers := []usecase.Answer{{Param: "status", Value: "quarantined"}}

	p := stub.New(map[stub.Key]usecase.Decision{
		{Query: "ambiguous"}: ask,
		{Query: "ambiguous", Answers: stub.AnswersKey(answers)}: call,
	}, &usecase.Decision{Kind: usecase.DecisionNone})

	got, err := p.Plan(t.Context(), "ambiguous", nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, ask, got)

	got, err = p.Plan(t.Context(), "ambiguous", answers, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, call, got)
}

// TestPlanRoutesOnTurnsToADifferentDecision drives Task 4's fixture need: a
// follow-up question phrased with no service name is the same Query either
// way, and only the conversation before it says which service it means.
func TestPlanRoutesOnTurnsToADifferentDecision(t *testing.T) {
	fromInventory := []usecase.Turn{
		{Question: "在庫の一覧を見せて", Kind: usecase.ResultKindResult, Service: "inventory", OperationID: "ListInventoryItems"},
	}
	fromAttendance := []usecase.Turn{
		{Question: "出勤記録を見せて", Kind: usecase.ResultKindResult, Service: "attendance", OperationID: "ListAttendanceRecords"},
	}
	wantInventory := usecase.Decision{Kind: usecase.DecisionCall, Service: "inventory", OperationID: "ListInventoryItems"}
	wantAttendance := usecase.Decision{Kind: usecase.DecisionCall, Service: "attendance", OperationID: "ListAttendanceRecords"}

	p := stub.New(map[stub.Key]usecase.Decision{
		{Query: "検品保留のものは？", Turns: stub.TurnsKey(fromInventory)}:  wantInventory,
		{Query: "検品保留のものは？", Turns: stub.TurnsKey(fromAttendance)}: wantAttendance,
	}, &usecase.Decision{Kind: usecase.DecisionNone})

	got, err := p.Plan(t.Context(), "検品保留のものは？", nil, fromInventory, nil)
	require.NoError(t, err)
	assert.Equal(t, wantInventory, got)

	got, err = p.Plan(t.Context(), "検品保留のものは？", nil, fromAttendance, nil)
	require.NoError(t, err)
	assert.Equal(t, wantAttendance, got)
}

func TestTurnsKeyIsOrderDependentAndEmptyForNoTurns(t *testing.T) {
	a := []usecase.Turn{
		{Service: "inventory", OperationID: "ListInventoryItems"},
		{Service: "attendance", OperationID: "ListAttendanceRecords"},
	}
	b := []usecase.Turn{
		{Service: "attendance", OperationID: "ListAttendanceRecords"},
		{Service: "inventory", OperationID: "ListInventoryItems"},
	}

	assert.NotEqual(t, stub.TurnsKey(a), stub.TurnsKey(b))
	assert.NotEmpty(t, stub.TurnsKey(a))
	assert.Empty(t, stub.TurnsKey(nil))
}

func TestAnswersKeyIsOrderIndependent(t *testing.T) {
	a := []usecase.Answer{{Param: "status", Value: "allocated"}, {Param: "quantity", Value: "1"}}
	b := []usecase.Answer{{Param: "quantity", Value: "1"}, {Param: "status", Value: "allocated"}}

	assert.Equal(t, stub.AnswersKey(a), stub.AnswersKey(b))
	assert.NotEmpty(t, stub.AnswersKey(a))
	assert.Empty(t, stub.AnswersKey(nil))
}
