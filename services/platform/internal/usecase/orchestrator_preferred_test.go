package usecase_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// getAttendanceRecordCatalog stands in for the reproduction in
// orchestrator.go's planPreferred doc comment: one safe operation whose
// path parameter "id" is required, and that the question alone
// ("勤怠記録を見せて") never carries.
func getAttendanceRecordCatalog() domain.Catalog {
	return domain.Catalog{Endpoints: []domain.Endpoint{{
		Service:     "attendance",
		OperationID: "GetAttendanceRecord",
		Method:      domain.MethodGet,
		Path:        "/api/attendance/records/{id}",
		Summary:     "Get one attendance record by id.",
		DisplayName: "勤怠記録の詳細",
		Parameters: []domain.Parameter{
			{Name: "id", In: "path", Required: true, Schema: domain.Schema{Type: domain.SchemaTypeString}},
		},
		Response: &domain.Schema{Type: domain.SchemaTypeObject},
	}}}
}

// TestPlanWithPreferredOnRequiredParameterSkipsThePlannerAndFormsInstead is
// the reproduction's fix: GetAttendanceRecord needs an id the question does
// not carry, so Plan never asks the model at all - it hands back a form for
// the preferred operation directly, the same way D8 already does for an
// unsafe call.
func TestPlanWithPreferredOnRequiredParameterSkipsThePlannerAndFormsInstead(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}

	orchestrator := usecase.NewOrchestrator(
		getAttendanceRecordCatalog(), planner, &fakeInvoker{}, &fakePermissionStore{},
	)

	result, err := orchestrator.Plan(t.Context(), adminUser(), "勤怠記録の詳細", nil, nil, "", "GetAttendanceRecord", nil)

	require.NoError(t, err)
	assert.Equal(t, 0, planner.calls, "the planner must never be called when a required parameter is unanswered")
	assert.Equal(t, usecase.ResultKindForm, result.Kind)
	assert.Equal(t, "attendance", result.Service)
	assert.Equal(t, "GetAttendanceRecord", result.OperationID)

	required, ok := result.Schema["required"].([]string)
	require.True(t, ok, "the form's schema must declare its required fields")
	assert.Contains(t, required, "id")
}

// TestPlanWithPreferredOnRequiredParameterPrefillsFromAnswers is the same
// path with the id already supplied as an answer (a person resubmitting
// after a previous ask, or having typed it into a form once already): the
// planner is consulted - id is already known - and a call it returns for
// the preferred operation is invoked exactly as usual.
func TestPlanWithPreferredOnRequiredParameterPrefillsFromAnswers(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind: usecase.DecisionCall, Service: "attendance", OperationID: "GetAttendanceRecord",
		Args: map[string]any{"id": "rec-1"},
	}}

	orchestrator := usecase.NewOrchestrator(
		getAttendanceRecordCatalog(), planner, &fakeInvoker{data: map[string]any{}}, &fakePermissionStore{},
	)

	answers := []usecase.Answer{{Param: "id", Value: "rec-1"}}

	result, err := orchestrator.Plan(t.Context(), adminUser(), "勤怠記録の詳細", answers, nil, "", "GetAttendanceRecord", nil)

	require.NoError(t, err)
	assert.Equal(t, 1, planner.calls, "id is already known, so the planner fills in whatever else remains")
	assert.Equal(t, usecase.ResultKindResult, result.Kind)
	assert.Equal(t, "GetAttendanceRecord", result.OperationID)
}

// TestPlanWithPreferredAndPlannerEscapesDegradesToForm covers the
// reproduction directly: with a required parameter already known (so the
// planner is consulted), a planner that reaches for list_capabilities,
// ask_user (DecisionAsk) or gives up (DecisionNone) instead of calling the
// preferred operation must never have its own answer shown - the person
// chose this operation, so what comes back is always a form for it.
func TestPlanWithPreferredAndPlannerEscapesDegradesToForm(t *testing.T) {
	tests := map[string]usecase.Decision{
		"list_capabilities": {Kind: usecase.DecisionListCapabilities},
		"none":              {Kind: usecase.DecisionNone},
		"ask":               {Kind: usecase.DecisionAsk, Question: "どの記録ですか？"},
		"call to a different operation": {
			Kind: usecase.DecisionCall, Service: "attendance", OperationID: "ListAttendanceRecords",
		},
	}

	for name, decision := range tests {
		t.Run(name, func(t *testing.T) {
			planner := &fakePlanner{decision: decision}

			orchestrator := usecase.NewOrchestrator(
				getAttendanceRecordCatalog(), planner, &fakeInvoker{}, &fakePermissionStore{},
			)

			answers := []usecase.Answer{{Param: "id", Value: "rec-1"}}

			result, err := orchestrator.Plan(t.Context(), adminUser(), "勤怠記録の詳細", answers, nil, "", "GetAttendanceRecord", nil)

			require.NoError(t, err)
			assert.Equal(t, 1, planner.calls)
			assert.Equal(t, usecase.ResultKindForm, result.Kind, "the planner's own answer must never be returned under preferred")
			assert.Equal(t, "GetAttendanceRecord", result.OperationID)
			assert.Equal(t, map[string]any{"id": "rec-1"}, result.Initial)
		})
	}
}

// TestPlanWithPreferredOnParameterlessOperationCallsThePlanner is the
// companion path: an operation with no required parameter at all is
// planned exactly as before - the model is offered the one tool and its
// call is invoked - only the built-ins are missing from what it was
// offered (see TestPlanWithPreferredBypassesTheNarrowerAndOffersOnlyThatOperation
// in orchestrator_alternatives_test.go for that assertion).
func TestPlanWithPreferredOnParameterlessOperationCallsThePlanner(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{
		Kind: usecase.DecisionCall, Service: "svc-c", OperationID: "Opc",
	}}

	orchestrator := usecase.NewOrchestrator(
		fourEndpointShortlist(), planner, &fakeInvoker{data: map[string]any{}}, &fakePermissionStore{},
	)

	result, err := orchestrator.Plan(t.Context(), adminUser(), "質問", nil, nil, "", "Opc", nil)

	require.NoError(t, err)
	assert.Equal(t, 1, planner.calls)
	assert.Equal(t, usecase.ResultKindResult, result.Kind)
	assert.Equal(t, "Opc", result.OperationID)
}
