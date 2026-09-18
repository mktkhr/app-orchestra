package app_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/pkg/app"
)

// fixtureAnthropicServer answers every POST /v1/messages with one
// tool_use block naming ListWidgets - the Messages API shape of the same
// canned answer fixtureChatServer gives the OpenAI-compatible transport,
// so TestNewWithLLMProviderAnthropicUsesTheToolCallingPlanner exercises
// the exact same "widgets please" -> ResultKindResult/table path, proving
// toolcall.Planner runs unchanged against either backend.
func fixtureAnthropicServer(t *testing.T) *httptest.Server {
	t.Helper()

	const response = `{
    "content": [{"type": "tool_use", "id": "toolu_1", "name": "ListWidgets", "input": {}}],
    "stop_reason": "tool_use",
    "usage": {"input_tokens": 12, "output_tokens": 4}
  }`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/messages", r.URL.Path)
		assert.Equal(t, "test-key", r.Header.Get("X-Api-Key"))
		assert.Equal(t, "2023-06-01", r.Header.Get("Anthropic-Version"))

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(response)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	return server
}

// TestNewWithLLMProviderAnthropicUsesTheToolCallingPlanner proves
// ORCHESTRA_LLM_PROVIDER=anthropic (app.ProviderAnthropic) reaches the
// same toolcall.Planner as the default provider, wired over
// internal/adapter/planner/chat/anthropic.Client instead of
// internal/adapter/planner/chat.Client - the same shape
// TestNewWithLLMBaseURLConfiguredUsesTheToolCallingPlanner (app_test.go)
// already proves for the default. LLM.BaseURL is deliberately left empty:
// the Anthropic backend needs none of its own (llmConfigured's own doc
// comment, app.go) - AnthropicBaseURL, not BaseURL, points this test at
// the fixture server instead of the real Messages API.
func TestNewWithLLMProviderAnthropicUsesTheToolCallingPlanner(t *testing.T) {
	fixture := fixtureService(t)
	anthropicServer := fixtureAnthropicServer(t)

	handler, err := app.New(&app.Config{
		Services: []app.Service{{Name: "fixture", URL: fixture.URL}},
		LLM: app.LLM{
			Model: "claude-haiku-4-5", Provider: app.ProviderAnthropic,
			AnthropicAPIKey: "test-key", AnthropicBaseURL: anthropicServer.URL,
		},
		DBPath:        filepath.Join(t.TempDir(), "app.db"),
		AdminPassword: appTestAdminPassword,
	})
	require.NoError(t, err)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	signInTestAdmin(t, server)

	raw, err := json.Marshal(map[string]string{"query": "widgets please"})
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/api/plan", bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Kind      string `json:"kind"`
		Component string `json:"component"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "result", body.Kind)
	assert.Equal(t, "table", body.Component)
}

// TestNewRejectsAnUnknownLLMProvider mirrors TestNewRejectsAnUnknownPicker
// (app_picker_test.go): a typo'd LLM.Provider fails app.New itself, not
// just internal/infra/config.Load - the same defence-in-depth
// ErrInvalidLLMMode already documents.
func TestNewRejectsAnUnknownLLMProvider(t *testing.T) {
	_, err := app.New(&app.Config{
		LLM:           app.LLM{Provider: "bedrock", Model: "some-model"},
		DBPath:        filepath.Join(t.TempDir(), "app.db"),
		AdminPassword: appTestAdminPassword,
	})

	require.Error(t, err)
	require.ErrorIs(t, err, app.ErrInvalidLLMProvider)
}

// TestNewRejectsLLMProviderAnthropicWithoutAnAPIKey mirrors
// TestNewRejectsPickerJevWithoutAnAPIKey (app_picker_test.go).
func TestNewRejectsLLMProviderAnthropicWithoutAnAPIKey(t *testing.T) {
	_, err := app.New(&app.Config{
		LLM:           app.LLM{Provider: app.ProviderAnthropic, Model: "claude-haiku-4-5"},
		DBPath:        filepath.Join(t.TempDir(), "app.db"),
		AdminPassword: appTestAdminPassword,
	})

	require.Error(t, err)
	require.ErrorIs(t, err, app.ErrMissingAnthropicAPIKey)
}

// TestNewWithLLMProviderLlamaSwapAndNoBaseURLFallsBackToTheStub is a
// regression test: internal/infra/config.Load never actually leaves
// LLMProvider empty - ORCHESTRA_LLM_PROVIDER unset still parses to
// LLMProviderLlamaSwap - so cmd/api always passes a non-empty
// LLM.Provider through even when ORCHESTRA_LLM_BASE_URL is unset (no LLM
// configured at all, production's default). llmConfigured must still fall
// back to the stub here, exactly as it did before Provider existed - an
// earlier version of this function treated any non-empty Provider as
// "configured" and broke every deployment with no base URL set
// (acceptance-e2e's stub-planner suites, caught by `make check`).
func TestNewWithLLMProviderLlamaSwapAndNoBaseURLFallsBackToTheStub(t *testing.T) {
	fixture := fixtureService(t)

	handler, err := app.New(&app.Config{
		Services: []app.Service{{Name: "fixture", URL: fixture.URL}},
		LLM:      app.LLM{Provider: app.ProviderLlamaSwap},
		PlanFixtures: []app.PlanFixture{
			{Query: "widgets please", Service: "fixture", OperationID: "ListWidgets"},
		},
		DBPath:        filepath.Join(t.TempDir(), "app.db"),
		AdminPassword: appTestAdminPassword,
	})
	require.NoError(t, err)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	signInTestAdmin(t, server)

	raw, err := json.Marshal(map[string]string{"query": "widgets please"})
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/api/plan", bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	// If newPlanner had instead built a real chat.Client against an empty
	// base URL (the bug this test guards), Orchestrator.Plan would have
	// called it and 500'd; the stub answers the fixture's own table entry
	// instead, exactly as TestNewWiresThePlanFixtureThroughToAResult
	// (app_test.go) proves for LLM.Provider left unset entirely.
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Kind      string `json:"kind"`
		Component string `json:"component"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "result", body.Kind)
	assert.Equal(t, "table", body.Component)
}

