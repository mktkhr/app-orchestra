// Package acceptance drives the real inventory service object graph
// (pkg/app) behind httptest, over HTTP, the way a client actually would. It
// is a separate module so it cannot reach into the service's internal/ tree.
package acceptance_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/mktkhr/app-orchestra/services/inventory/pkg/app"
)

type item struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Quantity int    `json:"quantity"`
}

type itemList struct {
	Items []item `json:"items"`
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	handler, err := app.New()
	require.NoError(t, err)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return server
}

// doJSON sends a request and returns its status code, decoding the JSON body
// into out (when non-nil) before closing it - so the body is always closed
// here, not left for the caller to remember.
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
	defer func() { _ = resp.Body.Close() }()

	if out != nil {
		require.NoError(t, json.NewDecoder(resp.Body).Decode(out))
	}

	return resp.StatusCode
}

func TestListInventoryItemsFiltersByStatus(t *testing.T) {
	server := newTestServer(t)

	var all itemList
	status := doJSON(t, server, http.MethodGet, "/api/inventory/items", nil, &all)
	require.Equal(t, http.StatusOK, status)
	require.Len(t, all.Items, 8)

	var allocated itemList
	status = doJSON(t, server, http.MethodGet, "/api/inventory/items?status=allocated", nil, &allocated)
	require.Equal(t, http.StatusOK, status)

	require.NotEmpty(t, allocated.Items)
	for _, it := range allocated.Items {
		assert.Equal(t, "allocated", it.Status)
	}
	assert.Less(t, len(allocated.Items), len(all.Items))
}

func TestGetInventoryItemReturnsExistingItem(t *testing.T) {
	server := newTestServer(t)

	var got item
	status := doJSON(t, server, http.MethodGet, "/api/inventory/items/itm-001", nil, &got)

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "itm-001", got.ID)
}

func TestGetInventoryItemReportsMissingItem(t *testing.T) {
	server := newTestServer(t)

	// itm-999 matches the contract's own id pattern (`^itm-[0-9]+$`,
	// services/inventory/api/openapi.yaml - added for TODO.md's
	// "real-attendance-detail", read by the platform's usecase.idAffinity)
	// but names no item the fixture seeds: the shape a real "not found"
	// looks like, as opposed to a malformed id, which the request
	// validation middleware now rejects with 400 before this handler ever
	// runs.
	status := doJSON(t, server, http.MethodGet, "/api/inventory/items/itm-999", nil, nil)

	assert.Equal(t, http.StatusNotFound, status)
}

func TestCreateInventoryItemAddsItem(t *testing.T) {
	server := newTestServer(t)

	newItem := map[string]any{"name": "テスト用アイテム", "status": "staged", "quantity": 3}

	var created item
	status := doJSON(t, server, http.MethodPost, "/api/inventory/items", newItem, &created)

	require.Equal(t, http.StatusCreated, status)
	assert.NotEmpty(t, created.ID)
	assert.Equal(t, "テスト用アイテム", created.Name)
	assert.Equal(t, "staged", created.Status)
	assert.Equal(t, 3, created.Quantity)

	var fetched item
	status = doJSON(t, server, http.MethodGet, "/api/inventory/items/"+created.ID, nil, &fetched)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, created, fetched)
}

func TestOpenAPISpecEndpointReturnsParsableYAML(t *testing.T) {
	server := newTestServer(t)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/openapi.yaml", http.NoBody)
	require.NoError(t, err)

	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/yaml", resp.Header.Get("Content-Type"))

	var doc map[string]any
	require.NoError(t, yaml.NewDecoder(resp.Body).Decode(&doc))
	assert.Equal(t, "3.0.3", doc["openapi"])

	paths, ok := doc["paths"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, paths, "/api/inventory/items")
}
