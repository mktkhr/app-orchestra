package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/handler"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/openapi"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// fakeOrchestrator is a test double for the planner interface plan.go
// declares: it always answers with the fixed result (or error) it was
// built with, and records what it was called with.
type fakeOrchestrator struct {
	result usecase.Result
	err    error

	user        *domain.User
	query       string
	answers     []usecase.Answer
	turns       []usecase.Turn
	workspaceID string
	preferred   string
	thinking    *bool
}

func (f *fakeOrchestrator) Plan(
	_ context.Context,
	user *domain.User,
	query string,
	answers []usecase.Answer,
	turns []usecase.Turn,
	workspaceID, preferred string,
	thinking *bool,
) (usecase.Result, error) {
	f.user = user
	f.query = query
	f.answers = answers
	f.turns = turns
	f.workspaceID = workspaceID
	f.preferred = preferred
	f.thinking = thinking

	return f.result, f.err
}

func TestPostPlanRendersAResult(t *testing.T) {
	orchestrator := &fakeOrchestrator{result: usecase.Result{
		Kind:               usecase.ResultKindResult,
		Component:          domain.ComponentTable,
		Data:               map[string]any{"items": []any{}},
		Service:            "inventory",
		ServiceDisplayName: "在庫管理",
		OperationID:        "ListInventoryItems",
		Args:               map[string]any{"status": "allocated"},
	}}

	h := handler.NewPlan(orchestrator)

	resp, err := h.PostPlan(t.Context(), openapi.PostPlanRequestObject{
		Body: &openapi.PlanRequest{Query: "在庫の一覧を見せて"},
	})

	require.NoError(t, err)
	body, ok := resp.(openapi.PostPlan200JSONResponse)
	require.True(t, ok)

	assert.Equal(t, openapi.DecisionKind("result"), body.Kind)
	require.NotNil(t, body.Component)
	assert.Equal(t, openapi.Component("table"), *body.Component)
	require.NotNil(t, body.Data)
	assert.Equal(t, map[string]any{"items": []any{}}, *body.Data)
	require.NotNil(t, body.Source)
	assert.Equal(t, "inventory", body.Source.Service)
	assert.Equal(t, "在庫管理", body.Source.ServiceDisplayName)
	assert.Equal(t, "ListInventoryItems", body.Source.OperationId)
	require.NotNil(t, body.Source.Args)
	assert.Equal(t, map[string]any{"status": "allocated"}, *body.Source.Args)

	assert.Nil(t, body.Fields)
	assert.Nil(t, body.View, "no chart hint was declared, so the result carries no view")

	assert.Equal(t, "在庫の一覧を見せて", orchestrator.query)
}

// TestPostPlanRendersAResultWithView is AC-P-105's wire half: a result
// whose endpoint declared x-ui-hint.chart carries the contract's own axes
// as view.chart, and no transform - a contract declares axes, never a
// transform.
func TestPostPlanRendersAResultWithView(t *testing.T) {
	orchestrator := &fakeOrchestrator{result: usecase.Result{
		Kind:        usecase.ResultKindResult,
		Component:   domain.ComponentChart,
		Data:        map[string]any{"items": []any{}},
		Service:     "inventory",
		OperationID: "ListInventoryItems",
		View: &domain.View{
			Chart: &domain.Chart{Category: "status", Value: "count", Kind: domain.ChartKindBar},
		},
	}}

	h := handler.NewPlan(orchestrator)

	resp, err := h.PostPlan(t.Context(), openapi.PostPlanRequestObject{
		Body: &openapi.PlanRequest{Query: "ステータス別の件数を見せて"},
	})

	require.NoError(t, err)
	body, ok := resp.(openapi.PostPlan200JSONResponse)
	require.True(t, ok)

	require.NotNil(t, body.View)
	assert.Nil(t, body.View.Transform)
	require.NotNil(t, body.View.Chart)
	assert.Equal(t, "status", body.View.Chart.Category)
	assert.Equal(t, "count", body.View.Chart.Value)
	assert.Equal(t, openapi.ViewChartKind("bar"), body.View.Chart.Kind)
}

