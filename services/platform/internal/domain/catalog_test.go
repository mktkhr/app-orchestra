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
