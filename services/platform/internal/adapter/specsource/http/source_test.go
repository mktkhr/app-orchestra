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
	assert.Len(t, catalog.Endpoints, 5,
		"listWidgets, createWidget, getWidget, exportWidgets, countWidgets: every x-orchestra-expose: true "+
			"operation, and no other — getSpec, getHiddenWidget, getOffWidget and getBadWidget are all "+
			"unmarked or marked false/non-boolean")

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
	assert.Equal(t, "ウィジェットの詳細", get.DisplayName)
	require.Len(t, get.Parameters, 1)
	assert.Equal(t, "id", get.Parameters[0].Name)
	assert.Equal(t, "path", get.Parameters[0].In)
	assert.True(t, get.Parameters[0].Required)

	// listWidgets declares no x-ui-hint at all: DisplayName stays empty
	// rather than falling back to anything here - DisplayNameOr is where a
	// caller's own fallback happens (domain/catalog.go), not the parser's.
	list, ok := catalog.Find("fixture", "listWidgets")
	require.True(t, ok)
	assert.Empty(t, list.DisplayName)
}

// TestFetchConvertsServiceDisplayName is DECISIONS.md's 2026-09-13 entry,
// one level up from TestFetchConvertsUIHint: a service's own
// info.x-ui-hint.displayName carries onto every one of its endpoints,
// operation-level DisplayName or not.
func TestFetchConvertsServiceDisplayName(t *testing.T) {
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
	assert.Equal(t, "フィクスチャ", get.ServiceDisplayName)

	list, ok := catalog.Find("fixture", "listWidgets")
	require.True(t, ok)
	assert.Equal(t, "フィクスチャ", list.ServiceDisplayName,
		"the service's own display name, unrelated to listWidgets' own (absent) operation-level one")
}

// TestFetchLeavesServiceDisplayNameEmptyWhenTheContractDeclaresNone is the
// other half: a service whose info object carries no x-ui-hint at all
// leaves every one of its endpoints' ServiceDisplayName empty -
// ServiceDisplayNameOr is where a caller's own fallback happens
// (domain/catalog.go), not the parser's.
func TestFetchLeavesServiceDisplayNameEmptyWhenTheContractDeclaresNone(t *testing.T) {
	doc := `openapi: 3.0.3
info: {title: bare, version: '1'}
paths:
  /bare:
    get:
      operationId: getBare
      x-orchestra-expose: true
      responses:
        "200": {description: n/a}
`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(doc)); err != nil {
			t.Errorf("writing bare-info response: %v", err)
		}
	}))
	defer server.Close()

	source := specsourcehttp.New([]specsourcehttp.Service{{Name: "bare", URL: server.URL}}, nil)

	catalog, err := source.Fetch(context.Background())
	require.NoError(t, err)

	bare, ok := catalog.Find("bare", "getBare")
	require.True(t, ok)
	assert.Empty(t, bare.ServiceDisplayName)
}

func TestFetchConvertsChartHint(t *testing.T) {
	server := fixtureServer(t)
	defer server.Close()

	source := specsourcehttp.New(
		[]specsourcehttp.Service{{Name: "fixture", URL: server.URL}},
		nil,
	)

	catalog, err := source.Fetch(context.Background())
	require.NoError(t, err)

	count, ok := catalog.Find("fixture", "countWidgets")
	require.True(t, ok)
	assert.Empty(t, count.UIHint,
		"countWidgets declares only x-ui-hint.chart, no component override")
	require.NotNil(t, count.ChartHint)
	assert.Equal(t, "status", count.ChartHint.Category)
	assert.Equal(t, "count", count.ChartHint.Value)
	assert.Equal(t, domain.ChartKindBar, count.ChartHint.Kind)

	// getWidget declares only x-ui-hint.component: it must still parse,
	// with no chart hint at all.
	get, ok := catalog.Find("fixture", "getWidget")
	require.True(t, ok)
	assert.Nil(t, get.ChartHint)
}

