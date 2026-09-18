package usecase_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// noParamEndpoint declares no parameters at all and no request body - arm
// 1's (ORCHESTRA_FILL_SKIP_EMPTY) own eligible shape.
func noParamEndpoint(method string) domain.Endpoint {
	return domain.Endpoint{
		Service: "svc-a", OperationID: "Opa", Method: method, Path: "/a",
		Response: &domain.Schema{Type: domain.SchemaTypeObject},
	}
}

// enumOnlyEndpoint declares one enum-valued parameter and no request body
// - arm 2's (ORCHESTRA_FILL_ENUM=jev) own eligible shape.
func enumOnlyEndpoint() domain.Endpoint {
	return domain.Endpoint{
		Service: "svc-b", OperationID: "Opb", Method: domain.MethodGet, Path: "/b",
		Parameters: []domain.Parameter{{
			Name: "status", In: "query",
			Schema: domain.Schema{
				Type: domain.SchemaTypeString, Enum: []string{"open", "closed"},
				EnumLabels: map[string]string{"open": "オープン", "closed": "クローズ"}, Title: "ステータス",
			},
		}},
		Response: &domain.Schema{Type: domain.SchemaTypeObject},
	}
}

// freeTextParamEndpoint declares one free-text parameter - not eligible
// for either arm.
func freeTextParamEndpoint() domain.Endpoint {
	return domain.Endpoint{
		Service: "svc-c", OperationID: "Opc", Method: domain.MethodGet, Path: "/c",
		Parameters: []domain.Parameter{{Name: "name", In: "query", Schema: domain.Schema{Type: domain.SchemaTypeString}}},
		Response:   &domain.Schema{Type: domain.SchemaTypeObject},
	}
}

// bodyEndpoint declares a request body and no parameters - not eligible
// for either arm.
func bodyEndpoint(method string) domain.Endpoint {
	return domain.Endpoint{
		Service: "svc-d", OperationID: "Opd", Method: method, Path: "/d",
		RequestBody: &domain.Schema{Type: domain.SchemaTypeObject},
		Response:    &domain.Schema{Type: domain.SchemaTypeObject},
	}
}

// mixedParamEndpoint declares one enum parameter and one free-text
// parameter - not eligible for arm 2 (not every parameter is enum-valued).
func mixedParamEndpoint() domain.Endpoint {
	return domain.Endpoint{
		Service: "svc-e", OperationID: "Ope", Method: domain.MethodGet, Path: "/e",
		Parameters: []domain.Parameter{
			{Name: "status", Schema: domain.Schema{Type: domain.SchemaTypeString, Enum: []string{"open"}}},
			{Name: "name", Schema: domain.Schema{Type: domain.SchemaTypeString}},
		},
		Response: &domain.Schema{Type: domain.SchemaTypeObject},
	}
}

func TestEligibleForFillSkip(t *testing.T) {
	safe := noParamEndpoint(domain.MethodGet)
	assert.True(t, usecase.EligibleForFillSkip(&safe), "no parameters, no body")

	unsafe := noParamEndpoint("POST")
	assert.True(t, usecase.EligibleForFillSkip(&unsafe), "eligibility does not depend on safety")

	withParam := enumOnlyEndpoint()
	assert.False(t, usecase.EligibleForFillSkip(&withParam), "a parameter disqualifies it")

	withBody := bodyEndpoint(domain.MethodGet)
	assert.False(t, usecase.EligibleForFillSkip(&withBody), "a request body disqualifies it")
}

func TestEligibleForFillEnum(t *testing.T) {
	allEnum := enumOnlyEndpoint()
	assert.True(t, usecase.EligibleForFillEnum(&allEnum), "every parameter enum-valued")

	freeText := freeTextParamEndpoint()
	assert.False(t, usecase.EligibleForFillEnum(&freeText), "a free-text parameter disqualifies it")

	mixed := mixedParamEndpoint()
	assert.False(t, usecase.EligibleForFillEnum(&mixed), "one non-enum parameter disqualifies the whole endpoint")

	withBody := bodyEndpoint(domain.MethodGet)
	assert.False(t, usecase.EligibleForFillEnum(&withBody), "a request body disqualifies it")

	noParams := noParamEndpoint(domain.MethodGet)
	assert.False(t, usecase.EligibleForFillEnum(&noParams), "no parameters at all is arm 1's own case, not arm 2's")
}

// fakeFiller is a test double for usecase.Filler: it always returns the
// fixed decision/ok/err it was built with, and records the endpoint it was
// called with.
type fakeFiller struct {
	decision usecase.Decision
	ok       bool
	err      error

	endpoint domain.Endpoint
	calls    int
}

func (f *fakeFiller) Fill(
	_ context.Context, e *domain.Endpoint, _ string, _ []usecase.Answer, _ []usecase.Turn, _ usecase.PlanContext,
) (usecase.Decision, bool, error) {
	f.endpoint = *e
	f.calls++

	return f.decision, f.ok, f.err
}

