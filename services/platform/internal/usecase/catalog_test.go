package usecase_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// findCatalogEntry returns the inventory entry named operationID among
// entries. Every test here reads an inventory operation - the attendance
// service's presence is what the admin test asserts, not what any of these
// read a field off - so the service is not a parameter (unparam).
func findCatalogEntry(t *testing.T, entries []usecase.CatalogEntry, operationID string) usecase.CatalogEntry {
	t.Helper()

	for _, e := range entries {
		if e.Service == "inventory" && e.OperationID == operationID {
			return e
		}
	}

	t.Fatalf("no inventory catalog entry for %s among %+v", operationID, entries)

	return usecase.CatalogEntry{}
}

// TestCatalogForAdminListsEveryOperationAcrossEveryService is Task 4's
// admin-sees-everything half of AC-P-101: an admin never consults the
// permission store (the same rule Orchestrator and Workspaces follow),
// and every endpoint of every configured service comes back.
func TestCatalogForAdminListsEveryOperationAcrossEveryService(t *testing.T) {
	permissions := &fakePermissionStore{} // grants nothing - admin must not need it

	c := usecase.NewCatalog(twoServiceCatalog(), permissions)

	entries, err := c.For(t.Context(), adminUser())

	require.NoError(t, err)
	assert.Len(t, entries, 3)
	assert.Empty(t, permissions.userID, "an admin's catalogue must never depend on a call to PermissionStore.For")
}

// TestCatalogForANonAdminListsOnlyWhatTheyMayCall is AC-P-101's other
// half: a person granted one operation sees only it, and none of the
// other service's - narrowed through the same catalogFor Orchestrator and
// Workspaces use (docs/specs/auth.md, section 5).
func TestCatalogForANonAdminListsOnlyWhatTheyMayCall(t *testing.T) {
	permissions := &fakePermissionStore{permissions: []domain.Permission{
		{Service: "inventory", OperationID: "ListInventoryItems"},
	}}

	c := usecase.NewCatalog(twoServiceCatalog(), permissions)

	entries, err := c.For(t.Context(), regularUser())

	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "inventory", entries[0].Service)
	assert.Equal(t, "ListInventoryItems", entries[0].OperationID)
}

// TestCatalogForANonAdminWithNoPermissionsListsNothing is the zero case of
// the same rule: no permission at all is an empty catalogue, not the whole
// one (domain.Catalog.For's own contract).
func TestCatalogForANonAdminWithNoPermissionsListsNothing(t *testing.T) {
	c := usecase.NewCatalog(twoServiceCatalog(), &fakePermissionStore{})

	entries, err := c.For(t.Context(), regularUser())

	require.NoError(t, err)
	assert.Empty(t, entries)
}

// TestCatalogForPropagatesAPermissionStoreError proves For does not
// swallow catalogFor's own error - the same failure Orchestrator.Plan and
// Workspaces.AddPanel both surface for the same reason.
func TestCatalogForPropagatesAPermissionStoreError(t *testing.T) {
	boom := assert.AnError
	c := usecase.NewCatalog(twoServiceCatalog(), &fakePermissionStore{err: boom})

	_, err := c.For(t.Context(), regularUser())

	require.Error(t, err)
}

// TestCatalogEntryCarriesTheSchemaFieldsAndComponentOfEachEndpoint checks
// the shape docs/specs/dashboard.md section 5 describes: the arguments
// schema (inputSchemaFor), the response's fields (domain.FieldsSchema, via
// fieldsFor) and the component domain.Render would choose.
func TestCatalogEntryCarriesTheSchemaFieldsAndComponentOfEachEndpoint(t *testing.T) {
	c := usecase.NewCatalog(inventoryCatalogWithStatusColumn(), &fakePermissionStore{})

	entries, err := c.For(t.Context(), adminUser())
	require.NoError(t, err)

	entry := findCatalogEntry(t, entries, "ListInventoryItems")

	assert.Equal(t, domain.ComponentTable, entry.Component)
	assert.NotEmpty(t, entry.Schema, "the call's arguments, from inputSchemaFor")
	assert.NotEmpty(t, entry.Fields, "the response's fields, from domain.FieldsSchema")
	assert.Nil(t, entry.View, "no x-ui-hint.chart on this endpoint")
}

