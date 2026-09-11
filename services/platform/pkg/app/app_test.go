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

func TestNewServesHealth(t *testing.T) {
	handler, err := app.New(&app.Config{})
	require.NoError(t, err)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/api/health", http.NoBody)
	require.NoError(t, err)

	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// fixtureSpec is a minimal OpenAPI document for a single safe list
// operation, served by a fixtureService.
const fixtureSpec = `
openapi: 3.0.3
info:
  title: Fixture
  version: 0.1.0
paths:
  /items:
    get:
      operationId: ListWidgets
      summary: List widgets.
      x-orchestra-expose: true
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
                properties:
                  items:
                    type: array
                    items:
                      type: object
`

// fixtureService serves fixtureSpec at /openapi.yaml and a fixed JSON body
// at /items, mirroring a real inventory-shaped service closely enough for
// the orchestrator to plan and invoke against it.
func fixtureService(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/openapi.yaml":
			w.Header().Set("Content-Type", "application/yaml")
			if _, err := w.Write([]byte(fixtureSpec)); err != nil {
				t.Errorf("writing fixture spec: %v", err)
			}
		case "/items":
			w.Header().Set("Content-Type", "application/json")
			if _, err := w.Write([]byte(`{"items":[]}`)); err != nil {
				t.Errorf("writing fixture response: %v", err)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	return server
}

func TestNewWiresThePlanFixtureThroughToAResult(t *testing.T) {
	fixture := fixtureService(t)

	handler, err := app.New(&app.Config{
		Services: []app.Service{{Name: "fixture", URL: fixture.URL}},
		PlanFixtures: []app.PlanFixture{
			{Query: "widgets please", Service: "fixture", OperationID: "ListWidgets"},
		},
	})
	require.NoError(t, err)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

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

// fixtureChatServer answers every chat-completions request with a tool
// call naming ListWidgets, regardless of what tools or messages it was
// sent - just enough to prove app.New wires the tool-calling planner in
// when Config.LLM.BaseURL is set, not to test the planner's own mapping
// (internal/adapter/planner/toolcall has that).
func fixtureChatServer(t *testing.T) *httptest.Server {
	t.Helper()

	const response = `{
    "choices": [{
      "finish_reason": "tool_calls",
      "message": {
        "role": "assistant",
        "tool_calls": [{"id": "call_1", "type": "function", "function": {"name": "ListWidgets", "arguments": "{}"}}]
      }
    }]
  }`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(response)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	return server
}

func TestNewWithLLMBaseURLConfiguredUsesTheToolCallingPlanner(t *testing.T) {
	fixture := fixtureService(t)
	chatServer := fixtureChatServer(t)

	handler, err := app.New(&app.Config{
		Services: []app.Service{{Name: "fixture", URL: fixture.URL}},
		LLM:      app.LLM{BaseURL: chatServer.URL, Model: "test-model"},
		// PlanFixtures is deliberately left empty: if the stub were chosen
		// instead of the tool-calling planner, "widgets please" would come
		// back as ResultKindNone, not the table this test asserts.
	})
	require.NoError(t, err)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

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

// fixtureJSONChatServer answers every chat-completions request with a
// message whose content is the JSON object internal/adapter/planner/jsonmode
// reads a Decision from - just enough to prove app.New wires that planner
// in when Config.LLM.Mode is app.ModeJSON, not to test its own mapping
// (internal/adapter/planner/jsonmode has that).
func fixtureJSONChatServer(t *testing.T) *httptest.Server {
	t.Helper()

	const response = `{
    "choices": [{
      "finish_reason": "stop",
      "message": {
        "role": "assistant",
        "content": "{\"kind\":\"call\",\"service\":\"fixture\",\"operationId\":\"ListWidgets\",\"args\":{}}"
      }
    }]
  }`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(response)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	return server
}

func TestNewWithLLMModeJSONUsesTheJSONPlanner(t *testing.T) {
	fixture := fixtureService(t)
	chatServer := fixtureJSONChatServer(t)

	handler, err := app.New(&app.Config{
		Services: []app.Service{{Name: "fixture", URL: fixture.URL}},
		LLM:      app.LLM{BaseURL: chatServer.URL, Model: "test-model", Mode: app.ModeJSON},
	})
	require.NoError(t, err)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

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

func TestNewRejectsAnUnknownLLMMode(t *testing.T) {
	_, err := app.New(&app.Config{
		LLM: app.LLM{BaseURL: "http://127.0.0.1:0", Mode: "not-a-real-mode"},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, app.ErrInvalidLLMMode)
}

func TestNewFailsWhenAConfiguredServiceIsUnreachable(t *testing.T) {
	_, err := app.New(&app.Config{Services: []app.Service{{Name: "gone", URL: "http://127.0.0.1:0"}}})

	require.Error(t, err)
}

// TestNewOpensTheWorkspaceStoreWhenDBPathIsSet is docs/plans/workspaces.md
// Task 0's "wire the store into pkg/app": a valid ORCHESTRA_DB_PATH lets
// the platform start.
func TestNewOpensTheWorkspaceStoreWhenDBPathIsSet(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "workspaces.db")

	_, err := app.New(&app.Config{DBPath: dbPath, AdminPassword: "correct horse battery staple"})

	require.NoError(t, err)
}

// TestNewSeedsTheAdminAccountWhenDBPathIsSet is docs/plans/auth.md, Task 0:
// a valid ORCHESTRA_ADMIN_PASSWORD lets the platform seed its first admin
// at startup.
func TestNewSeedsTheAdminAccountWhenDBPathIsSet(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "workspaces.db")

	_, err := app.New(&app.Config{DBPath: dbPath, AdminPassword: ""})

	require.Error(t, err)
}

// TestNewFailsWhenTheWorkspaceStoreCannotBeOpened is the other half: a bad
// path - one whose directory does not exist - fails startup instead of
// starting a platform that will forget its workspaces the moment someone
// tries to save one.
func TestNewFailsWhenTheWorkspaceStoreCannotBeOpened(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "missing-directory", "workspaces.db")

	_, err := app.New(&app.Config{DBPath: dbPath, AdminPassword: "correct horse battery staple"})

	require.Error(t, err)
}

func TestNewServesInvokeAndRejectsAnUnknownEndpoint(t *testing.T) {
	handler, err := app.New(&app.Config{})
	require.NoError(t, err)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	raw, err := json.Marshal(map[string]any{"service": "inventory", "operationId": "GetInventoryItem", "args": map[string]any{}})
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/api/invoke", bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	// app.Config{} configures no services, so the catalogue is empty: the
	// named operation is not in it, which is a 400 (docs/plans/orchestration.md,
	// Task 8), not the 501 the placeholder used to answer with.
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
