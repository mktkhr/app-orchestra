package app_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/pkg/app"
)

// appTestAdminPassword seeds the admin account every test in this file
// that needs a DBPath builds its Config with (app.New now refuses an
// empty one - see TestNewFailsWhenDBPathIsEmpty, below). None of these
// tests are about authentication itself (acceptance/session_test.go and
// acceptance/workspace_test.go cover that end-to-end); they just each need
// their own t.TempDir()-backed database to build a handler at all.
const appTestAdminPassword = "correct horse battery staple"

// signInTestAdmin signs in as the admin account appTestAdminPassword
// seeds, storing the resulting session cookie on server.Client()'s jar so
// every later request through it reaches its handler instead of
// requireSession's own 401 (docs/specs/auth.md, AC-A-102: everything but
// GET /api/health and POST /api/session requires one).
func signInTestAdmin(t *testing.T, server *httptest.Server) {
	t.Helper()

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	server.Client().Jar = jar

	raw, err := json.Marshal(map[string]string{"name": "admin", "password": appTestAdminPassword})
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/api/session", bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	require.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestNewFailsWhenDBPathIsEmpty closes the auth bypass docs/specs/auth.md
// A6 warns about: a nil session store used to make requireSession run
// every request as a fixed admin (see DECISIONS.md). Workspaces and
// accounts both need a database, so there is no Config for which running
// without one is a legitimate choice - New must refuse it itself, the
// same way internal/infra/config.Load already refuses to start cmd/api
// without ORCHESTRA_DB_PATH (config.ErrMissingDBPath), rather than leaving
// that check for a caller that goes through config.Load to remember and
// every other caller of New free to forget.
func TestNewFailsWhenDBPathIsEmpty(t *testing.T) {
	_, err := app.New(&app.Config{})

	require.Error(t, err)
	assert.ErrorIs(t, err, app.ErrMissingDBPath)
}

func TestNewServesHealth(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "app.db")

	handler, err := app.New(&app.Config{DBPath: dbPath, AdminPassword: appTestAdminPassword})
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

// TestNewWithContextTurnsConfiguredStillServesHealth exercises the
// non-zero branch of contextWindowOption: Config.ContextTurns, when set,
// must reach usecase.NewOrchestrator without New itself refusing to build
// (see TestNewServesHealth for the zero-value path every other test in
// this file already takes).
func TestNewWithContextTurnsConfiguredStillServesHealth(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "app.db")

	handler, err := app.New(&app.Config{DBPath: dbPath, AdminPassword: appTestAdminPassword, ContextTurns: 3})
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

// TestNewWiresAProposePlanFixtureThroughToAProposalResult closes the gap
// docs/plans/proposing.md Task 2 names: a stub-planner fixture can produce
// a DecisionProposal, carrying its own component/chart/title through
// Orchestrator.propose the same way a real propose_panel tool call would
// (docs/specs/proposing.md, section 3-4) - this is what lets
// e2e/src/proposing.test.ts and e2e/browser/proposing.spec.ts drive the
// journey from the built binary without a model.
func TestNewWiresAProposePlanFixtureThroughToAProposalResult(t *testing.T) {
	fixture := fixtureService(t)

	handler, err := app.New(&app.Config{
		Services: []app.Service{{Name: "fixture", URL: fixture.URL}},
		PlanFixtures: []app.PlanFixture{
			{
				Query:       "widgets as a bar chart please",
				Propose:     true,
				Service:     "fixture",
				OperationID: "ListWidgets",
				Args:        map[string]any{},
				Component:   "chart",
				Title:       "ウィジェットの棒グラフ",
				Chart:       &app.Chart{Category: "status", Value: "count", Kind: "bar"},
			},
		},
		DBPath:        filepath.Join(t.TempDir(), "app.db"),
		AdminPassword: appTestAdminPassword,
	})
	require.NoError(t, err)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	signInTestAdmin(t, server)

	// workspaceId is required here since docs/specs/offering.md: propose_panel
	// is only offered to a question asked from a workspace (O3/O4) - this
	// proposal fixture would otherwise be refused as a tool this request was
	// never offered (usecase.ErrToolNotOffered).
	raw, err := json.Marshal(map[string]string{
		"query": "widgets as a bar chart please", "workspaceId": "fixture-workspace",
	})
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/api/plan", bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Kind  string `json:"kind"`
		Panel struct {
			Service     string         `json:"service"`
			OperationID string         `json:"operationId"`
			Component   string         `json:"component"`
			Title       string         `json:"title"`
			View        map[string]any `json:"view"`
		} `json:"panel"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "proposal", body.Kind)
	assert.Equal(t, "fixture", body.Panel.Service)
	assert.Equal(t, "ListWidgets", body.Panel.OperationID)
	assert.Equal(t, "chart", body.Panel.Component)
	assert.Equal(t, "ウィジェットの棒グラフ", body.Panel.Title)
	assert.Equal(t, map[string]any{
		"chart": map[string]any{"category": "status", "value": "count", "kind": "bar"},
	}, body.Panel.View)
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
		Services:      []app.Service{{Name: "fixture", URL: fixture.URL}},
		LLM:           app.LLM{BaseURL: chatServer.URL, Model: "test-model", Mode: app.ModeJSON},
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

func TestNewRejectsAnUnknownLLMMode(t *testing.T) {
	_, err := app.New(&app.Config{
		LLM:           app.LLM{BaseURL: "http://127.0.0.1:0", Mode: "not-a-real-mode"},
		DBPath:        filepath.Join(t.TempDir(), "app.db"),
		AdminPassword: appTestAdminPassword,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, app.ErrInvalidLLMMode)
}

func TestNewFailsWhenAConfiguredServiceIsUnreachable(t *testing.T) {
	_, err := app.New(&app.Config{
		Services:      []app.Service{{Name: "gone", URL: "http://127.0.0.1:0"}},
		DBPath:        filepath.Join(t.TempDir(), "app.db"),
		AdminPassword: appTestAdminPassword,
	})

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
	handler, err := app.New(&app.Config{
		DBPath:        filepath.Join(t.TempDir(), "app.db"),
		AdminPassword: appTestAdminPassword,
	})
	require.NoError(t, err)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	signInTestAdmin(t, server)

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

// TestNewSeedsExtraAccountsFromSeedAccounts proves the composition-root
// seam pkg/app.Config.SeedAccounts uses to put a non-admin account in
// place - the way an acceptance test builds one, since there is no HTTP
// route that creates accounts (docs/specs/auth.md, section 8;
// docs/plans/auth.md, Task 3): a person seeded this way signs in over
// /api/session exactly like the admin does.
func TestNewSeedsExtraAccountsFromSeedAccounts(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "seed-accounts.db")

	handler, err := app.New(&app.Config{
		DBPath:        dbPath,
		AdminPassword: appTestAdminPassword,
		SeedAccounts: []app.SeedAccount{
			{Name: "yamada", Password: "yamada's password", Role: "user"},
		},
	})
	require.NoError(t, err)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	server.Client().Jar = jar

	raw, err := json.Marshal(map[string]any{"name": "yamada", "password": "yamada's password"})
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/api/session", bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Role string `json:"role"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "user", body.Role)
}

// TestNewFailsWhenASeedAccountIsInvalid proves a bad SeedAccounts entry -
// an empty password, the same rule AdminPassword itself follows - fails
// startup rather than silently skipping the account.
func TestNewFailsWhenASeedAccountIsInvalid(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "bad-seed-account.db")

	_, err := app.New(&app.Config{
		DBPath:        dbPath,
		AdminPassword: appTestAdminPassword,
		SeedAccounts:  []app.SeedAccount{{Name: "yamada", Password: "", Role: "user"}},
	})

	require.Error(t, err)
}
