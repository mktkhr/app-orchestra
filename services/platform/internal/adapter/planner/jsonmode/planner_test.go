package jsonmode_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/chat"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/jsonmode"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// fixtureCatalog mirrors toolcall's own fixture (planner_test.go in the
// sibling package): one safe list operation with an enum parameter, one
// unsafe create, so both the happy path and the enum-validation path have
// something real to check against.
func fixtureCatalog() domain.Catalog {
	return domain.Catalog{
		Endpoints: []domain.Endpoint{
			{
				Service:     "inventory",
				OperationID: "ListInventoryItems",
				Method:      domain.MethodGet,
				Path:        "/api/inventory/items",
				Summary:     "List inventory items, optionally filtered by status.",
				Parameters: []domain.Parameter{
					{
						Name: "status",
						In:   "query",
						Schema: domain.Schema{
							Type:       domain.SchemaTypeString,
							Enum:       []string{"allocated", "quarantined"},
							EnumLabels: map[string]string{"allocated": "引当済", "quarantined": "検品保留"},
						},
					},
				},
				Response: &domain.Schema{
					Type: domain.SchemaTypeObject,
					Properties: map[string]domain.Schema{
						"items": {Type: domain.SchemaTypeArray, Items: &domain.Schema{Type: domain.SchemaTypeObject}},
					},
				},
			},
			{
				Service:     "attendance",
				OperationID: "ListAttendanceRecords",
				Method:      domain.MethodGet,
				Path:        "/api/attendance/records",
				Summary:     "List attendance records.",
				Response: &domain.Schema{
					Type: domain.SchemaTypeObject,
					Properties: map[string]domain.Schema{
						"records": {Type: domain.SchemaTypeArray, Items: &domain.Schema{Type: domain.SchemaTypeObject}},
					},
				},
			},
		},
	}
}

// chatContent builds one chat-completions response body whose assistant
// message content is body - the shape the JSON planner reads its decision
// from, since it never calls tools. t.Fatal on a marshal failure rather
// than panicking: body is always a Go string literal in the tests below, so
// json.Marshal never actually fails - this is defensive, not expected to
// run.
func chatContent(t *testing.T, body string) string {
	t.Helper()

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshaling fixture content: %v", err)
	}

	return `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":` + string(raw) + `}}]}`
}

// chatTruncatedContent is chatContent's finish_reason "length" twin: what a
// repetition loop looked like when measured (docs/specs/shortlisting.md,
// 2026-09-15) - the model still generating, cut off mid-answer by
// chat.MaxTokens rather than finishing on its own.
func chatTruncatedContent(t *testing.T, body string) string {
	t.Helper()

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshaling fixture content: %v", err)
	}

	return `{"choices":[{"finish_reason":"length","message":{"role":"assistant","content":` + string(raw) + `}}]}`
}

// plannerFixture bundles a Planner under test with every request its stub
// server received, so a test can both drive Plan and inspect what a retry
// turn actually said - one struct field each, rather than a two-value
// return nonamedreturns and gocritic's unnamedResult would otherwise
// disagree about naming.
type plannerFixture struct {
	planner  *jsonmode.Planner
	requests []map[string]any
}

