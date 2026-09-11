package acceptance_test

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

// workspaceSummaryOnWire is the wire shape of one WorkspaceSummary.
type workspaceSummaryOnWire struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	PanelCount int    `json:"panelCount"`
}

// workspaceOnWire is the wire shape of Workspace.
type workspaceOnWire struct {
	ID     string        `json:"id"`
	Name   string        `json:"name"`
	Panels []panelOnWire `json:"panels"`
}

// panelOnWire is the wire shape of Panel.
type panelOnWire struct {
	ID          string         `json:"id"`
	WorkspaceID string         `json:"workspaceId"`
	Service     string         `json:"service"`
	OperationID string         `json:"operationId"`
	Args        map[string]any `json:"args"`
	Component   string         `json:"component"`
	Title       string         `json:"title"`
	Position    int            `json:"position"`
}

// newWorkspaceTestApp builds a platform over a fresh SQLite file in
// t.TempDir(), with inventory the only configured service - exposing
// ListInventoryItems and, when withUnexposed is true, an operation the
// service declares but does not mark x-orchestra-expose: true.
func newWorkspaceTestApp(t *testing.T, withUnexposed bool) *httptest.Server {
	t.Helper()

	spec := inventorySpec
	if withUnexposed {
		spec = inventorySpecWithUnexposedOp
	}

	inventory := newFixtureService(t, spec, "/api/inventory/items", `{"items":[{"id":"1"}]}`)

	dbPath := filepath.Join(t.TempDir(), "workspaces.db")

	handler, err := app.New(&app.Config{
		Services:      []app.Service{{Name: "inventory", URL: inventory.server.URL}},
		DBPath:        dbPath,
		AdminPassword: "correct horse battery staple",
	})
	require.NoError(t, err)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return server
}

// doJSON sends method to path with body (marshalled as JSON, or nil for
// none) and decodes the response into out (when non-nil).
func doJSON(t *testing.T, server *httptest.Server, method, path string, body, out any) int {
	t.Helper()

	var reader *bytes.Reader

	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)

		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}

	req, err := http.NewRequestWithContext(t.Context(), method, server.URL+path, reader)
	require.NoError(t, err)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	if out != nil {
		require.NoError(t, json.NewDecoder(resp.Body).Decode(out))
	}

	return resp.StatusCode
}

func TestWorkspaceLifecycleCreateAddPanelReadDelete(t *testing.T) {
	server := newWorkspaceTestApp(t, false)

	var created workspaceOnWire
	status := doJSON(t, server, http.MethodPost, "/api/workspaces", map[string]any{"name": "在庫ボード"}, &created)
	require.Equal(t, http.StatusCreated, status)
	assert.Equal(t, "在庫ボード", created.Name)
	require.NotEmpty(t, created.ID)

	var summaries []workspaceSummaryOnWire
	status = doJSON(t, server, http.MethodGet, "/api/workspaces", nil, &summaries)
	require.Equal(t, http.StatusOK, status)
	require.Len(t, summaries, 1)
	assert.Equal(t, created.ID, summaries[0].ID)
	assert.Equal(t, 0, summaries[0].PanelCount)

	var panel panelOnWire
	status = doJSON(t, server, http.MethodPost, "/api/workspaces/"+created.ID+"/panels", map[string]any{
		"service":     "inventory",
		"operationId": "ListInventoryItems",
		"args":        map[string]any{"status": "quarantined"},
		"component":   "table",
		"title":       "検品保留の在庫",
	}, &panel)
	require.Equal(t, http.StatusCreated, status)
	assert.Equal(t, created.ID, panel.WorkspaceID)
	assert.Equal(t, "検品保留の在庫", panel.Title)
	assert.Equal(t, "quarantined", panel.Args["status"])

	var ws workspaceOnWire
	status = doJSON(t, server, http.MethodGet, "/api/workspaces/"+created.ID, nil, &ws)
	require.Equal(t, http.StatusOK, status)
	require.Len(t, ws.Panels, 1)
	assert.Equal(t, panel.ID, ws.Panels[0].ID)
	assert.Equal(t, "inventory", ws.Panels[0].Service)
	assert.Equal(t, "ListInventoryItems", ws.Panels[0].OperationID)

	status = doJSON(t, server, http.MethodDelete, "/api/workspaces/"+created.ID+"/panels/"+panel.ID, nil, nil)
	require.Equal(t, http.StatusNoContent, status)

	status = doJSON(t, server, http.MethodGet, "/api/workspaces/"+created.ID, nil, &ws)
	require.Equal(t, http.StatusOK, status)
	assert.Empty(t, ws.Panels)

	status = doJSON(t, server, http.MethodDelete, "/api/workspaces/"+created.ID, nil, nil)
	require.Equal(t, http.StatusNoContent, status)

	status = doJSON(t, server, http.MethodGet, "/api/workspaces/"+created.ID, nil, nil)
	assert.Equal(t, http.StatusNotFound, status)
}

func TestAddPanelReturns404ForAnUnknownWorkspace(t *testing.T) {
	server := newWorkspaceTestApp(t, false)

	status := doJSON(t, server, http.MethodPost, "/api/workspaces/does-not-exist/panels", map[string]any{
		"service":     "inventory",
		"operationId": "ListInventoryItems",
		"args":        map[string]any{},
		"component":   "table",
		"title":       "検品保留の在庫",
	}, nil)

	assert.Equal(t, http.StatusNotFound, status)
}

func TestAddPanelReturns400ForAnOperationTheCatalogueDoesNotExpose(t *testing.T) {
	server := newWorkspaceTestApp(t, true)

	var created workspaceOnWire
	status := doJSON(t, server, http.MethodPost, "/api/workspaces", map[string]any{"name": "在庫ボード"}, &created)
	require.Equal(t, http.StatusCreated, status)

	status = doJSON(t, server, http.MethodPost, "/api/workspaces/"+created.ID+"/panels", map[string]any{
		"service":     "inventory",
		"operationId": "ResetInventoryInternal",
		"args":        map[string]any{},
		"component":   "table",
		"title":       "だめなやつ",
	}, nil)

	assert.Equal(t, http.StatusBadRequest, status)

	var ws workspaceOnWire
	status = doJSON(t, server, http.MethodGet, "/api/workspaces/"+created.ID, nil, &ws)
	require.Equal(t, http.StatusOK, status)
	assert.Empty(t, ws.Panels, "the rejected panel must not have been saved")
}
