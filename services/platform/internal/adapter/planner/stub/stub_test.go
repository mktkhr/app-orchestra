package stub_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/stub"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

func TestPlanReturnsTheDecisionForAMatchingQuery(t *testing.T) {
	want := usecase.Decision{Kind: usecase.DecisionCall, Service: "inventory", OperationID: "ListInventoryItems"}
	p := stub.New(map[stub.Key]usecase.Decision{{Query: "list items"}: want}, &usecase.Decision{Kind: usecase.DecisionNone})

	got, err := p.Plan(t.Context(), "list items", nil, nil)

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestPlanReturnsNotFoundForAnUnknownQuery(t *testing.T) {
	notFound := usecase.Decision{Kind: usecase.DecisionNone}
	p := stub.New(map[stub.Key]usecase.Decision{{Query: "list items"}: {Kind: usecase.DecisionCall}}, &notFound)

	got, err := p.Plan(t.Context(), "something else entirely", nil, nil)

	require.NoError(t, err)
	assert.Equal(t, notFound, got)
}

func TestNewCopiesTheTableSoLaterMutationDoesNotLeak(t *testing.T) {
	table := map[stub.Key]usecase.Decision{{Query: "list items"}: {Kind: usecase.DecisionCall, OperationID: "A"}}
	p := stub.New(table, &usecase.Decision{Kind: usecase.DecisionNone})

	table[stub.Key{Query: "list items"}] = usecase.Decision{Kind: usecase.DecisionCall, OperationID: "B"}

	got, err := p.Plan(t.Context(), "list items", nil, nil)

	require.NoError(t, err)
	assert.Equal(t, "A", got.OperationID)
}

func TestPlanIgnoresTools(t *testing.T) {
	want := usecase.Decision{Kind: usecase.DecisionAsk, Question: "どちら？"}
	p := stub.New(map[stub.Key]usecase.Decision{{Query: "ambiguous"}: want}, &usecase.Decision{Kind: usecase.DecisionNone})

	got, err := p.Plan(
		t.Context(),
		"ambiguous",
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

	got, err := p.Plan(t.Context(), "ambiguous", nil, nil)
	require.NoError(t, err)
	assert.Equal(t, ask, got)

	got, err = p.Plan(t.Context(), "ambiguous", answers, nil)
	require.NoError(t, err)
	assert.Equal(t, call, got)
}

func TestAnswersKeyIsOrderIndependent(t *testing.T) {
	a := []usecase.Answer{{Param: "status", Value: "allocated"}, {Param: "quantity", Value: "1"}}
	b := []usecase.Answer{{Param: "quantity", Value: "1"}, {Param: "status", Value: "allocated"}}

	assert.Equal(t, stub.AnswersKey(a), stub.AnswersKey(b))
	assert.NotEmpty(t, stub.AnswersKey(a))
	assert.Empty(t, stub.AnswersKey(nil))
}
