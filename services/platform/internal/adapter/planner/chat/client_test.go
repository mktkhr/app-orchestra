package chat_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/chat"
)

// canned is the fixed chat-completions response every fixture server in
// this file answers with: one assistant message carrying a single tool
// call, shaped exactly like the OpenAI chat-completions wire format.
const canned = `{
  "choices": [
    {
      "finish_reason": "tool_calls",
      "message": {
        "role": "assistant",
        "tool_calls": [
          {
            "id": "call_1",
            "type": "function",
            "function": {"name": "ListInventoryItems", "arguments": "{\"status\":\"quarantined\"}"}
          }
        ]
      }
    }
  ]
}`

func TestCompleteSendsModelMessagesAndTools(t *testing.T) {
	var gotBody map[string]any
	var gotAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")

		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(canned)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := chat.New(chat.Config{BaseURL: server.URL + "/v1", APIKey: "test-key", Model: "gemma4-26b-a4b-qat"})

	req := chat.Request{
		Messages: []chat.Message{
			{Role: "system", Content: "you are a planner"},
			{Role: "user", Content: "検品保留の在庫を見せて"},
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
		Temperature: chat.Zero(),
	}

	resp, err := client.Complete(t.Context(), &req)
	require.NoError(t, err)

	assert.Equal(t, "Bearer test-key", gotAuth)
	assert.Equal(t, "gemma4-26b-a4b-qat", gotBody["model"])

	messages, ok := gotBody["messages"].([]any)
	require.True(t, ok)
	assert.Len(t, messages, 2)

	tools, ok := gotBody["tools"].([]any)
	require.True(t, ok)
	assert.Len(t, tools, 1)

	// Defect 2 (docs/specs/shortlisting.md, measured 2026-09-15): with no
	// temperature on the wire, llama-server's own default applied and the
	// same 100-question fixture scored 32 and then 16 on one axis across
	// two runs, nothing else changed. Every planning call now fixes it at
	// 0 explicitly.
	assert.InDelta(t, 0.0, gotBody["temperature"], 0)

	require.Len(t, resp.Message.ToolCalls, 1)
	assert.Equal(t, "ListInventoryItems", resp.Message.ToolCalls[0].Function.Name)
	assert.JSONEq(t, `{"status":"quarantined"}`, resp.Message.ToolCalls[0].Function.Arguments)
	assert.Equal(t, "tool_calls", resp.FinishReason)
}

// TestCompleteWithNoTemperatureOmitsItFromTheWireBody documents the other
// half of Request.Temperature's contract: a Request that leaves it nil
// (the zero value, distinct from "explicitly 0") sends no "temperature"
// field at all, so a caller that genuinely wants the endpoint's own
// default still can.
func TestCompleteWithNoTemperatureOmitsItFromTheWireBody(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(canned)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := chat.New(chat.Config{BaseURL: server.URL + "/v1", Model: "gemma4-26b-a4b-qat"})

	_, err := client.Complete(t.Context(), &chat.Request{
		Messages: []chat.Message{{Role: "user", Content: "検品保留の在庫を見せて"}},
	})
	require.NoError(t, err)

	_, ok := gotBody["temperature"]
	assert.False(t, ok, "temperature must be absent from the wire body, not sent as 0, when Request leaves it nil")
}

func TestCompleteWithoutAPIKeySendsNoAuthorizationHeader(t *testing.T) {
	var gotAuth string
	sawAuth := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, sawAuth = r.Header.Get("Authorization"), r.Header.Get("Authorization") != ""

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(canned)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := chat.New(chat.Config{BaseURL: server.URL, Model: "m"})

	_, err := client.Complete(t.Context(), &chat.Request{Messages: []chat.Message{{Role: "user", Content: "hi"}}})
	require.NoError(t, err)

	assert.False(t, sawAuth, "unexpected Authorization header: %q", gotAuth)
}

func TestCompleteReturnsErrorOnNonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)

		if _, err := w.Write([]byte(`{"error":"boom"}`)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := chat.New(chat.Config{BaseURL: server.URL, Model: "m"})

	_, err := client.Complete(t.Context(), &chat.Request{Messages: []chat.Message{{Role: "user", Content: "hi"}}})
	require.Error(t, err)
}

func TestCompleteReturnsErrorWhenNoChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(`{"choices": []}`)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := chat.New(chat.Config{BaseURL: server.URL, Model: "m"})

	_, err := client.Complete(t.Context(), &chat.Request{Messages: []chat.Message{{Role: "user", Content: "hi"}}})
	require.Error(t, err)
}

func TestCompleteRequestModelOverridesConfigModel(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(canned)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := chat.New(chat.Config{BaseURL: server.URL, Model: "default-model"})

	_, err := client.Complete(t.Context(), &chat.Request{
		Model:    "override-model",
		Messages: []chat.Message{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)

	assert.Equal(t, "override-model", gotBody["model"])
}

func TestCompleteReturnsErrorWhenRequestCannotBeEncoded(t *testing.T) {
	client := chat.New(chat.Config{BaseURL: "http://example.invalid", Model: "m"})

	req := chat.Request{
		Messages: []chat.Message{{Role: "user", Content: "hi"}},
		Tools: []chat.ToolDefinition{
			{
				Type: "function",
				// A channel has no JSON representation: this exercises
				// Complete's own encoding failure, distinct from a
				// transport failure or a bad response.
				Function: chat.FunctionDefinition{Name: "f", Parameters: map[string]any{"bad": make(chan int)}},
			},
		},
	}

	_, err := client.Complete(t.Context(), &req)
	require.Error(t, err)
}

func TestCompleteReturnsErrorWhenTheRequestCannotBeBuilt(t *testing.T) {
	// A raw control byte in the URL fails net/http's own request
	// construction (url.Parse), before anything is ever sent.
	client := chat.New(chat.Config{BaseURL: "http://example.invalid/\x7f", Model: "m"})

	_, err := client.Complete(t.Context(), &chat.Request{Messages: []chat.Message{{Role: "user", Content: "hi"}}})
	require.Error(t, err)
}

func TestCompleteReturnsErrorWhenTheContextIsAlreadyDone(t *testing.T) {
	client := chat.New(chat.Config{BaseURL: "http://example.invalid", Model: "m"})

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := client.Complete(ctx, &chat.Request{Messages: []chat.Message{{Role: "user", Content: "hi"}}})
	require.Error(t, err)
}
