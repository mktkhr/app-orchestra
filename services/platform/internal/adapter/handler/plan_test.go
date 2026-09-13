package handler_test

import (
	"context"
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

	user    *domain.User
	query   string
	answers []usecase.Answer
	turns   []usecase.Turn
}

func (f *fakeOrchestrator) Plan(
	_ context.Context,
	user *domain.User,
	query string,
	answers []usecase.Answer,
	turns []usecase.Turn,
) (usecase.Result, error) {
	f.user = user
	f.query = query
	f.answers = answers
	f.turns = turns

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
