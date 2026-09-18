package anthropic_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/chat"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/chat/anthropic"
)

// decodeBody decodes raw as a JSON object - the shared helper both
// client_test.go and params_test.go use to inspect a marshalled request
// body without sending it (they never do: every test in this file uses a
// local httptest fixture, never api.anthropic.com).
func decodeBody(t *testing.T, raw []byte) map[string]any {
	t.Helper()

	var body map[string]any

	require.NoError(t, json.Unmarshal(raw, &body))

	return body
}

// textResponse is a fixture Messages API response carrying one "text"
// block, mirroring pick.Picker's own plain-content answer shape.
const textResponse = `{
  "content": [{"type": "text", "text": "op_list_inventory_items"}],
  "stop_reason": "end_turn",
  "usage": {"input_tokens": 123, "output_tokens": 7}
}`

// toolUseResponse is a fixture Messages API response carrying one
// "tool_use" block, mirroring toolcall.Planner's own tool-call answer
// shape - the OpenAI wire format's "choices[0].message.tool_calls".
const toolUseResponse = `{
  "content": [{"type": "tool_use", "id": "toolu_1", "name": "ListInventoryItems", "input": {"status": "quarantined"}}],
  "stop_reason": "tool_use",
  "usage": {"input_tokens": 456, "output_tokens": 12}
}`