// newPlanner builds a plannerFixture whose stub server answers each
// successive chat-completions request with the next entry of bodies (each
// already a full chat-completions JSON body), repeating the last one if
// more requests arrive than bodies were given, and records every request's
// decoded body into fixture.requests.
func newPlanner(t *testing.T, catalog domain.Catalog, bodies ...string) *plannerFixture {
	t.Helper()

	fixture := &plannerFixture{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var decoded map[string]any
		if err := json.NewDecoder(r.Body).Decode(&decoded); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		fixture.requests = append(fixture.requests, decoded)

		body := bodies[len(bodies)-1]
		if n := len(fixture.requests) - 1; n < len(bodies) {
			body = bodies[n]
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := chat.New(chat.Config{BaseURL: server.URL, Model: "test-model"})
	fixture.planner = jsonmode.New(client, catalog)

	return fixture
}

func TestPlanMapsAValidCallJSONObjectOntoADecisionCall(t *testing.T) {
	fixture := newPlanner(t, fixtureCatalog(), chatContent(t,
		`{"kind":"call","service":"inventory","operationId":"ListInventoryItems","args":{"status":"quarantined"}}`,
	))

	decision, err := fixture.planner.Plan(context.Background(), "検品保留の在庫を見せて", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), nil)
	require.NoError(t, err)

	assert.Equal(t, usecase.DecisionCall, decision.Kind)
	assert.Equal(t, "inventory", decision.Service)
	assert.Equal(t, "ListInventoryItems", decision.OperationID)
	assert.Equal(t, map[string]any{"status": "quarantined"}, decision.Args)
}

// TestPlanRendersAParameterSDescriptionInTheCatalogueText pins a real bug:
// renderParam never rendered a parameter's Description at all, so this
// planner's rendered catalogue text silently dropped an operation's own
// explanatory text. A parameter with a plain Description (no enum
// involved) must have that text reach the system prompt.
func TestPlanRendersAParameterSDescriptionInTheCatalogueText(t *testing.T) {
	catalog := domain.Catalog{Endpoints: []domain.Endpoint{
		{
			Service:     "attendance",
			OperationID: "ListAttendanceRecords",
			Method:      domain.MethodGet,
			Path:        "/api/attendance/records",
			Summary:     "List attendance records.",
			Parameters: []domain.Parameter{
				{
					Name: "from", In: "query",
					Schema: domain.Schema{
						Type:        domain.SchemaTypeString,
						Description: "The earliest date to include, inclusive, as YYYY-MM-DD.",
					},
				},
			},
			Response: &domain.Schema{Type: domain.SchemaTypeArray, Items: &domain.Schema{Type: domain.SchemaTypeObject}},
		},
	}}

	fixture := newPlanner(t, catalog, chatContent(t, `{"kind":"none"}`))

	_, err := fixture.planner.Plan(context.Background(), "何か", nil, nil, usecase.ToolsFor(catalog, usecase.PlanContext{WorkspaceID: "ws-1"}), nil)
	require.NoError(t, err)

	messages, ok := fixture.requests[0]["messages"].([]any)
	require.True(t, ok)

	system, ok := messages[0].(map[string]any)
	require.True(t, ok)

	content, ok := system["content"].(string)
	require.True(t, ok)
	assert.Contains(t, content, "The earliest date to include, inclusive, as YYYY-MM-DD.",
		"a parameter's own contract Description must reach the rendered catalogue text")
}

func TestPlanMapsAskKindOntoADecisionAsk(t *testing.T) {
	fixture := newPlanner(t, fixtureCatalog(), chatContent(t,
		`{"kind":"ask","service":"inventory","operationId":"ListInventoryItems","param":"status","question":"どのステータスですか？"}`,
	))

	decision, err := fixture.planner.Plan(context.Background(), "破損した在庫はある？", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), nil)
	require.NoError(t, err)

	assert.Equal(t, usecase.DecisionAsk, decision.Kind)
	assert.Equal(t, "inventory", decision.Service)
	assert.Equal(t, "ListInventoryItems", decision.OperationID)
	assert.Equal(t, "status", decision.Param)
	assert.Equal(t, "どのステータスですか？", decision.Question)
}

// TestPlanMapsProposePanelKindOntoADecisionProposal is section 3's jsonmode
// half, and the M5/proposing.md claim that both planners reach the same
// decision shape: the same fields toolcall's
// TestPlanMapsProposePanelOntoADecisionProposal asserts on, reached from
// this transport's own JSON object instead of a tool call.
func TestPlanMapsProposePanelKindOntoADecisionProposal(t *testing.T) {
	fixture := newPlanner(t, fixtureCatalog(), chatContent(t,
		`{"kind":"propose_panel","service":"inventory","operationId":"ListInventoryItems",`+
			`"args":{"status":"quarantined"},"component":"chart",`+
			`"chart":{"category":"status","value":"count","kind":"bar"},`+
			`"transform":{"groupBy":"status","aggregate":"count"},`+
			`"title":"ステータス別の在庫"}`,
	))

	decision, err := fixture.planner.Plan(context.Background(), "在庫をステータス別に棒グラフで置いて", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), nil)
	require.NoError(t, err)

	assert.Equal(t, usecase.DecisionProposal, decision.Kind)
	assert.Equal(t, "inventory", decision.Service)
	assert.Equal(t, "ListInventoryItems", decision.OperationID)
	assert.Equal(t, map[string]any{"status": "quarantined"}, decision.Args)
	assert.Equal(t, domain.ComponentChart, decision.Component)
	require.NotNil(t, decision.View)
	require.NotNil(t, decision.View.Chart)
	assert.Equal(t, "status", decision.View.Chart.Category)
	assert.Equal(t, "count", decision.View.Chart.Value)
	assert.Equal(t, domain.ChartKindBar, decision.View.Chart.Kind)
	require.NotNil(t, decision.View.Transform)
	assert.Equal(t, "status", decision.View.Transform.GroupBy)
	assert.Equal(t, domain.AggregateCount, decision.View.Transform.Aggregate)
	assert.Equal(t, "ステータス別の在庫", decision.Title)
}

