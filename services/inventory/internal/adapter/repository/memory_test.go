package repository_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/inventory/internal/adapter/repository"
	"github.com/mktkhr/app-orchestra/services/inventory/internal/domain"
)

func TestMemoryListReturnsAllItemsWhenStatusIsNil(t *testing.T) {
	t.Parallel()

	m := repository.NewMemory()

	items := m.List(nil)

	assert.Len(t, items, 8)
}

func TestMemoryListFiltersByStatus(t *testing.T) {
	t.Parallel()

	m := repository.NewMemory()
	status := domain.StatusQuarantined

	items := m.List(&status)

	require.Len(t, items, 2)
	for _, item := range items {
		assert.Equal(t, domain.StatusQuarantined, item.Status)
	}
}

func TestMemoryGetFindsExistingItem(t *testing.T) {
	t.Parallel()

	m := repository.NewMemory()

	item, ok := m.Get("itm-001")

	require.True(t, ok)
	assert.Equal(t, "itm-001", item.ID)
}

func TestMemoryGetReportsMissingItem(t *testing.T) {
	t.Parallel()

	m := repository.NewMemory()

	_, ok := m.Get("does-not-exist")

	assert.False(t, ok)
}

func TestMemoryCreateAssignsIDAndStores(t *testing.T) {
	t.Parallel()

	m := repository.NewMemory()

	created := m.Create(domain.NewItem{Name: "新規アイテム", Status: domain.StatusAllocated, Quantity: 5})

	assert.NotEmpty(t, created.ID)
	assert.Equal(t, "新規アイテム", created.Name)

	fetched, ok := m.Get(created.ID)
	require.True(t, ok)
	assert.Equal(t, created, fetched)
}

func TestMemoryCreateAssignsDistinctIDs(t *testing.T) {
	t.Parallel()

	m := repository.NewMemory()

	first := m.Create(domain.NewItem{Name: "一つ目", Status: domain.StatusStaged, Quantity: 1})
	second := m.Create(domain.NewItem{Name: "二つ目", Status: domain.StatusStaged, Quantity: 1})

	assert.NotEqual(t, first.ID, second.ID)
}