// TestNewWithAnthropicThinkingOmitsTheDisableField proves LLM.AnthropicThinking
// (ORCHESTRA_ANTHROPIC_THINKING=on, config.Config.AnthropicThinking)
// reaches internal/adapter/planner/chat/anthropic.Client's own wire body
// through pkg/app.newChatCompleter, not just anthropic.Config directly
// (that package's own client_test.go already proves the field works in
// isolation) - claude-sonnet-5 must send no "thinking" field at all.
func TestNewWithAnthropicThinkingOmitsTheDisableField(t *testing.T) {
	fixture := fixtureService(t)

	var gotBody map[string]any

	anthropicServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		const response = `{
      "content": [{"type": "tool_use", "id": "toolu_1", "name": "ListWidgets", "input": {}}],
      "stop_reason": "tool_use",
      "usage": {"input_tokens": 12, "output_tokens": 4}
    }`
		if _, err := w.Write([]byte(response)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(anthropicServer.Close)

	handler, err := app.New(&app.Config{
		Services: []app.Service{{Name: "fixture", URL: fixture.URL}},
		LLM: app.LLM{
			Model: "claude-sonnet-5", Provider: app.ProviderAnthropic,
			AnthropicAPIKey: "test-key", AnthropicBaseURL: anthropicServer.URL, AnthropicThinking: true,
		},
		DBPath:        filepath.Join(t.TempDir(), "app.db"),
		AdminPassword: appTestAdminPassword,
	})
	require.NoError(t, err)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	signInTestAdmin(t, server)

	raw, err := json.Marshal(map[string]string{"query": "widgets please"})
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/api/plan", bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	_, hasThinking := gotBody["thinking"]
	assert.False(t, hasThinking, "AnthropicThinking=true must omit \"thinking\" for claude-sonnet-5")
}

// LLM.Provider left unset (the zero value, "") taking the same
// ProviderLlamaSwap branch as before this subproject existed is already
// covered end to end by TestNewWithLLMBaseURLConfiguredUsesTheToolCallingPlanner
// (app_test.go), which never sets Provider at all - a second copy of that
// same request/response flow here would only be dupl (harness/quality/go/golangci.yml).
