package repository_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/attendance/internal/adapter/repository"
	"github.com/mktkhr/app-orchestra/services/attendance/internal/domain"
)

func TestMemoryListReturnsAllRecordsWhenKindIsNil(t *testing.T) {
	t.Parallel()

	m := repository.NewMemory()

	items := m.List(nil)

	assert.Len(t, items, 8)
}

func TestMemoryListFiltersByKind(t *testing.T) {
	t.Parallel()

	m := repository.NewMemory()
	kind := domain.KindOnCall

	items := m.List(&kind)

	require.Len(t, items, 2)
	for _, item := range items {
		assert.Equal(t, domain.KindOnCall, item.Kind)
	}
}

// Substitute holidays and compensatory days off are different things in
// Japanese labour practice; this test guards against the two being
// conflated in the fixture data or in List's filtering.
func TestMemoryListDistinguishesSubstituteAndCompensatory(t *testing.T) {
	t.Parallel()

	m := repository.NewMemory()
	substitute := domain.KindSubstitute
	compensatory := domain.KindCompensatory

	substituteItems := m.List(&substitute)
	compensatoryItems := m.List(&compensatory)

	require.NotEmpty(t, substituteItems)
	require.NotEmpty(t, compensatoryItems)
	for _, item := range substituteItems {
		assert.Equal(t, domain.KindSubstitute, item.Kind)
	}
	for _, item := range compensatoryItems {
		assert.Equal(t, domain.KindCompensatory, item.Kind)
	}
}

func TestMemoryGetFindsExistingRecord(t *testing.T) {
	t.Parallel()

	m := repository.NewMemory()

	item, ok := m.Get("att-001")

	require.True(t, ok)
	assert.Equal(t, "att-001", item.ID)
}

func TestMemoryGetReportsMissingRecord(t *testing.T) {
	t.Parallel()

	m := repository.NewMemory()

	_, ok := m.Get("does-not-exist")

	assert.False(t, ok)
}

func TestMemoryCreateAssignsIDAndStores(t *testing.T) {
	t.Parallel()

	m := repository.NewMemory()

	created := m.Create(domain.NewRecord{Employee: "新入社員", Kind: domain.KindDeemed, Date: "2026-05-01"})

	assert.NotEmpty(t, created.ID)
	assert.Equal(t, "新入社員", created.Employee)

	fetched, ok := m.Get(created.ID)
	require.True(t, ok)
	assert.Equal(t, created, fetched)
}

func TestMemoryCreateAssignsDistinctIDs(t *testing.T) {
	t.Parallel()

	m := repository.NewMemory()

	first := m.Create(domain.NewRecord{Employee: "一人目", Kind: domain.KindSubstitute, Date: "2026-05-02"})
	second := m.Create(domain.NewRecord{Employee: "二人目", Kind: domain.KindSubstitute, Date: "2026-05-03"})

	assert.NotEqual(t, first.ID, second.ID)
}
