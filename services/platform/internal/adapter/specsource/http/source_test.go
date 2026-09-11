package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	specsourcehttp "github.com/mktkhr/app-orchestra/services/platform/internal/adapter/specsource/http"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// fixtureServer serves testdata/fixture.yaml as GET /openapi.yaml, mirroring
// how a real service answers the platform.
func fixtureServer(t *testing.T) *httptest.Server {
	t.Helper()

	spec, err := os.ReadFile("testdata/fixture.yaml")
	require.NoError(t, err)

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openapi.yaml" {
			w.WriteHeader(http.StatusNotFound)

			return
		}

		w.Header().Set("Content-Type", "application/yaml")

		if _, err := w.Write(spec); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
}

func TestFetchBuildsCatalogueFromFixtureSpec(t *testing.T) {
	server := fixtureServer(t)
	defer server.Close()

	source := specsourcehttp.New(
		[]specsourcehttp.Service{{Name: "fixture", URL: server.URL}},
		nil,
	)

	catalog, err := source.Fetch(context.Background())

	require.NoError(t, err)
	assert.Len(t, catalog.Endpoints, 4,
		"listWidgets, createWidget, getWidget, exportWidgets: every x-orchestra-expose: true operation, "+
			"and no other — getSpec, getHiddenWidget, getOffWidget and getBadWidget are all unmarked or "+
			"marked false/non-boolean")

	list, ok := catalog.Find("fixture", "listWidgets")
	require.True(t, ok)
	assert.Equal(t, "GET", list.Method)
	assert.Equal(t, "/widgets", list.Path)
	assert.Equal(t, "fixture", list.Service)
	assert.Equal(t, "List widgets, optionally filtered by status.", list.Summary)

	require.Len(t, list.Parameters, 1)
	status := list.Parameters[0]
	assert.Equal(t, "status", status.Name)
	assert.Equal(t, "query", status.In)
	assert.False(t, status.Required)
	assert.Equal(t, []string{"allocated", "staged", "quarantined", "consigned"}, status.Schema.Enum)
	assert.Equal(t, map[string]string{
		"allocated":   "引当済",
		"staged":      "出荷準備完了",
		"quarantined": "検品保留",
		"consigned":   "預託在庫",
	}, status.Schema.EnumLabels)

	require.NotNil(t, list.Response)
	assert.Equal(t, domain.SchemaTypeObject, list.Response.Type)
	assert.Equal(t, []string{"items"}, list.Response.Required)
	items, ok := list.Response.Properties["items"]
	require.True(t, ok)
	assert.Equal(t, domain.SchemaTypeArray, items.Type)
	require.NotNil(t, items.Items)
	assert.Equal(t, domain.SchemaTypeObject, items.Items.Type)
}

func TestFetchConvertsRequestBody(t *testing.T) {
	server := fixtureServer(t)
	defer server.Close()

	source := specsourcehttp.New(
		[]specsourcehttp.Service{{Name: "fixture", URL: server.URL}},
		nil,
	)

	catalog, err := source.Fetch(context.Background())
	require.NoError(t, err)

	create, ok := catalog.Find("fixture", "createWidget")
	require.True(t, ok)
	assert.Equal(t, "POST", create.Method)
	assert.Equal(t, "/widgets", create.Path)
	require.NotNil(t, create.RequestBody)
	assert.Equal(t, domain.SchemaTypeObject, create.RequestBody.Type)
	_, hasStatus := create.RequestBody.Properties["status"]
	assert.True(t, hasStatus)
	assert.Equal(t, []string{"name", "status"}, create.RequestBody.Required,
		"the request body's required properties must be carried, in spec order")
}

func TestFetchConvertsUIHint(t *testing.T) {
	server := fixtureServer(t)
	defer server.Close()

	source := specsourcehttp.New(
		[]specsourcehttp.Service{{Name: "fixture", URL: server.URL}},
		nil,
	)

	catalog, err := source.Fetch(context.Background())
	require.NoError(t, err)

	get, ok := catalog.Find("fixture", "getWidget")
	require.True(t, ok)
	assert.Equal(t, "GET", get.Method)
	assert.Equal(t, "/widgets/{id}", get.Path)
	assert.Equal(t, domain.ComponentDetail, get.UIHint)
	require.Len(t, get.Parameters, 1)
	assert.Equal(t, "id", get.Parameters[0].Name)
	assert.Equal(t, "path", get.Parameters[0].In)
	assert.True(t, get.Parameters[0].Required)
}

