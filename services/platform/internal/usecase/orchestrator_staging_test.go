package usecase_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// withCapturedDefaultLogger swaps slog.Default() for a JSON handler
// writing into the returned buffer, restoring the previous default at
// cleanup - the same pattern
// internal/infra/httpserver/logging_internal_test.go uses for asserting a
// log line's own fields, chosen here (over threading a logger field onto
// Orchestrator) because planStaged already logs through slog.Default(),
// same as toolcall.Planner's own truncation warn.
func withCapturedDefaultLogger(t *testing.T) *bytes.Buffer {
	t.Helper()

	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })

	var buf bytes.Buffer

	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))

	return &buf
}

// fakePicker is a test double for usecase.Picker: it always returns the
// fixed pick (or error) it was built with, and records the query and
// catalogue it was called with.
type fakePicker struct {
	pick usecase.Pick
	err  error

	query   string
	catalog domain.Catalog
	calls   int
}

func (f *fakePicker) Pick(_ context.Context, query string, catalog domain.Catalog) (usecase.Pick, error) {
	f.query = query
	f.catalog = catalog
	f.calls++

	return f.pick, f.err
}

// stagingShortlist is four endpoints, one of them (Opb) declaring an enum
// parameter and one (Opc) a required parameter with no default at all -
// so a single fixture can drive every staged-path test below.
func stagingShortlist() domain.Catalog {
	return domain.Catalog{Endpoints: []domain.Endpoint{
		{
			Service: "svc-a", OperationID: "Opa", Method: domain.MethodGet, Path: "/a", Summary: "operation a",
			DisplayName: "操作a", Response: &domain.Schema{Type: domain.SchemaTypeObject},
		},
		{
			Service: "svc-b", OperationID: "Opb", Method: domain.MethodGet, Path: "/b", Summary: "operation b",
			DisplayName: "操作b",
			Parameters: []domain.Parameter{{
				Name:   "status",
				Schema: domain.Schema{Type: domain.SchemaTypeString, Enum: []string{"open", "closed"}},
			}},
			Response: &domain.Schema{Type: domain.SchemaTypeObject},
		},
		{
			Service: "svc-c", OperationID: "Opc", Method: domain.MethodGet, Path: "/c/{id}", Summary: "operation c",
			DisplayName: "操作c",
			Parameters: []domain.Parameter{
				{Name: "id", In: "path", Required: true, Schema: domain.Schema{Type: domain.SchemaTypeString}},
			},
			Response: &domain.Schema{Type: domain.SchemaTypeObject},
		},
		{
			Service: "svc-d", OperationID: "Opd", Method: domain.MethodGet, Path: "/d", Summary: "operation d",
			DisplayName: "操作d", Response: &domain.Schema{Type: domain.SchemaTypeObject},
		},
	}}
}

func stagedOrchestrator(t *testing.T, picker usecase.Picker, planner usecase.Planner, invoker usecase.Invoker) *usecase.Orchestrator {
	t.Helper()

	shortlist := stagingShortlist()
	narrower := &fakeNarrower{catalog: shortlist}

	return usecase.NewOrchestrator(
		shortlist, planner, invoker, &fakePermissionStore{},
		usecase.WithNarrower(narrower, 20), usecase.WithPicker(picker), usecase.WithStages(2),
	)
}