// fillCatalog holds every fixture endpoint above, so one shortlist drives
// every fill-arm test below.
func fillCatalog() domain.Catalog {
	return domain.Catalog{Endpoints: []domain.Endpoint{
		noParamEndpoint(domain.MethodGet), noParamEndpoint("POST"), enumOnlyEndpoint(), freeTextParamEndpoint(),
	}}
}

// TestFillSkipEmptySkipsThePlannerAndCallsTheSafeOperation is arm 1's own
// success path: a pick naming a no-parameter safe operation never reaches
// the planner at all, and resolves straight to the same call a model
// answering with no arguments would have produced.
func TestFillSkipEmptySkipsThePlannerAndCallsTheSafeOperation(t *testing.T) {
	picker := &fakePicker{pick: usecase.Pick{Kind: usecase.PickOperation, Service: "svc-a", OperationID: "Opa"}}
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}
	invoker := &fakeInvoker{data: map[string]any{}}

	catalog := domain.Catalog{Endpoints: []domain.Endpoint{noParamEndpoint(domain.MethodGet)}}
	orchestrator := usecase.NewOrchestrator(
		catalog, planner, invoker, &fakePermissionStore{},
		usecase.WithPicker(picker), usecase.WithStages(2), usecase.WithFillSkipEmpty(),
	)

	result, err := orchestrator.Plan(t.Context(), adminUser(), "質問", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, 0, planner.calls, "the fill's own model call is skipped entirely")
	assert.Equal(t, 1, invoker.calls)
	assert.Nil(t, invoker.args, "no arguments, the same outcome a no-argument model call would reach")
	assert.Equal(t, usecase.ResultKindResult, result.Kind)
}

// TestFillSkipEmptyOnAnUnsafeOperationFormsInsteadOfCalling is arm 1's own
// unsafe case: an unsafe no-parameter operation still gets its
// confirmation form, exactly as a model call naming it with no arguments
// would (o.call's own safe/unsafe split) - never invoked outright.
func TestFillSkipEmptyOnAnUnsafeOperationFormsInsteadOfCalling(t *testing.T) {
	picker := &fakePicker{pick: usecase.Pick{Kind: usecase.PickOperation, Service: "svc-a", OperationID: "Opa"}}
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}
	invoker := &fakeInvoker{data: map[string]any{}}

	catalog := domain.Catalog{Endpoints: []domain.Endpoint{noParamEndpoint("POST")}}
	orchestrator := usecase.NewOrchestrator(
		catalog, planner, invoker, &fakePermissionStore{},
		usecase.WithPicker(picker), usecase.WithStages(2), usecase.WithFillSkipEmpty(),
	)

	result, err := orchestrator.Plan(t.Context(), adminUser(), "質問", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, 0, planner.calls)
	assert.Equal(t, 0, invoker.calls, "an unsafe operation is never invoked outright")
	assert.Equal(t, usecase.ResultKindForm, result.Kind)
}

// TestFillSkipEmptyOffByDefaultLeavesTheFillToThePlanner is the "unset is
// byte-identical" case: with WithFillSkipEmpty never given, a picked
// no-parameter operation still goes through the planner exactly as before
// this subproject existed.
func TestFillSkipEmptyOffByDefaultLeavesTheFillToThePlanner(t *testing.T) {
	picker := &fakePicker{pick: usecase.Pick{Kind: usecase.PickOperation, Service: "svc-a", OperationID: "Opa"}}
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionCall, Service: "svc-a", OperationID: "Opa"}}
	invoker := &fakeInvoker{data: map[string]any{}}

	catalog := domain.Catalog{Endpoints: []domain.Endpoint{noParamEndpoint(domain.MethodGet)}}
	orchestrator := usecase.NewOrchestrator(
		catalog, planner, invoker, &fakePermissionStore{}, usecase.WithPicker(picker), usecase.WithStages(2),
	)

	_, err := orchestrator.Plan(t.Context(), adminUser(), "質問", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, 1, planner.calls, "unset means the fill's own model call still happens")
}