// TestPlanMapsProposePanelWithNoOptionalFieldsToAZeroValuedDecision proves
// the platform, not this planner, fills a left-out view in
// (docs/specs/proposing.md, section 4): a "propose_panel" answer naming
// nothing beyond the operation maps onto a Decision with no Component, no
// View and no Title.
func TestPlanMapsProposePanelWithNoOptionalFieldsToAZeroValuedDecision(t *testing.T) {
	fixture := newPlanner(t, fixtureCatalog(), chatContent(t,
		`{"kind":"propose_panel","service":"inventory","operationId":"ListInventoryItems","args":{}}`,
	))

	decision, err := fixture.planner.Plan(context.Background(), "在庫の一覧を置いて", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), nil)
	require.NoError(t, err)

	assert.Equal(t, usecase.DecisionProposal, decision.Kind)
	assert.Empty(t, decision.Component)
	assert.Nil(t, decision.View)
	assert.Empty(t, decision.Title)
}

// TestPlanProposePanelOnUnknownOperationReturnsAnError mirrors
// TestPlanOnUnknownOperationReturnsAnError for propose_panel's own
// validation path (decisionFromProposePanel).
func TestPlanProposePanelOnUnknownOperationReturnsAnError(t *testing.T) {
	fixture := newPlanner(t, fixtureCatalog(), chatContent(t,
		`{"kind":"propose_panel","service":"inventory","operationId":"NoSuchOperation","args":{}}`,
	))

	_, err := fixture.planner.Plan(context.Background(), "何か置いて", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), nil)
	require.Error(t, err)
}

// TestPlanProposePanelRejectsAnOutOfEnumArgument mirrors
// TestPlanRetriesOnceOnAnEnumValueOutsideTheParameterAndQuotesItBack's own
// validation, over propose_panel's args instead of a "call" answer's.
func TestPlanProposePanelRejectsAnOutOfEnumArgument(t *testing.T) {
	fixture := newPlanner(t, fixtureCatalog(),
		chatContent(t,
			`{"kind":"propose_panel","service":"inventory","operationId":"ListInventoryItems",`+
				`"args":{"status":"nonexistent"}}`,
		),
		chatContent(t,
			`{"kind":"propose_panel","service":"inventory","operationId":"ListInventoryItems",`+
				`"args":{"status":"quarantined"}}`,
		),
	)

	decision, err := fixture.planner.Plan(context.Background(), "検品保留の在庫を置いて", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), nil)
	require.NoError(t, err)
	assert.Equal(t, usecase.DecisionProposal, decision.Kind)
	assert.Equal(t, map[string]any{"status": "quarantined"}, decision.Args)
	assert.Len(t, fixture.requests, 2, "the first, out-of-enum answer must have been rejected and retried")
}

func TestPlanMapsListCapabilitiesKindOntoADecisionListCapabilities(t *testing.T) {
	fixture := newPlanner(t, fixtureCatalog(), chatContent(t, `{"kind":"list_capabilities","service":"inventory"}`))

	decision, err := fixture.planner.Plan(
		context.Background(), "在庫について、どういう操作ができる？", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}),
		nil,
	)
	require.NoError(t, err)

	assert.Equal(t, usecase.DecisionListCapabilities, decision.Kind)
	assert.Equal(t, "inventory", decision.Service)
}

func TestPlanMapsListCapabilitiesWithNoServiceToAnEmptyFilter(t *testing.T) {
	fixture := newPlanner(t, fixtureCatalog(), chatContent(t, `{"kind":"list_capabilities"}`))

	decision, err := fixture.planner.Plan(context.Background(), "何ができるの？", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), nil)
	require.NoError(t, err)

	assert.Equal(t, usecase.DecisionListCapabilities, decision.Kind)
	assert.Empty(t, decision.Service)
}