func TestPostPlanRendersAResultWithFields(t *testing.T) {
	orchestrator := &fakeOrchestrator{result: usecase.Result{
		Kind:        usecase.ResultKindResult,
		Component:   domain.ComponentTable,
		Data:        map[string]any{"items": []any{}},
		Service:     "inventory",
		OperationID: "ListInventoryItems",
		Fields: map[string]any{
			"status": map[string]any{
				"type":       "string",
				"enum":       []string{"quarantined"},
				"enumLabels": map[string]string{"quarantined": "検品保留"},
			},
		},
	}}

	h := handler.NewPlan(orchestrator)

	resp, err := h.PostPlan(t.Context(), openapi.PostPlanRequestObject{
		Body: &openapi.PlanRequest{Query: "検品保留の在庫を見せて"},
	})

	require.NoError(t, err)
	body, ok := resp.(openapi.PostPlan200JSONResponse)
	require.True(t, ok)

	require.NotNil(t, body.Fields)

	status, ok := (*body.Fields)["status"].(map[string]any)
	require.True(t, ok)

	enumLabels, ok := status["enumLabels"].(map[string]string)
	require.True(t, ok)
	assert.Equal(t, "検品保留", enumLabels["quarantined"])
}

func TestPostPlanRendersAForm(t *testing.T) {
	orchestrator := &fakeOrchestrator{result: usecase.Result{
		Kind:               usecase.ResultKindForm,
		Service:            "inventory",
		ServiceDisplayName: "在庫管理",
		OperationID:        "CreateInventoryItem",
		Schema:             map[string]any{"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}}},
		Initial:            map[string]any{"name": "widget"},
	}}

	h := handler.NewPlan(orchestrator)

	resp, err := h.PostPlan(t.Context(), openapi.PostPlanRequestObject{
		Body: &openapi.PlanRequest{Query: "在庫を登録して"},
	})

	require.NoError(t, err)
	body, ok := resp.(openapi.PostPlan200JSONResponse)
	require.True(t, ok)

	assert.Equal(t, openapi.DecisionKind("form"), body.Kind)
	require.NotNil(t, body.Schema)
	assert.Equal(t, map[string]any{"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}}}, *body.Schema)
	require.NotNil(t, body.Initial)
	assert.Equal(t, map[string]any{"name": "widget"}, *body.Initial)
	require.NotNil(t, body.Target)
	assert.Equal(t, "inventory", body.Target.Service)
	assert.Equal(t, "在庫管理", body.Target.ServiceDisplayName)
	assert.Equal(t, "CreateInventoryItem", body.Target.OperationId)
	assert.Nil(t, body.Source)
	assert.Nil(t, body.Data)
}

func TestPostPlanRendersNone(t *testing.T) {
	orchestrator := &fakeOrchestrator{result: usecase.Result{
		Kind:    usecase.ResultKindNone,
		Message: "見つかりませんでした",
	}}

	h := handler.NewPlan(orchestrator)

	resp, err := h.PostPlan(t.Context(), openapi.PostPlanRequestObject{
		Body: &openapi.PlanRequest{Query: "今日の天気は？"},
	})

	require.NoError(t, err)
	body, ok := resp.(openapi.PostPlan200JSONResponse)
	require.True(t, ok)

	assert.Equal(t, openapi.DecisionKind("none"), body.Kind)
	require.NotNil(t, body.Message)
	assert.Equal(t, "見つかりませんでした", *body.Message)
	assert.Nil(t, body.Source)
}

func TestPostPlanRendersAnAsk(t *testing.T) {
	orchestrator := &fakeOrchestrator{result: usecase.Result{
		Kind:     usecase.ResultKindAsk,
		Question: "どのステータスですか？",
		Param:    "status",
		Options: []domain.Option{
			{Value: "allocated", Label: "引当済"},
			{Value: "quarantined", Label: "検品保留"},
		},
	}}

	h := handler.NewPlan(orchestrator)

	resp, err := h.PostPlan(t.Context(), openapi.PostPlanRequestObject{
		Body: &openapi.PlanRequest{Query: "破損した在庫を見せて"},
	})

	require.NoError(t, err)
	body, ok := resp.(openapi.PostPlan200JSONResponse)
	require.True(t, ok)

	assert.Equal(t, openapi.DecisionKind("ask"), body.Kind)
	require.NotNil(t, body.Question)
	assert.Equal(t, "どのステータスですか？", *body.Question)
	require.NotNil(t, body.Param)
	assert.Equal(t, "status", *body.Param)
	require.NotNil(t, body.Options)
	assert.Equal(t, []openapi.Option{
		{Value: "allocated", Label: "引当済"},
		{Value: "quarantined", Label: "検品保留"},
	}, *body.Options)
	assert.Nil(t, body.Source)
	assert.Nil(t, body.Data)
}

// TestPostPlanRendersAPlainAskWithNoParamOrOptions is defect 1's wire half
// (docs/specs/shortlisting.md): Orchestrator.ask degrades an unknown or
// fabricated operation id to a plain question rather than
// ErrEndpointNotFound. That plain ask carries a question and nothing else
// - PlanResult.param and PlanResult.options must come back absent (nil),
// not pointing at an empty string and an empty array, since both are
// already optional on the "ask" variant of the contract.
func TestPostPlanRendersAPlainAskWithNoParamOrOptions(t *testing.T) {
	orchestrator := &fakeOrchestrator{result: usecase.Result{
		Kind:     usecase.ResultKindAsk,
		Question: "何について知りたいですか？",
	}}

	h := handler.NewPlan(orchestrator)

	resp, err := h.PostPlan(t.Context(), openapi.PostPlanRequestObject{
		Body: &openapi.PlanRequest{Query: "承認は必要？"},
	})

	require.NoError(t, err)
	body, ok := resp.(openapi.PostPlan200JSONResponse)
	require.True(t, ok)

	assert.Equal(t, openapi.DecisionKind("ask"), body.Kind)
	require.NotNil(t, body.Question)
	assert.Equal(t, "何について知りたいですか？", *body.Question)
	assert.Nil(t, body.Param)
	assert.Nil(t, body.Options)
}

// TestPostPlanRendersAProposal is section 4's wire half: kind "proposal"
// carries a panel, not a "result" with an extra field - Data and Source,
// the fields a "result" carries, stay nil.
func TestPostPlanRendersAProposal(t *testing.T) {
	orchestrator := &fakeOrchestrator{result: usecase.Result{
		Kind:        usecase.ResultKindProposal,
		Service:     "inventory",
		OperationID: "ListInventoryItems",
		Args:        map[string]any{"status": "quarantined"},
		Component:   domain.ComponentChart,
		View: &domain.View{
			Chart: &domain.Chart{Category: "status", Value: "count", Kind: domain.ChartKindBar},
		},
		Title: "ステータス別の在庫",
	}}

	h := handler.NewPlan(orchestrator)

	resp, err := h.PostPlan(t.Context(), openapi.PostPlanRequestObject{
		Body: &openapi.PlanRequest{Query: "在庫をステータス別に棒グラフで置いて"},
	})

	require.NoError(t, err)
	body, ok := resp.(openapi.PostPlan200JSONResponse)
	require.True(t, ok)

	assert.Equal(t, openapi.DecisionKind("proposal"), body.Kind)
	assert.Nil(t, body.Data, "a proposal is not a result: it carries no data")
	assert.Nil(t, body.Source, "a proposal is not a result: it carries no source")

	require.NotNil(t, body.Panel)
	assert.Equal(t, "inventory", body.Panel.Service)
	assert.Equal(t, "ListInventoryItems", body.Panel.OperationId)
	assert.Equal(t, map[string]any{"status": "quarantined"}, body.Panel.Args)
	assert.Equal(t, openapi.Component("chart"), body.Panel.Component)
	assert.Equal(t, "ステータス別の在庫", body.Panel.Title)
	require.NotNil(t, body.Panel.View)
	require.NotNil(t, body.Panel.View.Chart)
	assert.Equal(t, "status", body.Panel.View.Chart.Category)
	assert.Equal(t, "count", body.Panel.View.Chart.Value)
}

// TestPostPlanRendersAProposalWithNoArgsAsAnEmptyObject proves
// ProposedPanel.Args - not a pointer on the wire - is always present, even
// when the model proposed a panel with no arguments at all.
func TestPostPlanRendersAProposalWithNoArgsAsAnEmptyObject(t *testing.T) {
	orchestrator := &fakeOrchestrator{result: usecase.Result{
		Kind:        usecase.ResultKindProposal,
		Service:     "inventory",
		OperationID: "ListInventoryItems",
		Component:   domain.ComponentTable,
		Title:       "ListInventoryItems",
	}}

	h := handler.NewPlan(orchestrator)

	resp, err := h.PostPlan(t.Context(), openapi.PostPlanRequestObject{
		Body: &openapi.PlanRequest{Query: "在庫の一覧を置いて"},
	})

	require.NoError(t, err)
	body, ok := resp.(openapi.PostPlan200JSONResponse)
	require.True(t, ok)

	require.NotNil(t, body.Panel)
	assert.Equal(t, map[string]any{}, body.Panel.Args)
	assert.Nil(t, body.Panel.View)
}

func TestPostPlanPassesAnswersThrough(t *testing.T) {
	orchestrator := &fakeOrchestrator{result: usecase.Result{Kind: usecase.ResultKindNone}}

	h := handler.NewPlan(orchestrator)

	_, err := h.PostPlan(t.Context(), openapi.PostPlanRequestObject{
		Body: &openapi.PlanRequest{
			Query:   "在庫の一覧を見せて",
			Answers: &[]openapi.Answer{{Param: "status", Value: "allocated"}},
		},
	})

	require.NoError(t, err)
	require.Len(t, orchestrator.answers, 1)
	assert.Equal(t, usecase.Answer{Param: "status", Value: "allocated"}, orchestrator.answers[0])
}

// TestPostPlanPassesWorkspaceIDThrough is O4 (docs/specs/offering.md):
// PostPlan reads PlanRequest's own optional workspaceId and forwards it to
// Orchestrator.Plan unchanged.
func TestPostPlanPassesWorkspaceIDThrough(t *testing.T) {
	orchestrator := &fakeOrchestrator{result: usecase.Result{Kind: usecase.ResultKindNone}}

	h := handler.NewPlan(orchestrator)

	workspaceID := "ws-1"

	_, err := h.PostPlan(t.Context(), openapi.PostPlanRequestObject{
		Body: &openapi.PlanRequest{Query: "在庫の一覧を見せて", WorkspaceId: &workspaceID},
	})

	require.NoError(t, err)
	assert.Equal(t, "ws-1", orchestrator.workspaceID)
}

// TestPostPlanWithNoWorkspaceIDPassesEmptyStringThrough is the chat
// screen's own case: PlanRequest with no workspaceId at all (what
// pages/chat sends) must reach Orchestrator.Plan as "", not as a nil the
// orchestrator has to guess the meaning of.
func TestPostPlanWithNoWorkspaceIDPassesEmptyStringThrough(t *testing.T) {
	orchestrator := &fakeOrchestrator{result: usecase.Result{Kind: usecase.ResultKindNone}}

	h := handler.NewPlan(orchestrator)

	_, err := h.PostPlan(t.Context(), openapi.PostPlanRequestObject{
		Body: &openapi.PlanRequest{Query: "在庫の一覧を見せて"},
	})

	require.NoError(t, err)
	assert.Empty(t, orchestrator.workspaceID)
}

func TestPostPlanPassesTurnsThrough(t *testing.T) {
	orchestrator := &fakeOrchestrator{result: usecase.Result{Kind: usecase.ResultKindNone}}

	h := handler.NewPlan(orchestrator)

	service := "inventory"
	operationID := "ListInventoryItems"
	args := map[string]any{"status": "quarantined"}

	_, err := h.PostPlan(t.Context(), openapi.PostPlanRequestObject{
		Body: &openapi.PlanRequest{
			Query: "在庫の一覧を見せて",
			Turns: &[]openapi.Turn{{
				Question:    "検品保留の在庫を見せて",
				Kind:        openapi.DecisionKindResult,
				Service:     &service,
				OperationId: &operationID,
				Args:        &args,
			}},
		},
	})

	require.NoError(t, err)
	require.Len(t, orchestrator.turns, 1)
	assert.Equal(t, usecase.Turn{
		Question:    "検品保留の在庫を見せて",
		Kind:        usecase.ResultKindResult,
		Service:     "inventory",
		OperationID: "ListInventoryItems",
		Args:        map[string]any{"status": "quarantined"},
	}, orchestrator.turns[0])
}

func TestPostPlanWithNoTurnsPassesNilThrough(t *testing.T) {
	orchestrator := &fakeOrchestrator{result: usecase.Result{Kind: usecase.ResultKindNone}}

	h := handler.NewPlan(orchestrator)

	_, err := h.PostPlan(t.Context(), openapi.PostPlanRequestObject{
		Body: &openapi.PlanRequest{Query: "在庫の一覧を見せて"},
	})

	require.NoError(t, err)
	assert.Nil(t, orchestrator.turns)
}

func TestPostPlanNotImplementedErrorIs501(t *testing.T) {
	orchestrator := &fakeOrchestrator{err: errors.Join(usecase.ErrNotImplemented, errors.New("form path"))}

	h := handler.NewPlan(orchestrator)

	resp, err := h.PostPlan(t.Context(), openapi.PostPlanRequestObject{Body: &openapi.PlanRequest{Query: "在庫を登録して"}})

	require.NoError(t, err)
	_, ok := resp.(openapi.PostPlan501JSONResponse)
	assert.True(t, ok, "expected a 501 response, got %T", resp)
}

func TestPostPlanOtherErrorIs500(t *testing.T) {
	orchestrator := &fakeOrchestrator{err: errors.New("boom")}

	h := handler.NewPlan(orchestrator)

	resp, err := h.PostPlan(t.Context(), openapi.PostPlanRequestObject{Body: &openapi.PlanRequest{Query: "何か"}})

	require.NoError(t, err)
	_, ok := resp.(openapi.PostPlan500JSONResponse)
	assert.True(t, ok, "expected a 500 response, got %T", resp)
}

// TestPostPlanPassesPreferredThrough is section 4 of
// docs/specs/shortlisting.md: PostPlan reads PlanRequest's own optional
// preferred and forwards it to Orchestrator.Plan unchanged, the same shape
// TestPostPlanPassesWorkspaceIDThrough already proves for workspaceId.
func TestPostPlanPassesPreferredThrough(t *testing.T) {
	orchestrator := &fakeOrchestrator{result: usecase.Result{Kind: usecase.ResultKindNone}}

	h := handler.NewPlan(orchestrator)

	preferred := "ListInventoryItems"

	_, err := h.PostPlan(t.Context(), openapi.PostPlanRequestObject{
		Body: &openapi.PlanRequest{Query: "さっき見せてくれた方じゃなくて", Preferred: &preferred},
	})

	require.NoError(t, err)
	assert.Equal(t, "ListInventoryItems", orchestrator.preferred)
}

// TestPostPlanWithNoPreferredPassesEmptyStringThrough is the ordinary
// question's own case: a PlanRequest with no preferred at all must reach
// Orchestrator.Plan as "", not as a nil the orchestrator has to guess the
// meaning of.
func TestPostPlanWithNoPreferredPassesEmptyStringThrough(t *testing.T) {
	orchestrator := &fakeOrchestrator{result: usecase.Result{Kind: usecase.ResultKindNone}}

	h := handler.NewPlan(orchestrator)

	_, err := h.PostPlan(t.Context(), openapi.PostPlanRequestObject{
		Body: &openapi.PlanRequest{Query: "在庫の一覧を見せて"},
	})

	require.NoError(t, err)
	assert.Empty(t, orchestrator.preferred)
}

// TestPostPlanPassesThinkingThrough is the contract round-trip half of the
// platform knobs subproject (decided 2026-09-16): a PlanRequest carrying
// thinking: false must reach Orchestrator.Plan as that same *bool, not
// dropped or coerced along the way.
func TestPostPlanPassesThinkingThrough(t *testing.T) {
	orchestrator := &fakeOrchestrator{result: usecase.Result{Kind: usecase.ResultKindNone}}

	h := handler.NewPlan(orchestrator)

	thinking := false

	_, err := h.PostPlan(t.Context(), openapi.PostPlanRequestObject{
		Body: &openapi.PlanRequest{Query: "在庫の一覧を見せて", Thinking: &thinking},
	})

	require.NoError(t, err)
	require.NotNil(t, orchestrator.thinking)
	assert.False(t, *orchestrator.thinking)
}

// TestPostPlanPassesThinkingTrueThrough is the same round trip with the
// switch on, proving the value passed through is the request's own, not a
// hardcoded false.
func TestPostPlanPassesThinkingTrueThrough(t *testing.T) {
	orchestrator := &fakeOrchestrator{result: usecase.Result{Kind: usecase.ResultKindNone}}

	h := handler.NewPlan(orchestrator)

	thinking := true

	_, err := h.PostPlan(t.Context(), openapi.PostPlanRequestObject{
		Body: &openapi.PlanRequest{Query: "在庫の一覧を見せて", Thinking: &thinking},
	})

	require.NoError(t, err)
	require.NotNil(t, orchestrator.thinking)
	assert.True(t, *orchestrator.thinking)
}

// TestPostPlanWithNoThinkingPassesNilThrough is the omitted case: a
// PlanRequest with no thinking field at all must reach Orchestrator.Plan
// as nil - "use the platform's configured default" - never coerced to
// false.
func TestPostPlanWithNoThinkingPassesNilThrough(t *testing.T) {
	orchestrator := &fakeOrchestrator{result: usecase.Result{Kind: usecase.ResultKindNone}}

	h := handler.NewPlan(orchestrator)

	_, err := h.PostPlan(t.Context(), openapi.PostPlanRequestObject{
		Body: &openapi.PlanRequest{Query: "在庫の一覧を見せて"},
	})

	require.NoError(t, err)
	assert.Nil(t, orchestrator.thinking)
}

// TestPostPlanRendersAlternatives is AC-H-103's wire half: a result
// carrying usecase.Alternative entries renders them onto PlanResult's own
// "alternatives" array, by operationId, displayName and service.
func TestPostPlanRendersAlternatives(t *testing.T) {
	orchestrator := &fakeOrchestrator{result: usecase.Result{
		Kind:        usecase.ResultKindResult,
		Component:   domain.ComponentTable,
		Data:        map[string]any{"items": []any{}},
		Service:     "inventory",
		OperationID: "ListInventoryItems",
		Alternatives: []usecase.Alternative{
			{OperationID: "ListAttendanceRecords", DisplayName: "勤怠一覧", Service: "attendance"},
			{OperationID: "ListShipments", DisplayName: "出荷一覧", Service: "shipping"},
		},
	}}

	h := handler.NewPlan(orchestrator)

	resp, err := h.PostPlan(t.Context(), openapi.PostPlanRequestObject{
		Body: &openapi.PlanRequest{Query: "在庫の一覧を見せて"},
	})

	require.NoError(t, err)
	body, ok := resp.(openapi.PostPlan200JSONResponse)
	require.True(t, ok)

	require.NotNil(t, body.Alternatives)
	assert.Equal(t, []openapi.Alternative{
		{OperationId: "ListAttendanceRecords", DisplayName: "勤怠一覧", Service: "attendance"},
		{OperationId: "ListShipments", DisplayName: "出荷一覧", Service: "shipping"},
	}, *body.Alternatives)
}

// TestPostPlanOmitsAlternativesFromTheWireWhenThereAreNone is H7's wire
// half: a result with no Alternatives at all - what every existing
// PassThroughNarrower result is - re-marshals with no "alternatives" key,
// not an empty array, so the response stays byte-identical to what it was
// before this field existed (the same proof
// TestPlanWithPassThroughNarrowerOffersByteIdenticalTools gives the tool
// list, one layer down).
func TestPostPlanOmitsAlternativesFromTheWireWhenThereAreNone(t *testing.T) {
	orchestrator := &fakeOrchestrator{result: usecase.Result{
		Kind:        usecase.ResultKindResult,
		Component:   domain.ComponentTable,
		Data:        map[string]any{"items": []any{}},
		Service:     "inventory",
		OperationID: "ListInventoryItems",
	}}

	h := handler.NewPlan(orchestrator)

	resp, err := h.PostPlan(t.Context(), openapi.PostPlanRequestObject{
		Body: &openapi.PlanRequest{Query: "在庫の一覧を見せて"},
	})

	require.NoError(t, err)
	body, ok := resp.(openapi.PostPlan200JSONResponse)
	require.True(t, ok)
	assert.Nil(t, body.Alternatives)

	wire, err := json.Marshal(openapi.PlanResult(body))
	require.NoError(t, err)
	assert.NotContains(t, string(wire), "alternatives")
}

// TestPostPlanOmitsAlternativesForNonResultKinds proves toAPIPlanResult
// never carries Alternatives onto any kind but "result": a form, ask or
// none never offers a further candidate to choose instead, whatever a
// usecase.Result happened to be built with.
func TestPostPlanOmitsAlternativesForNonResultKinds(t *testing.T) {
	orchestrator := &fakeOrchestrator{result: usecase.Result{
		Kind:    usecase.ResultKindNone,
		Message: "見つかりませんでした",
		Alternatives: []usecase.Alternative{
			{OperationID: "ListShipments", DisplayName: "出荷一覧", Service: "shipping"},
		},
	}}

	h := handler.NewPlan(orchestrator)

	resp, err := h.PostPlan(t.Context(), openapi.PostPlanRequestObject{
		Body: &openapi.PlanRequest{Query: "何か"},
	})

	require.NoError(t, err)
	body, ok := resp.(openapi.PostPlan200JSONResponse)
	require.True(t, ok)
	assert.Nil(t, body.Alternatives)
}

func TestPostPlanUnrenderableDataIs500(t *testing.T) {
	orchestrator := &fakeOrchestrator{result: usecase.Result{
		Kind: usecase.ResultKindResult,
		Data: []any{"not an object"},
	}}

	h := handler.NewPlan(orchestrator)

	resp, err := h.PostPlan(t.Context(), openapi.PostPlanRequestObject{Body: &openapi.PlanRequest{Query: "何か"}})

	require.NoError(t, err)
	_, ok := resp.(openapi.PostPlan500JSONResponse)
	assert.True(t, ok, "expected a 500 response, got %T", resp)
}
