package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

func TestEndpointIsSafe(t *testing.T) {
	tests := []struct {
		method string
		want   bool
	}{
		{"GET", true},
		{"HEAD", true},
		{"QUERY", true},
		{"POST", false},
		{"PUT", false},
		{"PATCH", false},
		{"DELETE", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			e := domain.Endpoint{Method: tt.method}
			assert.Equal(t, tt.want, e.IsSafe())
		})
	}
}

func TestCatalogFindReturnsMatchingEndpoint(t *testing.T) {
	c := domain.Catalog{Endpoints: []domain.Endpoint{
		{Service: "inventory", OperationID: "listItems"},
		{Service: "attendance", OperationID: "listRecords"},
	}}

	e, ok := c.Find("attendance", "listRecords")

	assert.True(t, ok)
	assert.Equal(t, "attendance", e.Service)
	assert.Equal(t, "listRecords", e.OperationID)
}

func TestCatalogFindReturnsFalseWhenMissing(t *testing.T) {
	c := domain.Catalog{Endpoints: []domain.Endpoint{
		{Service: "inventory", OperationID: "listItems"},
	}}

	_, ok := c.Find("inventory", "createItem")

	assert.False(t, ok)
}

func TestCatalogFindOnEmptyCatalog(t *testing.T) {
	var c domain.Catalog

	_, ok := c.Find("inventory", "listItems")

	assert.False(t, ok)
}

// twoServiceCatalog is a catalogue of two services, three operations
// total, used by TestCatalogForNarrowsToTheNamedOperation to prove For
// keeps exactly what permissions name and drops everything else - both
// the sibling operation in the same service and the whole other service.
func twoServiceCatalog() domain.Catalog {
	return domain.Catalog{Endpoints: []domain.Endpoint{
		{Service: "inventory", OperationID: "ListInventoryItems"},
		{Service: "inventory", OperationID: "CreateInventoryItem"},
		{Service: "attendance", OperationID: "ListAttendanceRecords"},
	}}
}

func TestCatalogForNarrowsToTheNamedOperation(t *testing.T) {
	c := twoServiceCatalog()

	narrowed := c.For([]domain.Permission{{Service: "inventory", OperationID: "ListInventoryItems"}})

	assert.Len(t, narrowed.Endpoints, 1, "narrowed catalogue must hold exactly one endpoint")
	assert.Equal(t, "inventory", narrowed.Endpoints[0].Service)
	assert.Equal(t, "ListInventoryItems", narrowed.Endpoints[0].OperationID)

	_, ok := narrowed.Find("inventory", "CreateInventoryItem")
	assert.False(t, ok, "For must drop a sibling operation of the same service the permission did not name")

	_, ok = narrowed.Find("attendance", "ListAttendanceRecords")
	assert.False(t, ok, "For must drop an operation from a service the permission did not name")

	found, ok := narrowed.Find("inventory", "ListInventoryItems")
	assert.True(t, ok)
	assert.Equal(t, "ListInventoryItems", found.OperationID)
}

func TestCatalogForWithNoPermissionsYieldsAnEmptyCatalog(t *testing.T) {
	c := twoServiceCatalog()

	narrowed := c.For(nil)

	assert.Empty(t, narrowed.Endpoints)
}

func TestCatalogForKeepsEveryPermittedOperation(t *testing.T) {
	c := twoServiceCatalog()

	narrowed := c.For([]domain.Permission{
		{Service: "inventory", OperationID: "ListInventoryItems"},
		{Service: "attendance", OperationID: "ListAttendanceRecords"},
	})

	assert.Len(t, narrowed.Endpoints, 2)

	_, ok := narrowed.Find("inventory", "CreateInventoryItem")
	assert.False(t, ok)
}

// TestEndpointDisplayNameOrPrefersItsOwnDisplayName is DECISIONS.md's
// 2026-09-13 entry: a contract that declares x-ui-hint.displayName wins
// over whatever the caller would otherwise have shown.
func TestEndpointDisplayNameOrPrefersItsOwnDisplayName(t *testing.T) {
	e := domain.Endpoint{DisplayName: "在庫一覧"}

	assert.Equal(t, "在庫一覧", e.DisplayNameOr("List stock items."))
}

// TestEndpointDisplayNameOrFallsBackWhenTheContractDeclaresNone is the
// other half: a contract that says nothing about a display name changes
// nothing a person already saw before this field existed.
func TestEndpointDisplayNameOrFallsBackWhenTheContractDeclaresNone(t *testing.T) {
	e := domain.Endpoint{}

	assert.Equal(t, "List stock items.", e.DisplayNameOr("List stock items."))
}