// TestFillEnumCallsWithTheAnsweredArguments is arm 2's own DecisionCall
// path: a Filler answering with arguments dispatches straight to a call
// with them, never reaching the planner.
func TestFillEnumCallsWithTheAnsweredArguments(t *testing.T) {
	picker := &fakePicker{pick: usecase.Pick{Kind: usecase.PickOperation, Service: "svc-b", OperationID: "Opb"}}
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}
	invoker := &fakeInvoker{data: map[string]any{}}
	filler := &fakeFiller{
		ok: true,
		decision: usecase.Decision{
			Kind: usecase.DecisionCall, Service: "svc-b", OperationID: "Opb", Args: map[string]any{"status": "open"},
		},
	}

	catalog := domain.Catalog{Endpoints: []domain.Endpoint{enumOnlyEndpoint()}}
	orchestrator := usecase.NewOrchestrator(
		catalog, planner, invoker, &fakePermissionStore{},
		usecase.WithPicker(picker), usecase.WithStages(2), usecase.WithFillEnum(filler),
	)

	result, err := orchestrator.Plan(t.Context(), adminUser(), "オープンのを見せて", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, 1, filler.calls)
	assert.Equal(t, 0, planner.calls)
	assert.Equal(t, map[string]any{"status": "open"}, invoker.args)
	assert.Equal(t, usecase.ResultKindResult, result.Kind)
}

// TestFillEnumMismatchAsksOverTheParameter is arm 2's own DecisionAsk
// path: a Filler naming a mismatched parameter resolves through
// Orchestrator.ask exactly as a model's own ask_user would, with the
// catalogue's own enum options and labels.
func TestFillEnumMismatchAsksOverTheParameter(t *testing.T) {
	picker := &fakePicker{pick: usecase.Pick{Kind: usecase.PickOperation, Service: "svc-b", OperationID: "Opb"}}
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}
	invoker := &fakeInvoker{data: map[string]any{}}
	filler := &fakeFiller{
		ok: true,
		decision: usecase.Decision{
			Kind: usecase.DecisionAsk, Service: "svc-b", OperationID: "Opb", Param: "status",
			Question: "ステータスはどれですか？",
		},
	}

	catalog := domain.Catalog{Endpoints: []domain.Endpoint{enumOnlyEndpoint()}}
	orchestrator := usecase.NewOrchestrator(
		catalog, planner, invoker, &fakePermissionStore{},
		usecase.WithPicker(picker), usecase.WithStages(2), usecase.WithFillEnum(filler),
	)

	result, err := orchestrator.Plan(t.Context(), adminUser(), "だめなのを見せて", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, 0, planner.calls)
	assert.Equal(t, 0, invoker.calls)
	assert.Equal(t, usecase.ResultKindAsk, result.Kind)
	assert.Equal(t, "status", result.Param)
	assert.Equal(t, "ステータスはどれですか？", result.Question)
	assert.Equal(t, []domain.Option{{Value: "open", Label: "オープン"}, {Value: "closed", Label: "クローズ"}}, result.Options)
}

// TestFillEnumFailsOpenToThePlannerOnLowConfidenceOrError is the fail-open
// case, from the orchestrator's own point of view: a Filler returning
// ok == false (any reason - missing answer, low confidence) makes
// Orchestrator fall through to the ordinary fill exactly as if o.fillEnum
// were nil, never surfacing an error to the caller.
func TestFillEnumFailsOpenToThePlannerOnLowConfidenceOrError(t *testing.T) {
	picker := &fakePicker{pick: usecase.Pick{Kind: usecase.PickOperation, Service: "svc-b", OperationID: "Opb"}}
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionCall, Service: "svc-b", OperationID: "Opb"}}
	invoker := &fakeInvoker{data: map[string]any{}}
	filler := &fakeFiller{ok: false}

	catalog := domain.Catalog{Endpoints: []domain.Endpoint{enumOnlyEndpoint()}}
	orchestrator := usecase.NewOrchestrator(
		catalog, planner, invoker, &fakePermissionStore{},
		usecase.WithPicker(picker), usecase.WithStages(2), usecase.WithFillEnum(filler),
	)

	result, err := orchestrator.Plan(t.Context(), adminUser(), "質問", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, 1, filler.calls)
	assert.Equal(t, 1, planner.calls, "fail open falls through to the ordinary fill")
	assert.Equal(t, usecase.ResultKindResult, result.Kind)
}

// TestFillEnumOffByDefaultLeavesTheFillToThePlanner is arm 2's own "unset
// is byte-identical" case: with WithFillEnum never given, an all-enum
// picked operation still goes through the planner exactly as before this
// subproject existed - fillCatalog covers every fixture shape, unused here
// beyond proving the orchestrator builds over it without a Filler.
func TestFillEnumOffByDefaultLeavesTheFillToThePlanner(t *testing.T) {
	picker := &fakePicker{pick: usecase.Pick{Kind: usecase.PickOperation, Service: "svc-b", OperationID: "Opb"}}
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionCall, Service: "svc-b", OperationID: "Opb"}}
	invoker := &fakeInvoker{data: map[string]any{}}

	orchestrator := usecase.NewOrchestrator(
		fillCatalog(), planner, invoker, &fakePermissionStore{}, usecase.WithPicker(picker), usecase.WithStages(2),
	)

	_, err := orchestrator.Plan(t.Context(), adminUser(), "質問", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, 1, planner.calls, "unset means the fill's own model call still happens")
}