func TestPlanMapsNoneKindOntoDecisionNone(t *testing.T) {
	fixture := newPlanner(t, fixtureCatalog(), chatContent(t, `{"kind":"none"}`))

	decision, err := fixture.planner.Plan(context.Background(), "今日の天気は？", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), nil)
	require.NoError(t, err)

	assert.Equal(t, usecase.DecisionNone, decision.Kind)
}

func TestPlanOnUnknownOperationReturnsAnError(t *testing.T) {
	fixture := newPlanner(t, fixtureCatalog(), chatContent(t,
		`{"kind":"call","service":"inventory","operationId":"NoSuchOperation","args":{}}`,
	))

	_, err := fixture.planner.Plan(context.Background(), "何か", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), nil)
	require.Error(t, err)
}

// TestPlanRetriesOnceOnAnEnumValueOutsideTheParameterAndQuotesItBack is
// Task 11 Step 3: a first answer naming an enum value the parameter does
// not allow is rejected - nothing on the model's side enforces the enum
// the way strict:true does for tool calling - retried once with the
// rejected value quoted back, and the second, valid answer is used.
func TestPlanRetriesOnceOnAnEnumValueOutsideTheParameterAndQuotesItBack(t *testing.T) {
	fixture := newPlanner(t, fixtureCatalog(),
		chatContent(t, `{"kind":"call","service":"inventory","operationId":"ListInventoryItems","args":{"status":"lost"}}`),
		chatContent(t, `{"kind":"call","service":"inventory","operationId":"ListInventoryItems","args":{"status":"quarantined"}}`),
	)

	decision, err := fixture.planner.Plan(context.Background(), "紛失した在庫を見せて", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), nil)
	require.NoError(t, err)

	assert.Equal(t, usecase.DecisionCall, decision.Kind)
	assert.Equal(t, "quarantined", decision.Args["status"])

	require.Len(t, fixture.requests, 2)

	secondMessages, ok := fixture.requests[1]["messages"].([]any)
	require.True(t, ok)

	last, ok := secondMessages[len(secondMessages)-1].(map[string]any)
	require.True(t, ok)

	content, ok := last["content"].(string)
	require.True(t, ok)
	assert.Contains(t, content, "lost")
}

// TestPlanGivesUpAfterASecondBadAnswer is Task 11 Step 4: two bad answers
// in a row produce an error - no usecase.Decision, and so no service call
// can ever follow from it (Orchestrator never sees a Kind to act on).
func TestPlanGivesUpAfterASecondBadAnswer(t *testing.T) {
	fixture := newPlanner(t, fixtureCatalog(),
		chatContent(t, `{"kind":"call","service":"inventory","operationId":"ListInventoryItems","args":{"status":"lost"}}`),
		chatContent(t, `not json at all`),
	)

	_, err := fixture.planner.Plan(context.Background(), "紛失した在庫を見せて", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), nil)
	require.Error(t, err)
	assert.Len(t, fixture.requests, 2)
}

func TestPlanRetriesOnceOnUnparseableJSON(t *testing.T) {
	fixture := newPlanner(t, fixtureCatalog(),
		chatContent(t, `this is not json`),
		chatContent(t, `{"kind":"none"}`),
	)

	decision, err := fixture.planner.Plan(context.Background(), "何か", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), nil)
	require.NoError(t, err)
	assert.Equal(t, usecase.DecisionNone, decision.Kind)
	assert.Len(t, fixture.requests, 2)
}

// TestPlanRetriesOnceOnATruncatedAnswer is defect 3 (measured 2026-09-15,
// docs/specs/shortlisting.md; see chat.MaxTokens's own doc comment for the
// reproduction): a finish_reason "length" answer is fed into the same
// retry path as an unparseable one - there is nothing to parse in a
// truncated answer either - and a clean second answer still produces a
// real decision.
func TestPlanRetriesOnceOnATruncatedAnswer(t *testing.T) {
	fixture := newPlanner(t, fixtureCatalog(),
		chatTruncatedContent(t, `{"kind":"call","service":"inventory","operationId":"ListInvent`),
		chatContent(t, `{"kind":"none"}`),
	)

	decision, err := fixture.planner.Plan(context.Background(), "明細を1件確認したい", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), nil)
	require.NoError(t, err)
	assert.Equal(t, usecase.DecisionNone, decision.Kind)
	assert.Len(t, fixture.requests, 2)
}

