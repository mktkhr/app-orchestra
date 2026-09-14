package handler_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/handler"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/openapi"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// fakeCatalog is a test double for the catalog interface catalog.go
// declares: it records the user it was called with and answers with
// whatever it was built with.
type fakeCatalog struct {
	entries []usecase.CatalogEntry
	err     error
	user    *domain.User
}

func (f *fakeCatalog) For(_ context.Context, user *domain.User) ([]usecase.CatalogEntry, error) {
	f.user = user

	return f.entries, f.err
}

// TestGetCatalogConvertsEveryEntryOntoTheWire checks the two conversions
// toAPICatalogEntry makes: Fields is present only when non-empty (matching
// PlanResult.Fields' own rule), and View round-trips through toAPIView the
// same way a Panel's or a PlanResult's own View does.
func TestGetCatalogConvertsEveryEntryOntoTheWire(t *testing.T) {
	c := &fakeCatalog{entries: []usecase.CatalogEntry{
		{
			Service:            "inventory",
			ServiceDisplayName: "在庫管理",
			OperationID:        "ListInventoryItems",
			Summary:            "List stock items.",
			Component:          domain.ComponentTable,
			Schema:             map[string]any{"type": "object"},
			Fields:             map[string]any{"status": map[string]any{"type": "string"}},
			Examples:           []string{"在庫を見せて"},
		},
		{
			Service:     "inventory",
			OperationID: "CountInventoryByStatus",
			Summary:     "Count stock items by status.",
			Component:   domain.ComponentChart,
			Schema:      map[string]any{"type": "object"},
			View: &domain.View{Chart: &domain.Chart{
				Category: "status", Value: "count", Kind: domain.ChartKindBar,
			}},
		},
	}}

	h := handler.NewCatalog(c)

	resp, err := h.GetCatalog(handler.WithUser(t.Context(), testAdminUser), openapi.GetCatalogRequestObject{})
	require.NoError(t, err)

	out, ok := resp.(openapi.GetCatalog200JSONResponse)
	require.True(t, ok)
	require.Len(t, out, 2)

	assert.Equal(t, "inventory", out[0].Service)
	assert.Equal(t, "在庫管理", out[0].ServiceDisplayName)
	assert.Equal(t, "ListInventoryItems", out[0].OperationId)
	assert.Equal(t, openapi.Component("table"), out[0].Component)
	require.NotNil(t, out[0].Fields)
	assert.Nil(t, out[0].View)
	require.NotNil(t, out[0].Examples)
	assert.Equal(t, []string{"在庫を見せて"}, *out[0].Examples)

	require.NotNil(t, out[1].View)
	require.NotNil(t, out[1].View.Chart)
	assert.Equal(t, "status", out[1].View.Chart.Category)
	assert.Nil(t, out[1].Fields, "no fields given: absent, not an empty object")
	assert.Nil(t, out[1].Examples, "no examples given: absent, not an empty array")

	assert.Same(t, testAdminUser, c.user, "the signed-in user reaches the usecase")
}

// TestGetCatalogReturnsAnErrorWhenTheUsecaseFails proves a usecase
// failure is reported as an error rather than a wire response - the same
// shape every other handler in this package follows for its own store
// failures (see workspace_test.go).
func TestGetCatalogReturnsAnErrorWhenTheUsecaseFails(t *testing.T) {
	boom := errors.New("boom")
	h := handler.NewCatalog(&fakeCatalog{err: boom})

	_, err := h.GetCatalog(handler.WithUser(t.Context(), testAdminUser), openapi.GetCatalogRequestObject{})

	require.Error(t, err)
}

// TestGetCatalogWithNoEntriesReturnsAnEmptyList is the zero case: nothing
// granted is an empty list, not a nil response.
func TestGetCatalogWithNoEntriesReturnsAnEmptyList(t *testing.T) {
	h := handler.NewCatalog(&fakeCatalog{})

	resp, err := h.GetCatalog(handler.WithUser(t.Context(), testAdminUser), openapi.GetCatalogRequestObject{})
	require.NoError(t, err)

	out, ok := resp.(openapi.GetCatalog200JSONResponse)
	require.True(t, ok)
	assert.Empty(t, out)
}
