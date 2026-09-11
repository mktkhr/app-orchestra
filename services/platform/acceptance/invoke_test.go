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

// postInvoke posts operationId/args, against the fixed "inventory" service
// every test in this file drives, to /api/invoke and decodes the response.
func postInvoke(t *testing.T, server *httptest.Server, operationID string, args map[string]any) (int, invokeResponse) {
	t.Helper()

	raw, err := json.Marshal(map[string]any{"service": "inventory", "operationId": operationID, "args": args})
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

	status, body := postInvoke(t, server, "CreateInventoryItem",
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

// TestInvokeReturns400AndCallsNoServiceForAnOperationTheCatalogueDoesNotHave
// covers the two ways /api/invoke can be asked for an operation id the
// catalogue does not carry: one that the service never declared at all,
// and one it declared but did not mark x-orchestra-expose: true (so
// internal/adapter/specsource/http.parseSpec never turned it into a
// domain.Endpoint in the first place). Both must be indistinguishable from
// the platform's point of view - a 400, and the service never called -
// which is the point of D13 (docs/specs/orchestration.md, DECISIONS.md):
// filtering only in usecase.ToolsFor would leave this route open to a
// crafted POST, so the catalogue itself is where both cases are decided,
// once.
func TestInvokeReturns400AndCallsNoServiceForAnOperationTheCatalogueDoesNotHave(t *testing.T) {
	tests := []struct {
		name        string
		spec        string
		path        string
		operationID string
	}{
		{
			name:        "operation id the service never declared",
			spec:        inventorySpecWithCreate,
			path:        "/api/inventory/items/create",
			operationID: "NoSuchOperation",
		},
		{
			name:        "operation the service declared but did not mark x-orchestra-expose: true",
			spec:        inventorySpecWithUnexposedOp,
			path:        "/api/inventory/internal/reset",
			operationID: "ResetInventoryInternal",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inventory := newFixtureService(t, tc.spec, tc.path, `{}`)

			handler, err := app.New(&app.Config{
				Services: []app.Service{{Name: "inventory", URL: inventory.server.URL}},
			})
			require.NoError(t, err)

			server := httptest.NewServer(handler)
			t.Cleanup(server.Close)

			status, _ := postInvoke(t, server, tc.operationID, map[string]any{})

			require.Equal(t, http.StatusBadRequest, status)
			assert.Empty(t, inventory.requests, "an operation outside the catalogue must never reach the service")
		})
	}
}

func TestInvokeMissingRequiredArgumentReturns400AndCallsNoService(t *testing.T) {
	inventory := newFixtureService(t, inventorySpecWithCreate, "/api/inventory/items/create", `{}`)

	handler, err := app.New(&app.Config{
		Services: []app.Service{{Name: "inventory", URL: inventory.server.URL}},
	})
	require.NoError(t, err)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	status, _ := postInvoke(t, server, "CreateInventoryItem", map[string]any{"name": "不完全"})

	require.Equal(t, http.StatusBadRequest, status)
	assert.Empty(t, inventory.requests, "invalid arguments must never reach the service")
}
