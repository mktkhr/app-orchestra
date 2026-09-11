package acceptance_test

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

// inventorySpec and attendanceSpec are minimal OpenAPI documents, standing
// in for the real dummy services: this module cannot depend on
// services/inventory or services/attendance (a separate go.mod each), and
// Task 6 only needs a safe list operation to drive the orchestrator's
// safe-call path against.
const inventorySpec = `
openapi: 3.0.3
info:
  title: Fixture Inventory
  version: 0.1.0
paths:
  /api/inventory/items:
    get:
      operationId: ListInventoryItems
      summary: List stock items.
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

// inventorySpecWithCreate is inventorySpec plus an unsafe create operation,
// used to drive the form path (Task 7): the service must never receive a
// request for it.
const inventorySpecWithCreate = inventorySpec + `
  /api/inventory/items/create:
    post:
      operationId: CreateInventoryItem
      summary: Create a stock item.
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required:
                - name
                - status
              properties:
                name:
                  type: string
                status:
                  type: string
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
`

const attendanceSpec = `
openapi: 3.0.3
info:
  title: Fixture Attendance
  version: 0.1.0
paths:
  /api/attendance/records:
    get:
      operationId: ListAttendanceRecords
      summary: List attendance records.
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

// fixtureService serves spec at /openapi.yaml, and records every request
// it receives at any other path while answering it with body.
type fixtureService struct {
	server   *httptest.Server
	requests []*http.Request
}

func newFixtureService(t *testing.T, spec, path, body string) *fixtureService {
	t.Helper()

	f := &fixtureService{}

	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/openapi.yaml" {
			w.Header().Set("Content-Type", "application/yaml")
			if _, err := w.Write([]byte(spec)); err != nil {
				t.Errorf("writing fixture spec: %v", err)
			}

			return
		}

		f.requests = append(f.requests, r)

		if r.URL.Path != path {
			w.WriteHeader(http.StatusNotFound)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(f.server.Close)

	return f
}

// planResponse is the subset of PlanResult this test asserts on.
type planResponse struct {
	Kind      string         `json:"kind"`
	Component string         `json:"component"`
	Data      map[string]any `json:"data"`
	Source    struct {
		Service     string `json:"service"`
		OperationID string `json:"operationId"`
	} `json:"source"`
	Message string         `json:"message"`
	Schema  map[string]any `json:"schema"`
	Initial map[string]any `json:"initial"`
	Target  struct {
		Service     string `json:"service"`
		OperationID string `json:"operationId"`
	} `json:"target"`
}

// postPlan posts query to /api/plan and decodes the response.
func postPlan(t *testing.T, server *httptest.Server, query string) (int, planResponse) {
	t.Helper()

	raw, err := json.Marshal(map[string]string{"query": query})
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/api/plan", bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	var body planResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	return resp.StatusCode, body
}

func TestPlanSafeCallReachesTheServiceAndRendersATable(t *testing.T) {
	inventory := newFixtureService(t, inventorySpec, "/api/inventory/items", `{"items":[{"id":"1"}]}`)
	attendance := newFixtureService(t, attendanceSpec, "/api/attendance/records", `{"items":[]}`)

	handler, err := app.New(app.Config{
		Services: []app.Service{
			{Name: "inventory", URL: inventory.server.URL},
			{Name: "attendance", URL: attendance.server.URL},
		},
		PlanFixtures: []app.PlanFixture{
			{Query: "在庫の一覧を見せて", Service: "inventory", OperationID: "ListInventoryItems"},
		},
	})
	require.NoError(t, err)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	status, body := postPlan(t, server, "在庫の一覧を見せて")

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "result", body.Kind)
	assert.Equal(t, "table", body.Component)
	assert.Equal(t, "inventory", body.Source.Service)
	assert.Equal(t, "ListInventoryItems", body.Source.OperationID)
	require.Len(t, body.Data["items"], 1)

	require.Len(t, inventory.requests, 1, "the inventory fixture must actually have been called")
	assert.Equal(t, "/api/inventory/items", inventory.requests[0].URL.Path)
	assert.Empty(t, attendance.requests, "an unrelated service must not be called")
}

func TestPlanUnsafeCallReturnsAFormAndNeverReachesTheService(t *testing.T) {
	inventory := newFixtureService(t, inventorySpecWithCreate, "/api/inventory/items/create", `{}`)
	attendance := newFixtureService(t, attendanceSpec, "/api/attendance/records", `{"items":[]}`)

	handler, err := app.New(app.Config{
		Services: []app.Service{
			{Name: "inventory", URL: inventory.server.URL},
			{Name: "attendance", URL: attendance.server.URL},
		},
		PlanFixtures: []app.PlanFixture{
			{
				Query:       "在庫を登録して",
				Service:     "inventory",
				OperationID: "CreateInventoryItem",
				Args:        map[string]any{"name": "新しい棚", "status": "allocated"},
			},
		},
	})
	require.NoError(t, err)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	status, body := postPlan(t, server, "在庫を登録して")

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "form", body.Kind)

	require.NotNil(t, body.Schema)
	assert.Equal(t, "object", body.Schema["type"])

	properties, ok := body.Schema["properties"].(map[string]any)
	require.True(t, ok, "schema must describe the request body's properties")
	assert.Contains(t, properties, "name")
	assert.Contains(t, properties, "status")

	required, ok := body.Schema["required"].([]any)
	require.True(t, ok, "schema must carry the request body's required fields")
	assert.ElementsMatch(t, []any{"name", "status"}, required)

	assert.Equal(t, map[string]any{"name": "新しい棚", "status": "allocated"}, body.Initial)

	assert.Equal(t, "inventory", body.Target.Service)
	assert.Equal(t, "CreateInventoryItem", body.Target.OperationID)

	assert.Empty(t, inventory.requests, "an unsafe call must never reach the service")
	assert.Empty(t, attendance.requests, "an unrelated service must not be called")
}

func TestPlanNoneCallsNoServiceAndReportsAMessage(t *testing.T) {
	inventory := newFixtureService(t, inventorySpec, "/api/inventory/items", `{"items":[]}`)

	handler, err := app.New(app.Config{
		Services: []app.Service{{Name: "inventory", URL: inventory.server.URL}},
		// A query the table (default or configured) does not recognise
		// answers DecisionNone (see internal/adapter/planner/stub).
	})
	require.NoError(t, err)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	status, body := postPlan(t, server, "今日の天気は？")

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "none", body.Kind)
	assert.NotEmpty(t, body.Message)
	assert.Empty(t, inventory.requests, "a none decision must not call any service")
}
