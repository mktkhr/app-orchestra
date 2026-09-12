package toolcall_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

	decision, err := planner.Plan(context.Background(), "検品保留の在庫を見せて", nil, nil, usecase.ToolsFor(fixtureCatalog()))
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

	decision, err := planner.Plan(context.Background(), "破損した在庫はある？", nil, nil, usecase.ToolsFor(fixtureCatalog()))
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

	decision, err := planner.Plan(context.Background(), "在庫について、どういう操作ができる？", nil, nil, usecase.ToolsFor(fixtureCatalog()))
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

	decision, err := planner.Plan(context.Background(), "何ができるの？", nil, nil, usecase.ToolsFor(fixtureCatalog()))
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

	decision, err := planner.Plan(context.Background(), "今日の天気は？", nil, nil, usecase.ToolsFor(fixtureCatalog()))
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

	_, err := planner.Plan(context.Background(), "何か", nil, nil, usecase.ToolsFor(fixtureCatalog()))
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

	decision, err := planner.Plan(context.Background(), "何か", nil, nil, usecase.ToolsFor(catalog))
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

	_, err := planner.Plan(context.Background(), "破損した在庫を見せて", answers, nil, usecase.ToolsFor(fixtureCatalog()))
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

	_, err := planner.Plan(context.Background(), "在庫を見せて", nil, nil, usecase.ToolsFor(fixtureCatalog()))
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

// captureBody starts a stub server that answers every request with
// callResponse and records each request's raw body (for byte comparisons)
// alongside its decoded form (for content assertions). Every test below
// happens to only need a DecisionCall back, so it is always callResponse -
// not a parameter, because golangci-lint's unparam check (part of the
// fixed harness policy) rejects a parameter no caller ever varies.
type capturedRequest struct {
	raw     []byte
	decoded map[string]any
}

func captureBody(t *testing.T) (*httptest.Server, *[]capturedRequest) {
	t.Helper()

	var requests []capturedRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading request body: %v", err)
		}

		var decoded map[string]any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		requests = append(requests, capturedRequest{raw: raw, decoded: decoded})

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(callResponse)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	return server, &requests
}

// fixtureTurns is one earlier turn: the question a person asked and what
// the platform decided for it, in the shape docs/specs/context.md section 3
// describes. Its Kind is usecase.ResultKindResult, not any usecase.DecisionKind
// - a Turn records what /api/plan answered, per usecase.Turn's own doc
// comment.
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

