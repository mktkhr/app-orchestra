// Package repository holds the attendance service's storage adapters.
// Memory is the only one: this is a dummy service, so an in-memory fixture
// stands in for whatever real store a production attendance system would
// use.
package repository

import (
	"fmt"
	"sync"

	"github.com/mktkhr/app-orchestra/services/attendance/internal/domain"
)

// Memory is an in-memory, thread-safe store of attendance records, seeded
// with fixture data covering every kind.
type Memory struct {
	mu     sync.Mutex
	items  map[string]domain.Record
	nextID int
}

// NewMemory builds a Memory store pre-seeded with at least two records per
// kind, so that filtering by kind is observable from the fixture alone.
// substitute (振替休日, arranged before the work) and compensatory (代休,
// granted after it) each get their own records so that a query for one kind
// cannot accidentally be satisfied by the other's fixture data.
func NewMemory() *Memory {
	seed := []domain.Record{
		{ID: "att-001", Employee: "田中太郎", Kind: domain.KindDeemed, Date: "2026-04-01"},
		{ID: "att-002", Employee: "佐藤花子", Kind: domain.KindDeemed, Date: "2026-04-02"},
		{ID: "att-003", Employee: "鈴木一郎", Kind: domain.KindSubstitute, Date: "2026-04-05"},
		{ID: "att-004", Employee: "高橋美咲", Kind: domain.KindSubstitute, Date: "2026-04-06"},
		{ID: "att-005", Employee: "伊藤健太", Kind: domain.KindCompensatory, Date: "2026-04-10"},
		{ID: "att-006", Employee: "渡辺由美", Kind: domain.KindCompensatory, Date: "2026-04-11"},
		{ID: "att-007", Employee: "中村賢一", Kind: domain.KindOnCall, Date: "2026-04-15"},
		{ID: "att-008", Employee: "小林恵子", Kind: domain.KindOnCall, Date: "2026-04-16"},
	}

	items := make(map[string]domain.Record, len(seed))
	for _, item := range seed {
		items[item.ID] = item
	}

	return &Memory{items: items, nextID: len(seed) + 1}
}

// List returns every record, or, when kind is non-nil, only the records of
// that kind. The order is not guaranteed.
func (m *Memory) List(kind *domain.Kind) []domain.Record {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]domain.Record, 0, len(m.items))

	for _, item := range m.items {
		if kind != nil && item.Kind != *kind {
			continue
		}

		result = append(result, item)
	}

	return result
}

// Get returns the record with the given id, and whether it was found.
func (m *Memory) Get(id string) (domain.Record, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, ok := m.items[id]

	return item, ok
}

// Create assigns a new id to n and stores it, returning the stored Record.
func (m *Memory) Create(n domain.NewRecord) domain.Record {
	m.mu.Lock()
	defer m.mu.Unlock()

	item := domain.Record{
		ID:       fmt.Sprintf("att-%03d", m.nextID),
		Employee: n.Employee,
		Kind:     n.Kind,
		Date:     n.Date,
	}
	m.nextID++
	m.items[item.ID] = item

	return item
}
