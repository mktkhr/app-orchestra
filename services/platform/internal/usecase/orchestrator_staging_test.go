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
		{
			// Ope is unsafe (POST, not GET/HEAD/QUERY - domain.Endpoint.
			// IsSafe) with a required parameter of its own: the fixture
			// TestPlanStagedOnPickOperationWithRequiredParameterCallsThe
			// PlannerAndFormsWithItsArgs needs (create/create-attendance
			// regression, docs/specs/staging.md section 5) - a pick whose
			// question carries the argument an unsafe operation requires,
			// so the fill must still call the model rather than shortcut
			// straight to an empty form.
			Service: "svc-e", OperationID: "Ope", Method: "POST", Path: "/e", Summary: "operation e",
			DisplayName: "操作e",
			Parameters: []domain.Parameter{
				{Name: "name", In: "body", Required: true, Schema: domain.Schema{Type: domain.SchemaTypeString}},
			},
			Response: &domain.Schema{Type: domain.SchemaTypeObject},
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

// TestPlanStagedOnPickOperationWithRequiredParameterCallsThePlannerAndForms
// WithItsArgs is the create/create-attendance regression fix
// (docs/specs/staging.md section 5): a pick's question is the source of
// its own arguments, so planPreferred's required-parameter shortcut must
// not apply when fromPick is true - Ope's required "name" is not in
// answers, but the planner is still consulted, and its Args (which the
// question itself carried, e.g. 「名前はテスト品、数量は5、引当済で」)
// come back as the D8 confirm form's Initial values, because Ope is
// unsafe (o.call degrades an unsafe call to formFor with decision.Args).
func TestPlanStagedOnPickOperationWithRequiredParameterCallsThePlannerAndFormsWithItsArgs(t *testing.T) {
	picker := &fakePicker{pick: usecase.Pick{Kind: usecase.PickOperation, Service: "svc-e", OperationID: "Ope"}}
	planner := &fakePlanner{decision: usecase.Decision{
		Kind: usecase.DecisionCall, Service: "svc-e", OperationID: "Ope",
		Args: map[string]any{"name": "テスト品"},
	}}

	orchestrator := stagedOrchestrator(t, picker, planner, &fakeInvoker{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "登録して。名前はテスト品で", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, 1, planner.calls, "a pick must always consult the model for its arguments")
	require.Len(t, planner.tools, 1, "only the picked operation's tool must be offered")
	assert.Equal(t, "Ope", planner.tools[0].Name)
	assert.Equal(t, usecase.ResultKindForm, result.Kind, "Ope is unsafe, so the call degrades to a confirm form")
	assert.Equal(t, "Ope", result.OperationID)
	assert.Equal(t, map[string]any{"name": "テスト品"}, result.Initial)
}

// TestPlanStagedOnPickOperationWithRequiredParameterFallsBackToFormOnAnyOther
// Decision is the fallback planPreferred already had, still reached
// through a pick now that the shortcut above no longer applies before the
// call: Opc's required "id" cannot be supplied by the model either, and
// whatever it returns instead of a call to Opc degrades to the same form
// planPreferred always built for this case, built from the answers rather
// than the model's own (unusable) Args.
func TestPlanStagedOnPickOperationWithRequiredParameterFallsBackToFormOnAnyOtherDecision(t *testing.T) {
	picker := &fakePicker{pick: usecase.Pick{Kind: usecase.PickOperation, Service: "svc-c", OperationID: "Opc"}}
	planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionNone}}

	orchestrator := stagedOrchestrator(t, picker, planner, &fakeInvoker{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "質問", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, 1, planner.calls, "the planner must still be consulted before falling back to the form")
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

// TestPlanStagedFromPickHonoursARuleTwoAsk is planPreferred's fromPick
// DecisionAsk routing (line ~84, orchestrator_preferred.go) reaching
// askDegrade's rule 2: Opd declares no parameters at all, so "kind" is
// neither an enum (optionsForParam) nor required (askDegrade rule 1), and
// the model's own two options are trusted verbatim.
func TestPlanStagedFromPickHonoursARuleTwoAsk(t *testing.T) {
	picker := &fakePicker{pick: usecase.Pick{Kind: usecase.PickOperation, Service: "svc-d", OperationID: "Opd"}}
	planner := &fakePlanner{decision: usecase.Decision{
		Kind: usecase.DecisionAsk, Service: "svc-d", OperationID: "Opd", Question: "受注ですか、発注ですか？", Param: "kind",
		Options: []domain.Option{{Value: "sales", Label: "受注"}, {Value: "purchase", Label: "発注"}},
	}}

	orchestrator := stagedOrchestrator(t, picker, planner, &fakeInvoker{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "注文を見たい", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindAsk, result.Kind)
	assert.Equal(t, "受注ですか、発注ですか？", result.Question)
	assert.Equal(t, "kind", result.Param)
	assert.Equal(t, []domain.Option{{Value: "sales", Label: "受注"}, {Value: "purchase", Label: "発注"}}, result.Options)
}

// TestPlanStagedFromPickHonoursARuleThreeAsk is the same routing reaching
// askDegrade's rule 3: fewer than two model options and a non-required
// param degrades to a plain question, not a form.
func TestPlanStagedFromPickHonoursARuleThreeAsk(t *testing.T) {
	picker := &fakePicker{pick: usecase.Pick{Kind: usecase.PickOperation, Service: "svc-d", OperationID: "Opd"}}
	planner := &fakePlanner{decision: usecase.Decision{
		Kind: usecase.DecisionAsk, Service: "svc-d", OperationID: "Opd", Question: "いつの分ですか？", Param: "period",
	}}

	orchestrator := stagedOrchestrator(t, picker, planner, &fakeInvoker{})

	result, err := orchestrator.Plan(t.Context(), adminUser(), "操作dを見たい", nil, nil, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, usecase.ResultKindAsk, result.Kind)
	assert.Equal(t, "いつの分ですか？", result.Question)
	assert.Empty(t, result.Param)
	assert.Empty(t, result.Options)
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