// TestPlanRendersTurnsInTheUserMessageBeforeTheQuestion is
// docs/plans/context.md Task 2 Step 1: the request's messages carry the
// earlier question and the operation it resolved to, ahead of the current
// question in the one user message - never as a second "system"-role
// message. That would have been the more obvious split (a message per
// concern), but qwen3.5's chat template - the model this platform runs
// against - rejects a "system" message anywhere but the very first one
// ("System message must be at the beginning"), found by hand running this
// exact request against the real /api/plan endpoint; see buildMessages's
// doc comment.
func TestPlanRendersTurnsInTheUserMessageBeforeTheQuestion(t *testing.T) {
	server, requests := captureBody(t)

	client := chat.New(chat.Config{BaseURL: server.URL, Model: "test-model"})
	planner := toolcall.New(client, fixtureCatalog())

	_, err := planner.Plan(context.Background(), "勤怠でも同じことして", nil, fixtureTurns(), usecase.ToolsFor(fixtureCatalog()))
	require.NoError(t, err)

	require.Len(t, *requests, 1)

	messages, ok := (*requests)[0].decoded["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 2, "system prompt, then one user message carrying the turns and the question")

	system, ok := messages[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "system", system["role"])

	question, ok := messages[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "user", question["role"])

	content, ok := question["content"].(string)
	require.True(t, ok)
	assert.Contains(t, content, "検品保留の在庫を見せて")
	assert.Contains(t, content, "inventory")
	assert.Contains(t, content, "ListInventoryItems")
	assert.Contains(t, content, "quarantined")

	turnsIndex := strings.Index(content, "検品保留の在庫を見せて")
	questionIndex := strings.Index(content, "勤怠でも同じことして")
	require.NotEqual(t, -1, turnsIndex)
	require.NotEqual(t, -1, questionIndex)
	assert.Less(t, turnsIndex, questionIndex, "the turns must come before the current question")
}

// TestPlanRendersNoRowOfAnyPreviousAnswer is AC-M-103: usecase.Turn has no
// field for the answer's own data, so renderTurns cannot draw one from it
// - this pins the exact rendered shape (question, kind, service,
// operationId, args and nothing else) so a future change to renderTurns
// that tried to add anything beyond those fields would fail this test
// rather than slip through unnoticed.
func TestPlanRendersNoRowOfAnyPreviousAnswer(t *testing.T) {
	server, requests := captureBody(t)

	client := chat.New(chat.Config{BaseURL: server.URL, Model: "test-model"})
	planner := toolcall.New(client, fixtureCatalog())

	_, err := planner.Plan(context.Background(), "勤怠でも同じことして", nil, fixtureTurns(), usecase.ToolsFor(fixtureCatalog()))
	require.NoError(t, err)

	messages, ok := (*requests)[0].decoded["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 2)

	question, ok := messages[1].(map[string]any)
	require.True(t, ok)

	content, ok := question["content"].(string)
	require.True(t, ok)

	assert.Equal(t,
		"Here is the conversation so far, oldest first. Each line is a question the user "+
			"already asked and what the platform decided to do about it - service, operation and arguments, "+
			"never the data the operation returned. Use it only to understand what \"it\", \"the same thing\" "+
			"or an unnamed service in the new question below refers to.\n"+
			`- question: "検品保留の在庫を見せて", kind=result, service=inventory, operationId=ListInventoryItems, `+
			`args={"status":"quarantined"}`+
			"\n\n勤怠でも同じことして",
		content,
	)
}

// TestPlanOffersByteIdenticalToolsWithAndWithoutTurns is AC-M-102: the
// turns given to Plan must never change tools, which is what the prompt
// cache is kept warm for (M3, docs/specs/context.md section 4). Comparing
// the raw request bytes, not a re-encoded/decoded form, is deliberate:
// re-marshaling map[string]any always sorts keys and would hide a change
// in what was actually sent over the wire.
func TestPlanOffersByteIdenticalToolsWithAndWithoutTurns(t *testing.T) {
	serverWithoutTurns, requestsWithoutTurns := captureBody(t)
	serverWithTurns, requestsWithTurns := captureBody(t)

	clientWithoutTurns := chat.New(chat.Config{BaseURL: serverWithoutTurns.URL, Model: "test-model"})
	plannerWithoutTurns := toolcall.New(clientWithoutTurns, fixtureCatalog())

	clientWithTurns := chat.New(chat.Config{BaseURL: serverWithTurns.URL, Model: "test-model"})
	plannerWithTurns := toolcall.New(clientWithTurns, fixtureCatalog())

	_, err := plannerWithoutTurns.Plan(context.Background(), "在庫を見せて", nil, nil, usecase.ToolsFor(fixtureCatalog()))
	require.NoError(t, err)

	_, err = plannerWithTurns.Plan(
		context.Background(), "在庫を見せて", nil, fixtureTurns(), usecase.ToolsFor(fixtureCatalog()),
	)
	require.NoError(t, err)

	toolsWithoutTurns := extractToolsRaw(t, (*requestsWithoutTurns)[0].raw)
	toolsWithTurns := extractToolsRaw(t, (*requestsWithTurns)[0].raw)

	assert.True(t, bytes.Equal(toolsWithoutTurns, toolsWithTurns),
		"tools bytes differ:\nwithout turns: %s\nwith turns:    %s", toolsWithoutTurns, toolsWithTurns)
}

// extractToolsRaw decodes raw only as far as the "tools" field, keeping
// its own bytes exactly as they arrived on the wire.
func extractToolsRaw(t *testing.T, raw []byte) []byte {
	t.Helper()

	var wire struct {
		Tools json.RawMessage `json:"tools"`
	}

	require.NoError(t, json.Unmarshal(raw, &wire))
	require.NotEmpty(t, wire.Tools)

	return wire.Tools
}