// TestPlanOnTwoTruncatedAnswersInARowReturnsDecisionNoneNotAnError is
// defect 3's other half: unlike TestPlanGivesUpAfterASecondBadAnswer (a
// genuinely invalid answer twice, which still surfaces as an error), a
// model that never got the budget to finish on either attempt produced no
// usable decision, not a bug worth reporting as one - it must degrade to
// usecase.DecisionNone, never a 500.
func TestPlanOnTwoTruncatedAnswersInARowReturnsDecisionNoneNotAnError(t *testing.T) {
	fixture := newPlanner(t, fixtureCatalog(),
		chatTruncatedContent(t, `{"kind":"call","service":"inventory","operationId":"ListInvent`),
		chatTruncatedContent(t, `{"kind":"call","service":"inventory","operationId":"ListInvent`),
	)

	decision, err := fixture.planner.Plan(context.Background(), "明細を1件確認したい", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), nil)
	require.NoError(t, err)
	assert.Equal(t, usecase.DecisionNone, decision.Kind)
	assert.Len(t, fixture.requests, 2)
}

func TestPlanSendsAnswersAlongsideTheQuery(t *testing.T) {
	fixture := newPlanner(t, fixtureCatalog(), chatContent(t,
		`{"kind":"call","service":"inventory","operationId":"ListInventoryItems","args":{"status":"quarantined"}}`,
	))

	answers := []usecase.Answer{{Param: "status", Value: "quarantined"}}

	_, err := fixture.planner.Plan(context.Background(), "破損した在庫を見せて", answers, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), nil)
	require.NoError(t, err)

	messages, ok := fixture.requests[0]["messages"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, messages)

	last, ok := messages[len(messages)-1].(map[string]any)
	require.True(t, ok)

	content, ok := last["content"].(string)
	require.True(t, ok)
	assert.Contains(t, content, "破損した在庫を見せて")
	assert.Contains(t, content, "status")
	assert.Contains(t, content, "quarantined")
}

// TestPlanRendersTheCatalogueAndSetsResponseFormat asserts the request
// carries a response_format JSON schema and that the rendered catalogue
// text - operation id, service and the enum's Japanese label - reaches the
// system prompt, per docs/plans/orchestration.md Task 11.
func TestPlanRendersTheCatalogueAndSetsResponseFormat(t *testing.T) {
	fixture := newPlanner(t, fixtureCatalog(), chatContent(t, `{"kind":"none"}`))

	_, err := fixture.planner.Plan(context.Background(), "何か", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), nil)
	require.NoError(t, err)

	body := fixture.requests[0]

	assert.NotNil(t, body["response_format"])

	messages, ok := body["messages"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, messages)

	system, ok := messages[0].(map[string]any)
	require.True(t, ok)

	content, ok := system["content"].(string)
	require.True(t, ok)
	assert.Contains(t, content, "ListInventoryItems")
	assert.Contains(t, content, "inventory")
	assert.Contains(t, content, "検品保留")
}

// TestPlanWithNoMaxTokensSends1024 documents that an option-less Planner
// (today's behaviour) sends max_tokens: 1024, chat.MaxTokens's own fixed
// value - unchanged since ORCHESTRA_PLANNER_MAX_TOKENS started overriding
// it.
func TestPlanWithNoMaxTokensSends1024(t *testing.T) {
	fixture := newPlanner(t, fixtureCatalog(), chatContent(t, `{"kind":"none"}`))

	_, err := fixture.planner.Plan(context.Background(), "何か", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), nil)
	require.NoError(t, err)

	assert.InDelta(t, 1024.0, fixture.requests[0]["max_tokens"], 0)
}

// TestPlanWithMaxTokensOverridesTheDefault documents jsonmode.WithMaxTokens
// - toolcall.WithMaxTokens's equivalent for this planner (measured
// 2026-09-19, docs/specs/shortlisting.md).
func TestPlanWithMaxTokensOverridesTheDefault(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(chatContent(t, `{"kind":"none"}`))); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := chat.New(chat.Config{BaseURL: server.URL, Model: "test-model"})
	planner := jsonmode.New(client, fixtureCatalog(), jsonmode.WithMaxTokens(4000))

	_, err := planner.Plan(context.Background(), "何か", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), nil)
	require.NoError(t, err)

	assert.InDelta(t, 4000.0, gotBody["max_tokens"], 0)
}