// TestPlanStagedOnPickOperationCallsThePlannerWithOnlyThatOperationsTool is
// the core dispatch: a pick naming an operation runs the fill with exactly
// that operation's tool (no ask_user, since Opa declares no enum), and the
// result carries the shortlist's own next two entries as Alternatives.
func TestPlanStagedOnPickOperationCallsThePlannerWithOnlyThatOperationsTool(t *testing.T) {
	picker := &fakePicker{pick: usecase.Pick{Kind: usecase.PickOperation, Service: "svc-a", OperationID: "Opa"}}
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionCall, Service: "svc-a", OperationID: "Opa"}}
	invoker := &fakeInvoker{data: map[string]any{}}

	orchestrator := stagedOrchestrator(t, picker, planner, invoker)

	result, err := orchestrator.Plan(t.Context(), adminUser(), "質問", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, 1, picker.calls)
	assert.Equal(t, 1, planner.calls)
	require.Len(t, planner.tools, 1, "only the picked operation's tool must be offered")
	assert.Equal(t, "Opa", planner.tools[0].Name)
	assert.Equal(t, usecase.ResultKindResult, result.Kind)
	assert.Equal(t, "Opa", result.OperationID)
	assert.Equal(t, []usecase.Alternative{
		{OperationID: "Opb", DisplayName: "操作b", Service: "svc-b"},
		{OperationID: "Opc", DisplayName: "操作c", Service: "svc-c"},
	}, result.Alternatives)
}

// TestPlanStagedOnPickOperationWithEnumOffersAskUser is S5: the picked
// operation's enum parameter earns ask_user alongside its own tool.
func TestPlanStagedOnPickOperationWithEnumOffersAskUser(t *testing.T) {
	picker := &fakePicker{pick: usecase.Pick{Kind: usecase.PickOperation, Service: "svc-b", OperationID: "Opb"}}
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionCall, Service: "svc-b", OperationID: "Opb"}}
	invoker := &fakeInvoker{data: map[string]any{}}

	orchestrator := stagedOrchestrator(t, picker, planner, invoker)

	_, err := orchestrator.Plan(t.Context(), adminUser(), "質問", nil, nil, "", "", nil)

	require.NoError(t, err)
	require.Len(t, planner.tools, 2)

	names := []string{planner.tools[0].Name, planner.tools[1].Name}
	assert.Contains(t, names, "Opb")
	assert.Contains(t, names, "ask_user")
}

// TestPlanStagedOnPickOperationWithRequiredParameterFormsWithoutCallingThePlanner
// is planPreferred's own required-parameter short circuit, reached through
// the pick: Opc's required "id" is not in answers, so the planner is never
// consulted at all.
func TestPlanStagedOnPickOperationWithRequiredParameterFormsWithoutCallingThePlanner(t *testing.T) {
	picker := &fakePicker{pick: usecase.Pick{Kind: usecase.PickOperation, Service: "svc-c", OperationID: "Opc"}}
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}

	orchestrator := stagedOrchestrator(t, picker, planner, &fakeInvoker{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "質問", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, 0, planner.calls, "the planner must never be called when a required parameter is unanswered")
	assert.Equal(t, usecase.ResultKindForm, result.Kind)
	assert.Equal(t, "Opc", result.OperationID)
}

// TestPlanStagedOnPickListCapabilitiesNeverCallsThePlanner is S3's
// list_capabilities line: the same Result listCapabilities produces today,
// with no second model call.
func TestPlanStagedOnPickListCapabilitiesNeverCallsThePlanner(t *testing.T) {
	picker := &fakePicker{pick: usecase.Pick{Kind: usecase.PickListCapabilities}}
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}

	orchestrator := stagedOrchestrator(t, picker, planner, &fakeInvoker{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "何ができるの？", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, 0, planner.calls)
	assert.Equal(t, usecase.ResultKindResult, result.Kind)
	assert.Equal(t, "list_capabilities", result.OperationID)
}

// TestPlanStagedOnPickNoneNeverCallsThePlanner is S3's none line.
func TestPlanStagedOnPickNoneNeverCallsThePlanner(t *testing.T) {
	picker := &fakePicker{pick: usecase.Pick{Kind: usecase.PickNone}}
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionCall, Service: "svc-a", OperationID: "Opa"}}

	orchestrator := stagedOrchestrator(t, picker, planner, &fakeInvoker{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "今日の天気は？", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, 0, planner.calls)
	assert.Equal(t, usecase.ResultKindNone, result.Kind)
}

