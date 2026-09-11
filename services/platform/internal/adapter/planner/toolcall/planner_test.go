package toolcall_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/chat"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/toolcall"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// fixtureCatalog mirrors enough of the real inventory/attendance catalogue
// for the mapping tests below: one safe list operation per service, one
// unsafe create, so tests can assert both a resolved Service and the
// operation-id ambiguity case.
func fixtureCatalog() domain.Catalog {
	return domain.Catalog{
		Endpoints: []domain.Endpoint{
			{
				Service:     "inventory",
				OperationID: "ListInventoryItems",
				Method:      domain.MethodGet,
				Path:        "/api/inventory/items",
				Parameters: []domain.Parameter{
					{
						Name:   "status",
						In:     "query",
						Schema: domain.Schema{Type: domain.SchemaTypeString, Enum: []string{"allocated", "quarantined"}},
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
				Response: &domain.Schema{
					Type: domain.SchemaTypeObject,
					Properties: map[string]domain.Schema{
						"records": {Type: domain.SchemaTypeArray, Items: &domain.Schema{Type: domain.SchemaTypeObject}},
					},
				},
			},
			{
				Service:     "inventory",
				OperationID: "CreateInventoryItem",
				Method:      "POST",
				Path:        "/api/inventory/items",
				RequestBody: &domain.Schema{
					Type:       domain.SchemaTypeObject,
					Required:   []string{"name", "quantity"},
					Properties: map[string]domain.Schema{"name": {Type: domain.SchemaTypeString}, "quantity": {Type: domain.SchemaTypeInteger}},
				},
			},
			// Two services declaring the very same operation id: a
			// tool-calling endpoint only ever names the operation, never the
			// service, so this is the one case resolveService cannot tell
			// apart from the tool call alone. See TestPlanOnAmbiguousOperationIDPicksFirstCatalogueMatch.
			{Service: "svc-a", OperationID: "Duplicate", Method: domain.MethodGet, Path: "/a"},
			{Service: "svc-b", OperationID: "Duplicate", Method: domain.MethodGet, Path: "/b"},
		},
	}
}

// stubServer answers every chat-completions request with body, regardless
// of what was asked - the mapping tests below only care about how the
// planner turns a canned response into a Decision, not about the request
// it sent.
func stubServer(t *testing.T, body string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	return server
}

func newPlanner(t *testing.T, body string, catalog domain.Catalog) *toolcall.Planner {
	t.Helper()

	server := stubServer(t, body)
	client := chat.New(chat.Config{BaseURL: server.URL, Model: "test-model"})

	return toolcall.New(client, catalog)
}

const callResponse = `{
  "choices": [{
    "finish_reason": "tool_calls",
    "message": {
      "role": "assistant",
      "tool_calls": [{
        "id": "call_1",
        "type": "function",
        "function": {"name": "ListInventoryItems", "arguments": "{\"status\":\"quarantined\"}"}
      }]
    }
  }]
}`

func TestPlanMapsAToolCallOntoADecisionCallWithItsService(t *testing.T) {
	planner := newPlanner(t, callResponse, fixtureCatalog())

	decision, err := planner.Plan(context.Background(), "検品保留の在庫を見せて", nil, usecase.ToolsFor(fixtureCatalog()))
	require.NoError(t, err)

	assert.Equal(t, usecase.DecisionCall, decision.Kind)
	assert.Equal(t, "inventory", decision.Service)
	assert.Equal(t, "ListInventoryItems", decision.OperationID)
	assert.Equal(t, map[string]any{"status": "quarantined"}, decision.Args)
}

const askResponse = `{
  "choices": [{
    "finish_reason": "tool_calls",
    "message": {
      "role": "assistant",
      "tool_calls": [{
        "id": "call_1",
        "type": "function",
        "function": {
          "name": "ask_user",
          "arguments": "{\"question\":\"どのステータスですか？\",\"service\":\"inventory\",\"operationId\":\"ListInventoryItems\",\"param\":\"status\",\"options\":[{\"value\":\"allocated\",\"label\":\"引当済\"}]}"
        }
      }]
    }
  }]
}`

func TestPlanMapsAskUserOntoADecisionAsk(t *testing.T) {
	planner := newPlanner(t, askResponse, fixtureCatalog())

	decision, err := planner.Plan(context.Background(), "破損した在庫はある？", nil, usecase.ToolsFor(fixtureCatalog()))
	require.NoError(t, err)

	assert.Equal(t, usecase.DecisionAsk, decision.Kind)
	assert.Equal(t, "inventory", decision.Service)
	assert.Equal(t, "ListInventoryItems", decision.OperationID)
	assert.Equal(t, "status", decision.Param)
	assert.Equal(t, "どのステータスですか？", decision.Question)
	require.Len(t, decision.Options, 1)
	assert.Equal(t, domain.Option{Value: "allocated", Label: "引当済"}, decision.Options[0])
}

const listCapabilitiesResponse = `{
  "choices": [{
    "finish_reason": "tool_calls",
    "message": {
      "role": "assistant",
      "tool_calls": [{
        "id": "call_1",
        "type": "function",
        "function": {"name": "list_capabilities", "arguments": "{\"service\":\"inventory\"}"}
      }]
    }
  }]
}`

// TestPlanMapsListCapabilitiesOntoADecisionListCapabilities is the key
// case resolveService must never see: list_capabilities is not an
// operation id any endpoint in fixtureCatalog declares (it is a built-in
// tool, not derived from any spec - usecase.ListCapabilitiesTool), so if
// the planner routed it through decisionFromCall the way it does every
// other tool name, resolveService would fail to find it and this would
// come back as ErrUnknownOperation instead of a decision.
func TestPlanMapsListCapabilitiesOntoADecisionListCapabilities(t *testing.T) {
	planner := newPlanner(t, listCapabilitiesResponse, fixtureCatalog())

	decision, err := planner.Plan(context.Background(), "在庫について、どういう操作ができる？", nil, usecase.ToolsFor(fixtureCatalog()))
	require.NoError(t, err)

	assert.Equal(t, usecase.DecisionListCapabilities, decision.Kind)
	assert.Equal(t, "inventory", decision.Service)
}

const listCapabilitiesNoServiceResponse = `{
  "choices": [{
    "finish_reason": "tool_calls",
    "message": {
      "role": "assistant",
      "tool_calls": [{
        "id": "call_1",
        "type": "function",
        "function": {"name": "list_capabilities", "arguments": "{}"}
      }]
    }
  }]
}`

func TestPlanMapsListCapabilitiesWithNoServiceArgumentToAnEmptyFilter(t *testing.T) {
	planner := newPlanner(t, listCapabilitiesNoServiceResponse, fixtureCatalog())

	decision, err := planner.Plan(context.Background(), "何ができるの？", nil, usecase.ToolsFor(fixtureCatalog()))
	require.NoError(t, err)

	assert.Equal(t, usecase.DecisionListCapabilities, decision.Kind)
	assert.Empty(t, decision.Service)
}

const noToolCallResponse = `{
  "choices": [{
    "finish_reason": "stop",
    "message": {"role": "assistant", "content": "今日の天気は分かりません。"}
  }]
}`

func TestPlanMapsNoToolCallOntoDecisionNone(t *testing.T) {
	planner := newPlanner(t, noToolCallResponse, fixtureCatalog())

	decision, err := planner.Plan(context.Background(), "今日の天気は？", nil, usecase.ToolsFor(fixtureCatalog()))
	require.NoError(t, err)

	assert.Equal(t, usecase.DecisionNone, decision.Kind)
}

func TestPlanOnUnknownOperationReturnsAnError(t *testing.T) {
	response := `{
    "choices": [{
      "finish_reason": "tool_calls",
      "message": {
        "role": "assistant",
        "tool_calls": [{"id": "call_1", "type": "function", "function": {"name": "NoSuchOperation", "arguments": "{}"}}]
      }
    }]
  }`

	planner := newPlanner(t, response, fixtureCatalog())

	_, err := planner.Plan(context.Background(), "何か", nil, usecase.ToolsFor(fixtureCatalog()))
	require.Error(t, err)
}

func TestPlanOnAmbiguousOperationIDPicksFirstCatalogueMatch(t *testing.T) {
	response := `{
    "choices": [{
      "finish_reason": "tool_calls",
      "message": {
        "role": "assistant",
        "tool_calls": [{"id": "call_1", "type": "function", "function": {"name": "Duplicate", "arguments": "{}"}}]
      }
    }]
  }`

	catalog := fixtureCatalog()
	planner := newPlanner(t, response, catalog)

	decision, err := planner.Plan(context.Background(), "何か", nil, usecase.ToolsFor(catalog))
	require.NoError(t, err)

	assert.Equal(t, usecase.DecisionCall, decision.Kind)
	// Deterministic, not correct: see the doc comment on resolveService in
	// planner.go for why the first catalogue match is the accepted answer
	// here, and DECISIONS.md for the tradeoff.
	assert.Equal(t, "svc-a", decision.Service)
}

func TestPlanSendsAnswersAlongsideTheQuery(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(callResponse)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := chat.New(chat.Config{BaseURL: server.URL, Model: "test-model"})
	planner := toolcall.New(client, fixtureCatalog())

	answers := []usecase.Answer{{Param: "status", Value: "quarantined"}}

	_, err := planner.Plan(context.Background(), "破損した在庫を見せて", answers, usecase.ToolsFor(fixtureCatalog()))
	require.NoError(t, err)

	messages, ok := gotBody["messages"].([]any)
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

func TestPlanShapesToolsWithAdditionalPropertiesFalseAndPerToolStrict(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(callResponse)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := chat.New(chat.Config{BaseURL: server.URL, Model: "test-model"})
	planner := toolcall.New(client, fixtureCatalog())

	_, err := planner.Plan(context.Background(), "在庫を見せて", nil, usecase.ToolsFor(fixtureCatalog()))
	require.NoError(t, err)

	tools, ok := gotBody["tools"].([]any)
	require.True(t, ok)

	byName := map[string]map[string]any{}

	for _, raw := range tools {
		tool, tOk := raw.(map[string]any)
		require.True(t, tOk)

		fn, fOk := tool["function"].(map[string]any)
		require.True(t, fOk)

		name, nOk := fn["name"].(string)
		require.True(t, nOk)
		byName[name] = fn
	}

	// ListInventoryItems declares "status" as optional (fixtureCatalog has
	// no Required on that Parameter), so it cannot be strict without
	// inventing a required list the model was never told about - see
	// DECISIONS.md and the doc comment on shapeTool.
	listParams, ok := byName["ListInventoryItems"]["parameters"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, listParams["additionalProperties"])
	assert.Equal(t, false, byName["ListInventoryItems"]["strict"])

	// CreateInventoryItem requires every property it declares, so it stays
	// strict.
	assert.Equal(t, true, byName["CreateInventoryItem"]["strict"])

	// ask_user's own schema (usecase.AskUserTool) requires every property
	// it declares too.
	assert.Equal(t, true, byName["ask_user"]["strict"])
}