func TestCompleteSendsSystemMessagesToolsAndHeaders(t *testing.T) {
	var gotBody map[string]any

	var gotHeaders http.Header

	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		gotPath = r.URL.Path

		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(textResponse)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := anthropic.New(anthropic.Config{BaseURL: server.URL, APIKey: "test-key", Model: "claude-haiku-4-5"})

	maxTokens := 1024
	temp := 0.0

	resp, err := client.Complete(t.Context(), &chat.Request{
		Messages: []chat.Message{
			{Role: "system", Content: "you are a planner"},
			{Role: "user", Content: "在庫を見せて"},
		},
		Tools: []chat.ToolDefinition{
			{
				Type: "function",
				Function: chat.FunctionDefinition{
					Name:        "ListInventoryItems",
					Description: "List stock items.",
					Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
				},
			},
		},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	})
	require.NoError(t, err)

	assert.Equal(t, "/v1/messages", gotPath)
	assert.Equal(t, "test-key", gotHeaders.Get("x-api-key"))
	assert.Equal(t, "2023-06-01", gotHeaders.Get("anthropic-version"))
	assert.Equal(t, "application/json", gotHeaders.Get("content-type"))

	assert.Equal(t, "claude-haiku-4-5", gotBody["model"])
	assert.Equal(t, "you are a planner", gotBody["system"], "system message must go on \"system\", not \"messages\"")
	assert.InDelta(t, 1024.0, gotBody["max_tokens"], 0)

	messages, ok := gotBody["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 1, "the system message must not also appear in messages")

	msg, ok := messages[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "user", msg["role"])
	assert.Equal(t, "在庫を見せて", msg["content"])

	tools, ok := gotBody["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)

	tool, ok := tools[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ListInventoryItems", tool["name"])
	assert.Equal(t, "List stock items.", tool["description"])
	_, hasInputSchema := tool["input_schema"]
	assert.True(t, hasInputSchema)
	_, hasType := tool["type"]
	assert.False(t, hasType, "tools must be the Messages API's flat shape, not the OpenAI {type,function} envelope")

	assert.Equal(t, "op_list_inventory_items", resp.Message.Content)
	assert.Equal(t, "end_turn", resp.FinishReason)
}

func TestCompleteMapsAToolUseBlockOntoAToolCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(toolUseResponse)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := anthropic.New(anthropic.Config{BaseURL: server.URL, Model: "claude-haiku-4-5"})

	maxTokens := 1024

	resp, err := client.Complete(t.Context(), &chat.Request{
		Messages:  []chat.Message{{Role: "user", Content: "検品保留の在庫を見せて"}},
		MaxTokens: &maxTokens,
	})
	require.NoError(t, err)

	require.Len(t, resp.Message.ToolCalls, 1)
	assert.Equal(t, "toolu_1", resp.Message.ToolCalls[0].ID)
	assert.Equal(t, "function", resp.Message.ToolCalls[0].Type)
	assert.Equal(t, "ListInventoryItems", resp.Message.ToolCalls[0].Function.Name)
	assert.JSONEq(t, `{"status":"quarantined"}`, resp.Message.ToolCalls[0].Function.Arguments)
	assert.Equal(t, "tool_use", resp.FinishReason)
}

func TestCompleteMapsMaxTokensStopReasonToFinishReasonLength(t *testing.T) {
	const truncated = `{
  "content": [{"type": "text", "text": "partial"}],
  "stop_reason": "max_tokens",
  "usage": {"input_tokens": 10, "output_tokens": 1024}
}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(truncated)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := anthropic.New(anthropic.Config{BaseURL: server.URL, Model: "claude-sonnet-5"})

	maxTokens := 1024

	resp, err := client.Complete(t.Context(), &chat.Request{
		Messages:  []chat.Message{{Role: "user", Content: "hi"}},
		MaxTokens: &maxTokens,
	})
	require.NoError(t, err)

	assert.Equal(t, chat.FinishReasonLength, resp.FinishReason)
	assert.Equal(t, 1024, resp.Usage.CompletionTokens)
}

func TestCompleteReturnsAnErrorNamingARefusal(t *testing.T) {
	const refused = `{
  "content": [{"type": "text", "text": "I can't help with that."}],
  "stop_reason": "refusal",
  "usage": {"input_tokens": 10, "output_tokens": 5}
}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(refused)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := anthropic.New(anthropic.Config{BaseURL: server.URL, Model: "claude-haiku-4-5"})

	maxTokens := 1024

	_, err := client.Complete(t.Context(), &chat.Request{
		Messages:  []chat.Message{{Role: "user", Content: "hi"}},
		MaxTokens: &maxTokens,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, anthropic.ErrRefused)
	assert.Contains(t, err.Error(), "I can't help with that.")
}

func TestCompleteReturnsErrorOnNonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)

		if _, err := w.Write([]byte(
			`{"type":"error","error":{"type":"invalid_request_error","message":"temperature: extra field"}}`,
		)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := anthropic.New(anthropic.Config{BaseURL: server.URL, Model: "claude-sonnet-5"})

	maxTokens := 1024

	_, err := client.Complete(t.Context(), &chat.Request{
		Messages:  []chat.Message{{Role: "user", Content: "hi"}},
		MaxTokens: &maxTokens,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, anthropic.ErrRequestFailed)
}

func TestCompleteReturnsErrorWhenRequestHasNoMaxTokens(t *testing.T) {
	client := anthropic.New(anthropic.Config{BaseURL: "http://example.invalid", Model: "claude-haiku-4-5"})

	_, err := client.Complete(t.Context(), &chat.Request{Messages: []chat.Message{{Role: "user", Content: "hi"}}})
	require.Error(t, err)
	require.ErrorIs(t, err, anthropic.ErrMissingMaxTokens)
}

func TestCompleteReturnsErrorWhenNoModelIsConfigured(t *testing.T) {
	client := anthropic.New(anthropic.Config{BaseURL: "http://example.invalid"})

	maxTokens := 1024

	_, err := client.Complete(t.Context(), &chat.Request{
		Messages:  []chat.Message{{Role: "user", Content: "hi"}},
		MaxTokens: &maxTokens,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, anthropic.ErrMissingModel)
}

func TestCompleteRequestModelOverridesConfigModel(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(textResponse)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := anthropic.New(anthropic.Config{BaseURL: server.URL, Model: "claude-haiku-4-5"})

	maxTokens := 1024

	_, err := client.Complete(t.Context(), &chat.Request{
		Model:     "claude-opus-5",
		Messages:  []chat.Message{{Role: "user", Content: "hi"}},
		MaxTokens: &maxTokens,
	})
	require.NoError(t, err)

	assert.Equal(t, "claude-opus-5", gotBody["model"])
}

// TestBuildRequestBodyMatchesWhatCompleteWouldSend proves the token-count
// helper (client.go's BuildRequestBody) is not a second, drifting
// definition of the wire body: it goes through the exact same
// toWireRequest Complete does, asserted here via the model-parameter table
// test (params_test.go) sharing this same function - this test only
// checks BuildRequestBody never touches the network and returns valid,
// decodable JSON for a real planning-shaped request.
func TestBuildRequestBodyMatchesWhatCompleteWouldSend(t *testing.T) {
	maxTokens := 1024
	temp := 0.0

	body, err := anthropic.BuildRequestBody("claude-sonnet-5", &chat.Request{
		Messages: []chat.Message{
			{Role: "system", Content: "you are a planner"},
			{Role: "user", Content: "hi"},
		},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	})
	require.NoError(t, err)

	decoded := decodeBody(t, body)

	assert.Equal(t, "claude-sonnet-5", decoded["model"])
	assert.InDelta(t, 1024.0, decoded["max_tokens"], 0)
	_, hasTemperature := decoded["temperature"]
	assert.False(t, hasTemperature, "claude-sonnet-5 must never see temperature on the wire")
}
