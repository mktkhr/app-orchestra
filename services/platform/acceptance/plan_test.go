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
      x-orchestra-expose: true
      parameters:
        - name: status
          in: query
          required: false
          description: Restrict the result to items in this status.
          schema:
            type: string
            enum: [allocated, staged, quarantined, consigned]
            x-enum-labels:
              allocated: 引当済
              staged: 出荷準備完了
              quarantined: 検品保留
              consigned: 預託在庫
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
      x-orchestra-expose: true
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

// inventorySpecWithUnexposedOp is inventorySpecWithCreate plus one further
// operation carrying no x-orchestra-expose mark at all, used to drive
// Task 17's acceptance criterion: an operation the catalogue never
// exposed must be unreachable through /api/invoke, exactly as if it did
// not exist.
const inventorySpecWithUnexposedOp = inventorySpecWithCreate + `
  /api/inventory/internal/reset:
    post:
      operationId: ResetInventoryInternal
      summary: Reset the service's in-memory store. Not exposed to the model.
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
		Service     string         `json:"service"`
		OperationID string         `json:"operationId"`
		Args        map[string]any `json:"args"`
	} `json:"source"`
	Message string         `json:"message"`
	Schema  map[string]any `json:"schema"`
	Initial map[string]any `json:"initial"`
	Target  struct {
		Service     string `json:"service"`
		OperationID string `json:"operationId"`
	} `json:"target"`
	Question string             `json:"question"`
	Param    string             `json:"param"`
	Options  []planOptionOnWire `json:"options"`
}

// planOptionOnWire is the wire shape of Option.
type planOptionOnWire struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// answerOnWire is the wire shape of one Answer.
type answerOnWire struct {
	Param string `json:"param"`
	Value string `json:"value"`
}

// postPlan posts query (and, when given, answers) to /api/plan and decodes
// the response.
func postPlan(t *testing.T, server *httptest.Server, query string, answers ...answerOnWire) (int, planResponse) {
	t.Helper()

	payload := map[string]any{"query": query}
	if len(answers) > 0 {
		payload["answers"] = answers
	}

	raw, err := json.Marshal(payload)
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

	server := newTestApp(t, &app.Config{
		Services: []app.Service{
			{Name: "inventory", URL: inventory.server.URL},
			{Name: "attendance", URL: attendance.server.URL},
		},
		PlanFixtures: []app.PlanFixture{
			{Query: "在庫の一覧を見せて", Service: "inventory", OperationID: "ListInventoryItems"},
		},
	})

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

	server := newTestApp(t, &app.Config{
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

	server := newTestApp(t, &app.Config{
		Services: []app.Service{{Name: "inventory", URL: inventory.server.URL}},
		// A query the table (default or configured) does not recognise
		// answers DecisionNone (see internal/adapter/planner/stub).
	})

	status, body := postPlan(t, server, "今日の天気は？")

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "none", body.Kind)
	assert.NotEmpty(t, body.Message)
	assert.Empty(t, inventory.requests, "a none decision must not call any service")
}

// TestPlanAskDecisionListsCatalogueOptionsAndCallsNoService drives Task 9
// Step 1: an ask decision must list every value the inventorySpec fixture's
// "status" enum declares, with its Japanese label, and it must never touch
// a service - a disambiguation question is answered from the catalogue
// alone.
func TestPlanAskDecisionListsCatalogueOptionsAndCallsNoService(t *testing.T) {
	inventory := newFixtureService(t, inventorySpec, "/api/inventory/items", `{"items":[]}`)

	server := newTestApp(t, &app.Config{
		Services: []app.Service{{Name: "inventory", URL: inventory.server.URL}},
		PlanFixtures: []app.PlanFixture{
			{
				Query:       "破損した在庫を見せて",
				Ask:         true,
				Question:    "どのステータスですか？",
				Param:       "status",
				Service:     "inventory",
				OperationID: "ListInventoryItems",
			},
		},
	})

	status, body := postPlan(t, server, "破損した在庫を見せて")

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "ask", body.Kind)
	assert.Equal(t, "どのステータスですか？", body.Question)
	assert.Equal(t, "status", body.Param)
	assert.ElementsMatch(t, []planOptionOnWire{
		{Value: "allocated", Label: "引当済"},
		{Value: "staged", Label: "出荷準備完了"},
		{Value: "quarantined", Label: "検品保留"},
		{Value: "consigned", Label: "預託在庫"},
	}, body.Options)

	assert.Empty(t, inventory.requests, "an ask decision must never call a service")
}

// TestPlanResubmittedWithAnswersReachesThePlannerAndProducesAResult drives
// Task 9 Step 2: posting the same question again with an answer must reach
// the planner with it (the stub fixture below only matches the {query,
// answers} pair, so a mismatch would fall through to DecisionNone) and
// produce the resulting table.
func TestPlanResubmittedWithAnswersReachesThePlannerAndProducesAResult(t *testing.T) {
	inventory := newFixtureService(t, inventorySpec, "/api/inventory/items", `{"items":[{"id":"itm-1","status":"quarantined"}]}`)

	server := newTestApp(t, &app.Config{
		Services: []app.Service{{Name: "inventory", URL: inventory.server.URL}},
		PlanFixtures: []app.PlanFixture{
			{
				Query:       "破損した在庫を見せて",
				Ask:         true,
				Question:    "どのステータスですか？",
				Param:       "status",
				Service:     "inventory",
				OperationID: "ListInventoryItems",
			},
			{
				Query:       "破損した在庫を見せて",
				Answers:     []app.Answer{{Param: "status", Value: "quarantined"}},
				Service:     "inventory",
				OperationID: "ListInventoryItems",
				Args:        map[string]any{"status": "quarantined"},
			},
		},
	})

	status, body := postPlan(t, server, "破損した在庫を見せて", answerOnWire{Param: "status", Value: "quarantined"})

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "result", body.Kind)
	assert.Equal(t, "table", body.Component)
	assert.Equal(t, "inventory", body.Source.Service)
	assert.Equal(t, "ListInventoryItems", body.Source.OperationID)
	require.Len(t, body.Data["items"], 1)

	require.Len(t, inventory.requests, 1)
	assert.Equal(t, "status=quarantined", inventory.requests[0].URL.RawQuery)
}