// TestPlanFallsBackWithoutResponseFormatWhenTheEndpointRejectsIt covers the
// "falls back to asking for JSON in the prompt when it does not [accept
// response_format]" half of Task 11: an endpoint that 400s on a request
// carrying response_format is retried once, bare, and that plain request
// still succeeds.
func TestPlanFallsBackWithoutResponseFormatWhenTheEndpointRejectsIt(t *testing.T) {
	var gotBodies []map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var decoded map[string]any
		if err := json.NewDecoder(r.Body).Decode(&decoded); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		gotBodies = append(gotBodies, decoded)

		if decoded["response_format"] != nil {
			w.WriteHeader(http.StatusBadRequest)

			if _, err := w.Write([]byte(`{"error":"response_format not supported"}`)); err != nil {
				t.Errorf("writing fixture error body: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(chatContent(t, `{"kind":"none"}`))); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := chat.New(chat.Config{BaseURL: server.URL, Model: "test-model"})
	planner := jsonmode.New(client, fixtureCatalog())

	decision, err := planner.Plan(context.Background(), "何か", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), nil)
	require.NoError(t, err)
	assert.Equal(t, usecase.DecisionNone, decision.Kind)

	require.Len(t, gotBodies, 2)
	assert.NotNil(t, gotBodies[0]["response_format"])
	assert.Nil(t, gotBodies[1]["response_format"])
}

// fixtureTurns is one earlier turn: the question a person asked and what
// the platform decided for it, in the shape docs/specs/context.md section 3
// describes - the same fixture toolcall's own tests use (planner_test.go,
// sibling package), kept here too since the two test files do not import
// each other.
func fixtureTurns() []usecase.Turn {
	return []usecase.Turn{
		{
			Question:    "検品保留の在庫を見せて",
			Kind:        usecase.ResultKindResult,
			Service:     "inventory",
			OperationID: "ListInventoryItems",
			Args:        map[string]any{"status": "quarantined"},
		},
	}
}

// systemContent reads request's system message (messages[0]) content.
func systemContent(t *testing.T, request map[string]any) string {
	t.Helper()

	messages, ok := request["messages"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, messages)

	system, ok := messages[0].(map[string]any)
	require.True(t, ok)

	content, ok := system["content"].(string)
	require.True(t, ok)

	return content
}

// TestPlanRendersTurnsAfterTheCatalogueInTheSystemPrompt is
// docs/plans/context.md Task 2 Step 3: the same turns toolcall renders,
// rendered here too, but appended after the already-rendered catalogue
// text in the one system message this planner sends (M3,
// docs/specs/context.md section 4) rather than as a separate message.
func TestPlanRendersTurnsAfterTheCatalogueInTheSystemPrompt(t *testing.T) {
	fixture := newPlanner(t, fixtureCatalog(), chatContent(t, `{"kind":"none"}`))

	_, err := fixture.planner.Plan(
		context.Background(), "勤怠でも同じことして", nil, fixtureTurns(), usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}),
		nil,
	)
	require.NoError(t, err)

	content := systemContent(t, fixture.requests[0])

	catalogueIndex := strings.Index(content, "ListInventoryItems")
	turnsIndex := strings.Index(content, "検品保留の在庫を見せて")

	require.NotEqual(t, -1, catalogueIndex, "catalogue must be rendered")
	require.NotEqual(t, -1, turnsIndex, "turns must be rendered")
	assert.Less(t, catalogueIndex, turnsIndex, "turns must come after the catalogue, never before it")

	assert.Contains(t, content, "inventory")
	assert.Contains(t, content, "quarantined")
}

// TestPlanRendersNoRowOfAnyPreviousAnswer is AC-M-103: usecase.Turn has no
// field for the answer's own data, so renderTurns cannot draw one from it
// - this pins the exact text appended after the catalogue, so a future
// change to renderTurns that tried to add anything beyond question, kind,
// service, operationId and args would fail this test rather than slip
// through unnoticed.
func TestPlanRendersNoRowOfAnyPreviousAnswer(t *testing.T) {
	fixture := newPlanner(t, fixtureCatalog(), chatContent(t, `{"kind":"none"}`))

	_, err := fixture.planner.Plan(
		context.Background(), "勤怠でも同じことして", nil, fixtureTurns(), usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}),
		nil,
	)
	require.NoError(t, err)

	content := systemContent(t, fixture.requests[0])

	wantSuffix := "\n\nHere is the conversation so far, oldest first. Each line is a question the user " +
		"already asked and what the platform decided to do about it - service, operation and arguments, " +
		"never the data the operation returned. Use it only to understand what \"it\", \"the same thing\" " +
		"or an unnamed service in the new question below refers to.\n" +
		`- question: "検品保留の在庫を見せて", kind=result, service=inventory, operationId=ListInventoryItems, ` +
		`args={"status":"quarantined"}`

	assert.True(t, strings.HasSuffix(content, wantSuffix), "content: %s", content)
}

