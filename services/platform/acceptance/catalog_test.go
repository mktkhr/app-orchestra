package acceptance_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/pkg/app"
)

// catalogEntryOnWire is the wire shape of one CatalogEntry, as GET
// /api/catalog lists it.
type catalogEntryOnWire struct {
	Service     string         `json:"service"`
	OperationID string         `json:"operationId"`
	Summary     string         `json:"summary"`
	Component   string         `json:"component"`
	Schema      map[string]any `json:"schema"`
	Fields      map[string]any `json:"fields"`
	View        *struct {
		Chart *struct {
			Category string `json:"category"`
			Value    string `json:"value"`
			Kind     string `json:"kind"`
		} `json:"chart"`
	} `json:"view"`
}

// inventorySpecWithChart is inventorySpec plus an operation whose contract
// declares x-ui-hint.chart, the fixture Task 4's own acceptance criterion
// needs: an operation with the hint carries it as `view`, one without
// omits it (docs/plans/dashboard.md, Task 4).
const inventorySpecWithChart = inventorySpec + `
  /api/inventory/status-counts:
    get:
      operationId: CountInventoryByStatus
      summary: Count stock items by status.
      x-orchestra-expose: true
      x-ui-hint:
        chart:
          category: status
          value: count
          kind: bar
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: array
                items:
                  type: object
                  required:
                    - status
                    - count
                  properties:
                    status:
                      type: string
                    count:
                      type: integer
`

// findCatalogEntry returns the entry named service/operationID among
// entries.
func findCatalogEntry(t *testing.T, entries []catalogEntryOnWire, service, operationID string) catalogEntryOnWire {
	t.Helper()

	for _, e := range entries {
		if e.Service == service && e.OperationID == operationID {
			return e
		}
	}

	t.Fatalf("no catalog entry for %s/%s among %+v", service, operationID, entries)

	return catalogEntryOnWire{}
}

// newCatalogTestApp builds a platform with inventory (ListInventoryItems
// and CountInventoryByStatus, the latter carrying x-ui-hint.chart) and
// attendance (ListAttendanceRecords) both configured, and one non-admin
// account seeded alongside the admin - the same shape newUsersTestApp
// builds, but over two services so a person granted only one can be shown
// to see none of the other's operations (AC-P-101).
func newCatalogTestApp(t *testing.T) *httptest.Server {
	t.Helper()

	inventory := newFixtureService(t, inventorySpecWithChart, "/api/inventory/items", `{"items":[{"id":"1"}]}`)
	attendance := newFixtureService(t, attendanceSpec, "/api/attendance/records", `{"items":[]}`)

	return newTestApp(t, &app.Config{
		Services: []app.Service{
			{Name: "inventory", URL: inventory.server.URL},
			{Name: "attendance", URL: attendance.server.URL},
		},
		SeedAccounts: []app.SeedAccount{
			{Name: nonAdminName, Password: nonAdminPassword, Role: "user"},
		},
	})
}

// TestCatalogListsOnlyWhatAPersonMayCallAndCarriesItsChartHint is
// docs/plans/dashboard.md, Task 4, Step 2's main scenario (AC-P-101,
// AC-P-105's catalogue half): an admin sees every operation across both
// services; a person granted only inventory sees only inventory's, and
// none of attendance's; the operation whose contract declares
// x-ui-hint.chart carries it as `view`, and the one that does not omits
// it.
func TestCatalogListsOnlyWhatAPersonMayCallAndCarriesItsChartHint(t *testing.T) {
	server := newCatalogTestApp(t)

	// The admin sees every operation the catalogue holds, across both
	// services.
	var adminEntries []catalogEntryOnWire
	status := doJSON(t, server, http.MethodGet, "/api/catalog", nil, &adminEntries)
	require.Equal(t, http.StatusOK, status)
	require.Len(t, adminEntries, 3)

	// ListInventoryItems declares no x-ui-hint.chart: it renders as a
	// table, its arguments are described as a schema, and it carries no
	// view.
	listItems := findCatalogEntry(t, adminEntries, "inventory", "ListInventoryItems")
	assert.Equal(t, "table", listItems.Component)
	assert.NotEmpty(t, listItems.Schema)
	assert.Nil(t, listItems.View)

	// CountInventoryByStatus declares x-ui-hint.chart: the axes come back
	// as view.chart, and the component is "chart" (domain.Render, P2) -
	// domain.FieldsSchema only ever describes a table's or a detail's own
	// properties (docs/specs/dashboard.md, Task 3's ChartHint rule wins
	// ahead of both), so a chart-hinted endpoint carries no fields either,
	// exactly as chartViewFor's own Result does for the same endpoint.
	countByStatus := findCatalogEntry(t, adminEntries, "inventory", "CountInventoryByStatus")
	assert.Equal(t, "chart", countByStatus.Component)
	assert.Empty(t, countByStatus.Fields)
	require.NotNil(t, countByStatus.View)
	require.NotNil(t, countByStatus.View.Chart)
	assert.Equal(t, "status", countByStatus.View.Chart.Category)
	assert.Equal(t, "count", countByStatus.View.Chart.Value)
	assert.Equal(t, "bar", countByStatus.View.Chart.Kind)

	// A person granted only the one inventory operation sees only it, and
	// nothing of attendance's operation (AC-P-101).
	yamada := newClientWithJar(t)
	signInAs(t, yamada, server, nonAdminName, nonAdminPassword)

	var beforeGrant []catalogEntryOnWire
	status = doJSONWithClient(t, yamada, server.URL, http.MethodGet, "/api/catalog", nil, &beforeGrant)
	require.Equal(t, http.StatusOK, status)
	assert.Empty(t, beforeGrant, "no permission granted yet")

	var accounts []accountOnWire
	status = doJSON(t, server, http.MethodGet, "/api/users", nil, &accounts)
	require.Equal(t, http.StatusOK, status)
	yamadaID := findAccountID(t, accounts, nonAdminName)

	status = doJSON(t, server, http.MethodPut, "/api/users/"+yamadaID+"/permissions", map[string]any{
		"permissions": []map[string]any{
			{"service": "inventory", "operationId": "ListInventoryItems"},
		},
	}, nil)
	require.Equal(t, http.StatusNoContent, status)

	var narrowed []catalogEntryOnWire
	status = doJSONWithClient(t, yamada, server.URL, http.MethodGet, "/api/catalog", nil, &narrowed)
	require.Equal(t, http.StatusOK, status)
	require.Len(t, narrowed, 1)
	assert.Equal(t, "inventory", narrowed[0].Service)
	assert.Equal(t, "ListInventoryItems", narrowed[0].OperationID)
}

// TestCatalogEndpointIs401WithoutASession is AC-P-101's implicit half: no
// session is 401, the same rule every route but /api/health follows
// (docs/specs/auth.md, AC-A-102).
func TestCatalogEndpointIs401WithoutASession(t *testing.T) {
	server := newCatalogTestApp(t)

	anon := newClientWithJar(t)

	status := doJSONWithClient(t, anon, server.URL, http.MethodGet, "/api/catalog", nil, nil)
	assert.Equal(t, http.StatusUnauthorized, status)
}