// TestCatalogEntryCarriesTheEndpointsChartHintAsView is AC-P-105's
// catalogue half (docs/plans/dashboard.md, Task 4): an operation whose
// contract declares x-ui-hint.chart carries it as View, built the same way
// chartViewFor builds one for a /api/plan result.
func TestCatalogEntryCarriesTheEndpointsChartHintAsView(t *testing.T) {
	c := usecase.NewCatalog(inventoryCatalogWithChartHint(), &fakePermissionStore{})

	entries, err := c.For(t.Context(), adminUser())
	require.NoError(t, err)

	entry := findCatalogEntry(t, entries, "ListInventoryItems")

	require.NotNil(t, entry.View)
	require.NotNil(t, entry.View.Chart)
	assert.Equal(t, "status", entry.View.Chart.Category)
	assert.Equal(t, "count", entry.View.Chart.Value)
	assert.Equal(t, domain.ChartKindBar, entry.View.Chart.Kind)
	assert.Nil(t, entry.View.Transform, "a contract declares axes, never a transform")
}

// TestCatalogEntryDisplayNameFallsBackToSummary is DECISIONS.md's
// 2026-09-13 entry's catalogue half: an endpoint whose contract declares
// no x-ui-hint.displayName still gives OperationPicker and the default
// panel title something to show - exactly what they already showed
// (Summary) before this field existed.
func TestCatalogEntryDisplayNameFallsBackToSummary(t *testing.T) {
	c := usecase.NewCatalog(inventoryCatalog(), &fakePermissionStore{})

	entries, err := c.For(t.Context(), adminUser())
	require.NoError(t, err)

	entry := findCatalogEntry(t, entries, "ListInventoryItems")

	assert.Equal(t, entry.Summary, entry.DisplayName)
}

// TestCatalogEntryDisplayNamePrefersTheContractsOwnDisplayName is the
// other half: a contract that declares x-ui-hint.displayName wins over
// Summary.
func TestCatalogEntryDisplayNamePrefersTheContractsOwnDisplayName(t *testing.T) {
	catalog := domain.Catalog{Endpoints: []domain.Endpoint{
		{
			Service:     "inventory",
			OperationID: "ListInventoryItems",
			Summary:     "List stock items, optionally filtered by status.",
			DisplayName: "在庫一覧",
			Response:    &domain.Schema{Type: domain.SchemaTypeObject},
		},
	}}

	c := usecase.NewCatalog(catalog, &fakePermissionStore{})

	entries, err := c.For(t.Context(), adminUser())
	require.NoError(t, err)

	entry := findCatalogEntry(t, entries, "ListInventoryItems")

	assert.Equal(t, "在庫一覧", entry.DisplayName)
}

// TestCatalogEntryServiceDisplayNameFallsBackToService is DECISIONS.md's
// 2026-09-13 entry's catalogue half, one level up: an endpoint whose
// service declares no info.x-ui-hint.displayName still gives
// OperationPicker's group headers something to show - the identifier
// itself, exactly what they already showed before this field existed.
func TestCatalogEntryServiceDisplayNameFallsBackToService(t *testing.T) {
	c := usecase.NewCatalog(inventoryCatalog(), &fakePermissionStore{})

	entries, err := c.For(t.Context(), adminUser())
	require.NoError(t, err)

	entry := findCatalogEntry(t, entries, "ListInventoryItems")

	assert.Equal(t, "inventory", entry.ServiceDisplayName)
}

// TestCatalogEntryServiceDisplayNamePrefersTheContractsOwnDisplayName is
// the other half: a service's contract that declares
// info.x-ui-hint.displayName wins over its identifier.
func TestCatalogEntryServiceDisplayNamePrefersTheContractsOwnDisplayName(t *testing.T) {
	catalog := domain.Catalog{Endpoints: []domain.Endpoint{
		{
			Service:            "inventory",
			ServiceDisplayName: "在庫管理",
			OperationID:        "ListInventoryItems",
			Summary:            "List stock items, optionally filtered by status.",
			Response:           &domain.Schema{Type: domain.SchemaTypeObject},
		},
	}}

	c := usecase.NewCatalog(catalog, &fakePermissionStore{})

	entries, err := c.For(t.Context(), adminUser())
	require.NoError(t, err)

	entry := findCatalogEntry(t, entries, "ListInventoryItems")

	assert.Equal(t, "在庫管理", entry.ServiceDisplayName)
}

// TestCatalogEntryOmitsFieldsWhenTheResponseDescribesNone matches
// /api/plan's own rule for the same field: an endpoint whose response has
// nothing FieldsSchema can describe (an unsafe create, here) carries no
// Fields at all - absent, not an empty map.
func TestCatalogEntryOmitsFieldsWhenTheResponseDescribesNone(t *testing.T) {
	c := usecase.NewCatalog(inventoryCatalog(), &fakePermissionStore{})

	entries, err := c.For(t.Context(), adminUser())
	require.NoError(t, err)

	entry := findCatalogEntry(t, entries, "CreateInventoryItem")

	assert.Nil(t, entry.Fields)
}