// TestPlanRendersTheCataloguePrefixByteIdenticallyWithAndWithoutTurns is
// AC-M-102's equivalent for this planner: tools only ever selects between
// the two fixed system prompt variants New precomputed (offering.md, O2),
// so the invariant M3 protects here is that the rendered catalogue - the
// prefix a prompt cache would be warm for - is exactly the same bytes
// whether or not turns is empty; systemPromptFor only ever appends after
// it, never rewrites it.
func TestPlanRendersTheCataloguePrefixByteIdenticallyWithAndWithoutTurns(t *testing.T) {
	fixtureWithoutTurns := newPlanner(t, fixtureCatalog(), chatContent(t, `{"kind":"none"}`))
	fixtureWithTurns := newPlanner(t, fixtureCatalog(), chatContent(t, `{"kind":"none"}`))

	_, err := fixtureWithoutTurns.planner.Plan(
		context.Background(), "在庫について教えて", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}),
		nil,
	)
	require.NoError(t, err)

	_, err = fixtureWithTurns.planner.Plan(
		context.Background(), "在庫について教えて", nil, fixtureTurns(), usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}),
		nil,
	)
	require.NoError(t, err)

	contentWithoutTurns := systemContent(t, fixtureWithoutTurns.requests[0])
	contentWithTurns := systemContent(t, fixtureWithTurns.requests[0])

	assert.True(t, strings.HasPrefix(contentWithTurns, contentWithoutTurns),
		"the catalogue prefix must be byte-identical with and without turns")
	assert.Greater(t, len(contentWithTurns), len(contentWithoutTurns))
}

// TestPlanDoesNotMentionProposePanelWithNoWorkspace is AC-O-101
// (docs/specs/offering.md) for this planner: when tools carries no
// propose_panel - usecase.ToolsFor's own condition for it, evaluated with
// no workspace id - the system prompt never describes it at all, which is
// what actually stops the model from reaching for it (section 5's third
// exclusion: an absent tool is absent, not disabled and explained).
func TestPlanDoesNotMentionProposePanelWithNoWorkspace(t *testing.T) {
	fixture := newPlanner(t, fixtureCatalog(), chatContent(t, `{"kind":"none"}`))

	_, err := fixture.planner.Plan(
		context.Background(), "何か", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{}),
		nil,
	)
	require.NoError(t, err)

	content := systemContent(t, fixture.requests[0])
	assert.NotContains(t, content, "propose_panel")
}

// TestPlanMentionsProposePanelWithAWorkspace is AC-O-102: a question asked
// from a workspace (tools carries propose_panel) is told about it.
func TestPlanMentionsProposePanelWithAWorkspace(t *testing.T) {
	fixture := newPlanner(t, fixtureCatalog(), chatContent(t, `{"kind":"none"}`))

	_, err := fixture.planner.Plan(
		context.Background(), "何か", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}),
		nil,
	)
	require.NoError(t, err)

	content := systemContent(t, fixture.requests[0])
	assert.Contains(t, content, "propose_panel")
}

// TestPlanRefusesAProposePanelAnswerWithNoWorkspace is AC-O-104 for this
// planner: a model that answers "propose_panel" anyway - despite never
// being told the shape (this stub server just returns it regardless) - is
// refused the same way an unrecognised kind is (ErrUnknownKind), not
// treated as a valid proposal. maxAttempts (2) bounds the retries this
// causes, so Plan gives up rather than looping forever on a model that
// keeps insisting.
func TestPlanRefusesAProposePanelAnswerWithNoWorkspace(t *testing.T) {
	fixture := newPlanner(t, fixtureCatalog(), chatContent(t,
		`{"kind":"propose_panel","service":"inventory","operationId":"ListInventoryItems","args":{}}`,
	))

	_, err := fixture.planner.Plan(
		context.Background(), "在庫を置いて", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{}),
		nil,
	)

	require.Error(t, err)
	require.ErrorIs(t, err, jsonmode.ErrUnknownKind)
}
