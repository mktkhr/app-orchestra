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

// invokeResponse is the subset of InvokeResult this test asserts on.
type invokeResponse struct {
	Component string         `json:"component"`
	Data      map[string]any `json:"data"`
}

// postInvoke posts service/operationId/args to /api/invoke and decodes the
// response.
func postInvoke(t *testing.T, server *httptest.Server, service, operationID string, args map[string]any) (int, invokeResponse) {
	t.Helper()

	raw, err := json.Marshal(map[string]any{"service": service, "operationId": operationID, "args": args})
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/api/invoke", bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	var body invokeResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	return resp.StatusCode, body
}

func TestInvokeReachesTheServiceAndRendersTheCreatedEntityAsDetail(t *testing.T) {
	inventory := newFixtureService(t, inventorySpecWithCreate, "/api/inventory/items/create",
		`{"id":"1","name":"検証用","status":"quarantined"}`)
	attendance := newFixtureService(t, attendanceSpec, "/api/attendance/records", `{"items":[]}`)

	handler, err := app.New(&app.Config{
		Services: []app.Service{
			{Name: "inventory", URL: inventory.server.URL},
			{Name: "attendance", URL: attendance.server.URL},
		},
	})
	require.NoError(t, err)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	status, body := postInvoke(t, server, "inventory", "CreateInventoryItem",
		map[string]any{"name": "検証用", "status": "quarantined"})

	require.Equal(t, http.StatusOK, status)
	// CreateInventoryItem's response is a single object, so it renders as
	// "detail" - even though the endpoint has a request body, which would
	// make domain.Render say "form" for a call that has not happened yet.
	// This one already has.
	assert.Equal(t, "detail", body.Component)
	assert.Equal(t, "1", body.Data["id"])

	require.Len(t, inventory.requests, 1, "the inventory fixture must actually have been called")
	assert.Equal(t, "/api/inventory/items/create", inventory.requests[0].URL.Path)
	assert.Empty(t, attendance.requests, "an unrelated service must not be called")
}

func TestInvokeUnknownOperationReturns400AndCallsNoService(t *testing.T) {
	inventory := newFixtureService(t, inventorySpecWithCreate, "/api/inventory/items/create", `{}`)

	handler, err := app.New(&app.Config{
		Services: []app.Service{{Name: "inventory", URL: inventory.server.URL}},
	})
	require.NoError(t, err)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	status, _ := postInvoke(t, server, "inventory", "NoSuchOperation", map[string]any{})

	require.Equal(t, http.StatusBadRequest, status)
	assert.Empty(t, inventory.requests, "an unknown operation must never reach the service")
}

func TestInvokeMissingRequiredArgumentReturns400AndCallsNoService(t *testing.T) {
	inventory := newFixtureService(t, inventorySpecWithCreate, "/api/inventory/items/create", `{}`)

	handler, err := app.New(&app.Config{
		Services: []app.Service{{Name: "inventory", URL: inventory.server.URL}},
	})
	require.NoError(t, err)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	status, _ := postInvoke(t, server, "inventory", "CreateInventoryItem", map[string]any{"name": "不完全"})

	require.Equal(t, http.StatusBadRequest, status)
	assert.Empty(t, inventory.requests, "invalid arguments must never reach the service")
}
