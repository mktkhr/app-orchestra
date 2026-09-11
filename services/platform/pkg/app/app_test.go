package app_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/pkg/app"
)

func TestNewServesHealth(t *testing.T) {
	handler, err := app.New(app.Config{})
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

	handler, err := app.New(app.Config{
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

func TestNewFailsWhenAConfiguredServiceIsUnreachable(t *testing.T) {
	_, err := app.New(app.Config{Services: []app.Service{{Name: "gone", URL: "http://127.0.0.1:0"}}})

	require.Error(t, err)
}

func TestNewServesInvokeAndRejectsAnUnknownEndpoint(t *testing.T) {
	handler, err := app.New(app.Config{})
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