func TestFetchOmitsResponseForNonJSONContent(t *testing.T) {
	server := fixtureServer(t)
	defer server.Close()

	source := specsourcehttp.New(
		[]specsourcehttp.Service{{Name: "fixture", URL: server.URL}},
		nil,
	)

	catalog, err := source.Fetch(context.Background())
	require.NoError(t, err)

	export, ok := catalog.Find("fixture", "exportWidgets")
	require.True(t, ok, "exportWidgets is x-orchestra-expose: true, so it must reach the catalogue")
	assert.Nil(t, export.Response, "exportWidgets answers YAML, which has no JSON schema to render")
}

func TestFetchExcludesOperationsNotMarkedExposed(t *testing.T) {
	server := fixtureServer(t)
	defer server.Close()

	source := specsourcehttp.New(
		[]specsourcehttp.Service{{Name: "fixture", URL: server.URL}},
		nil,
	)

	catalog, err := source.Fetch(context.Background())
	require.NoError(t, err)

	_, ok := catalog.Find("fixture", "getSpec")
	assert.False(t, ok, "an operation with no x-orchestra-expose at all must default to unexposed")

	_, ok = catalog.Find("fixture", "getHiddenWidget")
	assert.False(t, ok, "an operation with no x-orchestra-expose at all must default to unexposed")

	_, ok = catalog.Find("fixture", "getOffWidget")
	assert.False(t, ok, "x-orchestra-expose: false must be treated as unexposed")

	_, ok = catalog.Find("fixture", "getBadWidget")
	assert.False(t, ok, "a non-boolean x-orchestra-expose value must be treated as unexposed")
}

func TestFetchFailsWhenAnyServiceIsUnreachable(t *testing.T) {
	reachable := fixtureServer(t)
	defer reachable.Close()

	unreachable := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	unreachableURL := unreachable.URL
	unreachable.Close() // closed: nothing listens here any more

	source := specsourcehttp.New([]specsourcehttp.Service{
		{Name: "reachable", URL: reachable.URL},
		{Name: "unreachable", URL: unreachableURL},
	}, nil)

	catalog, err := source.Fetch(context.Background())

	require.Error(t, err)
	assert.Empty(t, catalog.Endpoints, "a partial catalogue would silently hide endpoints")
}

func TestFetchFailsOnNonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	source := specsourcehttp.New([]specsourcehttp.Service{{Name: "broken", URL: server.URL}}, nil)

	_, err := source.Fetch(context.Background())

	require.Error(t, err)
}

func TestFetchFailsOnUnparsableSpec(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte("not: [a, valid, openapi, document")); err != nil {
			t.Errorf("writing malformed response: %v", err)
		}
	}))
	defer server.Close()

	source := specsourcehttp.New([]specsourcehttp.Service{{Name: "broken", URL: server.URL}}, nil)

	_, err := source.Fetch(context.Background())

	require.Error(t, err)
}

func TestFetchWithNoServicesReturnsEmptyCatalogue(t *testing.T) {
	source := specsourcehttp.New(nil, nil)

	catalog, err := source.Fetch(context.Background())

	require.NoError(t, err)
	assert.Empty(t, catalog.Endpoints)
}

func TestFetchTrimsTrailingSlashFromBaseURL(t *testing.T) {
	server := fixtureServer(t)
	defer server.Close()

	source := specsourcehttp.New(
		[]specsourcehttp.Service{{Name: "fixture", URL: server.URL + "/"}},
		nil,
	)

	catalog, err := source.Fetch(context.Background())

	require.NoError(t, err)
	assert.Len(t, catalog.Endpoints, 4, "listWidgets, createWidget, getWidget, exportWidgets")
}
