package handler_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/mktkhr/app-orchestra/services/inventory/internal/adapter/handler"
	"github.com/mktkhr/app-orchestra/services/inventory/internal/adapter/openapi"
	"github.com/mktkhr/app-orchestra/services/inventory/internal/adapter/repository"
)

func TestItemsListInventoryItemsReturnsEveryItemWithoutFilter(t *testing.T) {
	h := handler.NewItems(repository.NewMemory())

	resp, err := h.ListInventoryItems(t.Context(), openapi.ListInventoryItemsRequestObject{})

	require.NoError(t, err)
	list, ok := resp.(openapi.ListInventoryItems200JSONResponse)
	require.True(t, ok)
	assert.Len(t, list.Items, 8)
}

func TestItemsListInventoryItemsFiltersByStatus(t *testing.T) {
	h := handler.NewItems(repository.NewMemory())
	status := openapi.Staged

	resp, err := h.ListInventoryItems(t.Context(), openapi.ListInventoryItemsRequestObject{
		Params: openapi.ListInventoryItemsParams{Status: &status},
	})

	require.NoError(t, err)
	list, ok := resp.(openapi.ListInventoryItems200JSONResponse)
	require.True(t, ok)
	require.NotEmpty(t, list.Items)
	for _, item := range list.Items {
		assert.Equal(t, openapi.Staged, item.Status)
	}
}

func TestItemsGetInventoryItemReturnsExistingItem(t *testing.T) {
	h := handler.NewItems(repository.NewMemory())

	resp, err := h.GetInventoryItem(t.Context(), openapi.GetInventoryItemRequestObject{Id: "itm-001"})

	require.NoError(t, err)
	got, ok := resp.(openapi.GetInventoryItem200JSONResponse)
	require.True(t, ok)
	assert.Equal(t, "itm-001", got.Id)
}

func TestItemsGetInventoryItemReportsMissingItem(t *testing.T) {
	h := handler.NewItems(repository.NewMemory())

	resp, err := h.GetInventoryItem(t.Context(), openapi.GetInventoryItemRequestObject{Id: "does-not-exist"})

	require.NoError(t, err)
	_, ok := resp.(openapi.GetInventoryItem404JSONResponse)
	assert.True(t, ok)
}

func TestItemsCreateInventoryItemStoresAndReturnsItem(t *testing.T) {
	h := handler.NewItems(repository.NewMemory())

	resp, err := h.CreateInventoryItem(t.Context(), openapi.CreateInventoryItemRequestObject{
		Body: &openapi.CreateInventoryItemJSONRequestBody{
			Name:     "テストアイテム",
			Status:   openapi.Consigned,
			Quantity: 7,
		},
	})

	require.NoError(t, err)
	created, ok := resp.(openapi.CreateInventoryItem201JSONResponse)
	require.True(t, ok)
	assert.NotEmpty(t, created.Id)
	assert.Equal(t, "テストアイテム", created.Name)
	assert.Equal(t, openapi.Consigned, created.Status)
	assert.Equal(t, 7, created.Quantity)
}

func TestItemsGetInventorySpecServesParsableYAML(t *testing.T) {
	h := handler.NewItems(repository.NewMemory())

	resp, err := h.GetInventorySpec(t.Context(), openapi.GetInventorySpecRequestObject{})

	require.NoError(t, err)
	got, ok := resp.(openapi.GetInventorySpec200ApplicationyamlResponse)
	require.True(t, ok)

	var doc map[string]any
	require.NoError(t, yaml.NewDecoder(got.Body).Decode(&doc))
	assert.Equal(t, "3.0.3", doc["openapi"])
}