// TestPlanStagedOnPickProposePanelFallsBackToTheSingleCallOverTheWhole
// ShortlistIsSection3sFallback: propose_panel is the one built-in with no
// pick-shaped format of its own, so it falls back to today's single call
// over the whole shortlist - the planner is offered every endpoint plus
// the built-ins, and whatever it returns is the result.
func TestPlanStagedOnPickProposePanelFallsBackToTheSingleCallOverTheWholeShortlist(t *testing.T) {
	picker := &fakePicker{pick: usecase.Pick{Kind: usecase.PickProposePanel}}
	planner := &fakePlanner{decision: usecase.Decision{
		Kind: usecase.DecisionProposal, Service: "svc-a", OperationID: "Opa",
	}}

	orchestrator := stagedOrchestrator(t, picker, planner, &fakeInvoker{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "画面に置いて", nil, nil, "ws-1", "", nil)

	require.NoError(t, err)
	assert.Equal(t, 1, planner.calls)
	assert.Len(t, planner.tools, len(stagingShortlist().Endpoints)+3, "the whole shortlist's tools plus the three built-ins")
	assert.Equal(t, usecase.ResultKindProposal, result.Kind)
}

// TestPlanStagedFromPickHonoursDecisionAskAsAKindAsk is planPreferred's
// fromPick addition: a DecisionAsk naming the picked operation's own enum
// parameter must reach the person as kind: ask, with options, not degrade
// to a form.
func TestPlanStagedFromPickHonoursDecisionAskAsAKindAsk(t *testing.T) {
	picker := &fakePicker{pick: usecase.Pick{Kind: usecase.PickOperation, Service: "svc-b", OperationID: "Opb"}}
	planner := &fakePlanner{decision: usecase.Decision{
		Kind: usecase.DecisionAsk, Service: "svc-b", OperationID: "Opb", Question: "どのステータス？", Param: "status",
	}}

	orchestrator := stagedOrchestrator(t, picker, planner, &fakeInvoker{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "オープンでないほうを見せて", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindAsk, result.Kind)
	assert.Equal(t, "どのステータス？", result.Question)
	assert.Equal(t, "status", result.Param)
	assert.ElementsMatch(t, []domain.Option{{Value: "open"}, {Value: "closed"}}, result.Options)
}

// TestPlanWithPreferredNeverCallsThePickerEvenUnderStages2 is S1's "a
// preferred in the request bypasses the pick, as it bypasses narrowing".
func TestPlanWithPreferredNeverCallsThePickerEvenUnderStages2(t *testing.T) {
	picker := &fakePicker{pick: usecase.Pick{Kind: usecase.PickOperation, Service: "svc-a", OperationID: "Opa"}}
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionCall, Service: "svc-d", OperationID: "Opd"}}

	orchestrator := stagedOrchestrator(t, picker, planner, &fakeInvoker{data: map[string]any{}})

	_, err := orchestrator.Plan(t.Context(), adminUser(), "質問", nil, nil, "", "Opd", nil)

	require.NoError(t, err)
	assert.Equal(t, 0, picker.calls, "preferred must bypass the picker entirely")
}

// TestPlanWithStages1NeverCallsThePicker proves the default (and an
// explicit 1) never consults a configured picker at all - only ==2 does.
func TestPlanWithStages1NeverCallsThePicker(t *testing.T) {
	picker := &fakePicker{pick: usecase.Pick{Kind: usecase.PickNone}}
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}

	shortlist := stagingShortlist()
	narrower := &fakeNarrower{catalog: shortlist}

	orchestrator := usecase.NewOrchestrator(
		shortlist, planner, &fakeInvoker{}, &fakePermissionStore{},
		usecase.WithNarrower(narrower, 20), usecase.WithPicker(picker), usecase.WithStages(1),
	)

	_, err := orchestrator.Plan(t.Context(), adminUser(), "質問", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, 0, picker.calls)
	assert.Equal(t, 1, planner.calls)
}

// TestPlanWithNoPickerConfiguredNeverCallsOne is the same proof for every
// existing test in this package: NewOrchestrator's own default (stages 1,
// no picker) behaves exactly as it always has.
func TestPlanWithNoPickerConfiguredNeverCallsOne(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}

	orchestrator := usecase.NewOrchestrator(inventoryCatalog(), planner, &fakeInvoker{}, &fakePermissionStore{})

	_, err := orchestrator.Plan(t.Context(), adminUser(), "在庫の一覧を見せて", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, 1, planner.calls)
}

