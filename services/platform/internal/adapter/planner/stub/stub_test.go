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
	p := stub.New(map[string]usecase.Decision{"list items": want}, &usecase.Decision{Kind: usecase.DecisionNone})

	got, err := p.Plan(t.Context(), "list items", nil, nil)

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestPlanReturnsNotFoundForAnUnknownQuery(t *testing.T) {
	notFound := usecase.Decision{Kind: usecase.DecisionNone}
	p := stub.New(map[string]usecase.Decision{"list items": {Kind: usecase.DecisionCall}}, &notFound)

	got, err := p.Plan(t.Context(), "something else entirely", nil, nil)

	require.NoError(t, err)
	assert.Equal(t, notFound, got)
}

func TestNewCopiesTheTableSoLaterMutationDoesNotLeak(t *testing.T) {
	table := map[string]usecase.Decision{"list items": {Kind: usecase.DecisionCall, OperationID: "A"}}
	p := stub.New(table, &usecase.Decision{Kind: usecase.DecisionNone})

	table["list items"] = usecase.Decision{Kind: usecase.DecisionCall, OperationID: "B"}

	got, err := p.Plan(t.Context(), "list items", nil, nil)

	require.NoError(t, err)
	assert.Equal(t, "A", got.OperationID)
}

func TestPlanIgnoresAnswersAndTools(t *testing.T) {
	want := usecase.Decision{Kind: usecase.DecisionAsk, Question: "どちら？"}
	p := stub.New(map[string]usecase.Decision{"ambiguous": want}, &usecase.Decision{Kind: usecase.DecisionNone})

	got, err := p.Plan(
		t.Context(),
		"ambiguous",
		[]usecase.Answer{{Param: "status", Value: "allocated"}},
		[]usecase.Tool{{Name: "ListInventoryItems"}},
	)

	require.NoError(t, err)
	assert.Equal(t, want, got)
}