// assertFetchFailsOnEachSpec runs one table of malformed-extension bodies
// (each a paths fragment, appended to a bare openapi/info header) and
// asserts that fetching each one fails with an empty catalogue - the
// shape TestFetchFailsOnMalformedChartHint and TestFetchFailsOnMalformedExamples
// both need, extracted so golangci's dupl check sees one copy, not two.
func assertFetchFailsOnEachSpec(t *testing.T, tests map[string]string, errMessage string) {
	t.Helper()

	for name, spec := range tests {
		t.Run(name, func(t *testing.T) {
			doc := "openapi: 3.0.3\ninfo: {title: bad, version: '1'}\n" + spec

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if _, err := w.Write([]byte(doc)); err != nil {
					t.Errorf("writing malformed response: %v", err)
				}
			}))
			defer server.Close()

			source := specsourcehttp.New([]specsourcehttp.Service{{Name: "bad", URL: server.URL}}, nil)

			catalog, err := source.Fetch(context.Background())

			require.Error(t, err, errMessage)
			assert.Empty(t, catalog.Endpoints)
		})
	}
}

func TestFetchFailsOnMalformedChartHint(t *testing.T) {
	tests := map[string]string{
		"not an object": `
paths:
  /bad:
    get:
      operationId: getBad
      x-orchestra-expose: true
      x-ui-hint:
        chart: "bar"
      responses:
        "200": {description: n/a}
`,
		"missing category": `
paths:
  /bad:
    get:
      operationId: getBad
      x-orchestra-expose: true
      x-ui-hint:
        chart: {value: count, kind: bar}
      responses:
        "200": {description: n/a}
`,
		"missing value": `
paths:
  /bad:
    get:
      operationId: getBad
      x-orchestra-expose: true
      x-ui-hint:
        chart: {category: status, kind: bar}
      responses:
        "200": {description: n/a}
`,
		"kind outside bar/line/pie": `
paths:
  /bad:
    get:
      operationId: getBad
      x-orchestra-expose: true
      x-ui-hint:
        chart: {category: status, value: count, kind: scatter}
      responses:
        "200": {description: n/a}
`,
		"wrong type entirely": `
paths:
  /bad:
    get:
      operationId: getBad
      x-orchestra-expose: true
      x-ui-hint:
        chart: {category: status, value: count, kind: 3}
      responses:
        "200": {description: n/a}
`,
	}

	assertFetchFailsOnEachSpec(t, tests, "a malformed x-ui-hint.chart is a fetch error, not a silently empty hint")
}

// TestFetchConvertsExamples is AC-G-104: x-orchestra-examples travels from
// an operation into domain.Endpoint.Examples, and an operation that
// declares none carries a nil slice rather than an error or a fallback.
func TestFetchConvertsExamples(t *testing.T) {
	server := fixtureServer(t)
	defer server.Close()

	source := specsourcehttp.New(
		[]specsourcehttp.Service{{Name: "fixture", URL: server.URL}},
		nil,
	)

	catalog, err := source.Fetch(context.Background())
	require.NoError(t, err)

	list, ok := catalog.Find("fixture", "listWidgets")
	require.True(t, ok)
	assert.Equal(t, []string{"引当済のウィジェットを見せて", "ウィジェット一覧"}, list.Examples)

	get, ok := catalog.Find("fixture", "getWidget")
	require.True(t, ok)
	assert.Empty(t, get.Examples, "getWidget declares no x-orchestra-examples at all")
}

func TestFetchFailsOnMalformedExamples(t *testing.T) {
	tests := map[string]string{
		"not an array": `
paths:
  /bad:
    get:
      operationId: getBad
      x-orchestra-expose: true
      x-orchestra-examples: "widget list"
      responses:
        "200": {description: n/a}
`,
		"array with a non-string element": `
paths:
  /bad:
    get:
      operationId: getBad
      x-orchestra-expose: true
      x-orchestra-examples: ["widget list", 3]
      responses:
        "200": {description: n/a}
`,
	}

	assertFetchFailsOnEachSpec(
		t, tests, "a malformed x-orchestra-examples is a fetch error, not a silently empty list",
	)
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
	assert.Len(t, catalog.Endpoints, 5, "listWidgets, createWidget, getWidget, exportWidgets, countWidgets")
}