// TestPlanWithStages2AndNoPickerIsErrStagesRequirePicker is WithStages's
// own documented deviation from docs/plans/staging.md's Task 2: forbidigo
// (harness/quality/go/golangci.yml) forbids panic outright, and changing
// NewOrchestrator's signature to return an error would ripple through its
// ~70 existing call sites - so this misconfiguration is checked at the top
// of every Plan call instead of at construction. NewOrchestrator itself
// never fails.
func TestPlanWithStages2AndNoPickerIsErrStagesRequirePicker(t *testing.T) {
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}

	orchestrator := usecase.NewOrchestrator(
		inventoryCatalog(), planner, &fakeInvoker{}, &fakePermissionStore{}, usecase.WithStages(2),
	)

	_, err := orchestrator.Plan(t.Context(), adminUser(), "質問", nil, nil, "", "", nil)

	require.ErrorIs(t, err, usecase.ErrStagesRequirePicker)
	assert.Equal(t, 0, planner.calls, "the planner must never be reached with a misconfigured Orchestrator")
}

// TestPlanStagedWithUnknownPickedOperationIsErrEndpointNotFound is the
// defensive lookup in planPicked: a pick naming an id/service pair not
// actually present in the shortlist it was given - which should not
// happen, since candidatesFor is built from that same shortlist, but is
// not ruled out by the Picker interface - fails loudly rather than
// silently falling back to something else.
func TestPlanStagedWithUnknownPickedOperationIsErrEndpointNotFound(t *testing.T) {
	picker := &fakePicker{pick: usecase.Pick{Kind: usecase.PickOperation, Service: "svc-x", OperationID: "NoSuchOp"}}
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}

	orchestrator := stagedOrchestrator(t, picker, planner, &fakeInvoker{})

	_, err := orchestrator.Plan(t.Context(), adminUser(), "質問", nil, nil, "", "", nil)

	require.ErrorIs(t, err, usecase.ErrEndpointNotFound)
}

// TestPlanStagedLogsThePicksOperationIdAndAmbiguousAtInfo is S4: the
// pick's outcome is logged, not acted on - checked through a captured
// default logger (withCapturedDefaultLogger) rather than a mock, since
// planStaged logs through slog.Default() directly.
func TestPlanStagedLogsThePicksOperationIdAndAmbiguousAtInfo(t *testing.T) {
	buf := withCapturedDefaultLogger(t)

	picker := &fakePicker{pick: usecase.Pick{
		Kind: usecase.PickOperation, Service: "svc-a", OperationID: "Opa", Ambiguous: true,
	}}
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionCall, Service: "svc-a", OperationID: "Opa"}}

	orchestrator := stagedOrchestrator(t, picker, planner, &fakeInvoker{data: map[string]any{}})

	_, err := orchestrator.Plan(t.Context(), adminUser(), "質問", nil, nil, "", "", nil)
	require.NoError(t, err)

	logged := buf.String()
	assert.Contains(t, logged, `"pick_operation_id":"Opa"`)
	assert.Contains(t, logged, `"pick_ambiguous":true`)
	assert.Contains(t, logged, `"pick_ms":`)
}

// TestPlanStagedWrapsAPickerError proves a Picker's error reaches the
// caller, wrapped, rather than being swallowed - the same treatment
// TestPlanWrapsAPlannerError and TestPlanWrapsAnInvokerError give their
// own collaborators' errors.
func TestPlanStagedWrapsAPickerError(t *testing.T) {
	boom := assert.AnError
	picker := &fakePicker{err: boom}
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}

	orchestrator := stagedOrchestrator(t, picker, planner, &fakeInvoker{})

	_, err := orchestrator.Plan(t.Context(), adminUser(), "質問", nil, nil, "", "", nil)

	require.ErrorIs(t, err, boom)
}
