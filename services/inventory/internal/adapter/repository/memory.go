// Package repository holds the inventory service's storage adapters. Memory
// is the only one: this is a dummy service, so an in-memory fixture stands
// in for whatever real store a production inventory system would use.
package repository

import (
	"fmt"
	"sync"

	"github.com/mktkhr/app-orchestra/services/inventory/internal/domain"
)

// Memory is an in-memory, thread-safe store of stock items, seeded with
// fixture data covering every status.
type Memory struct {
	mu     sync.Mutex
	items  map[string]domain.Item
	nextID int
}

// Quantities on hand for the seed fixture below, named so the fixture data
// carries no unexplained integer literals.
const (
	qtyUSBCable         = 120
	qtyLaptopStand      = 45
	qtyWirelessMouse    = 80
	qtyExternalSSD      = 30
	qtyPackingBoxes     = 200
	qtyLithiumBatteries = 15
	qtyDisplayShelving  = 10
	qtySampleMonitors   = 6
)

// NewMemory builds a Memory store pre-seeded with two items per status, so
// that filtering by status is observable from the fixture alone.
func NewMemory() *Memory {
	seed := []domain.Item{
		{ID: "itm-001", Name: "USB-Cケーブル 1m", Status: domain.StatusAllocated, Quantity: qtyUSBCable},
		{ID: "itm-002", Name: "ノートPCスタンド", Status: domain.StatusAllocated, Quantity: qtyLaptopStand},
		{ID: "itm-003", Name: "ワイヤレスマウス", Status: domain.StatusStaged, Quantity: qtyWirelessMouse},
		{ID: "itm-004", Name: "外付けSSD 1TB", Status: domain.StatusStaged, Quantity: qtyExternalSSD},
		{ID: "itm-005", Name: "梱包用ダンボール", Status: domain.StatusQuarantined, Quantity: qtyPackingBoxes},
		{ID: "itm-006", Name: "リチウムイオン電池パック", Status: domain.StatusQuarantined, Quantity: qtyLithiumBatteries},
		{ID: "itm-007", Name: "季節催事用ディスプレイ棚", Status: domain.StatusConsigned, Quantity: qtyDisplayShelving},
		{ID: "itm-008", Name: "サンプル展示用モニター", Status: domain.StatusConsigned, Quantity: qtySampleMonitors},
	}

	items := make(map[string]domain.Item, len(seed))
	for _, item := range seed {
		items[item.ID] = item
	}

	return &Memory{items: items, nextID: len(seed) + 1}
}

// List returns every item, or, when status is non-nil, only the items in
// that status. The order is not guaranteed.
func (m *Memory) List(status *domain.Status) []domain.Item {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]domain.Item, 0, len(m.items))

	for _, item := range m.items {
		if status != nil && item.Status != *status {
			continue
		}

		result = append(result, item)
	}

	return result
}

// Get returns the item with the given id, and whether it was found.
func (m *Memory) Get(id string) (domain.Item, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, ok := m.items[id]

	return item, ok
}

// Create assigns a new id to n and stores it, returning the stored Item.
func (m *Memory) Create(n domain.NewItem) domain.Item {
	m.mu.Lock()
	defer m.mu.Unlock()

	item := domain.Item{
		ID:       fmt.Sprintf("itm-%03d", m.nextID),
		Name:     n.Name,
		Status:   n.Status,
		Quantity: n.Quantity,
	}
	m.nextID++
	m.items[item.ID] = item

	return item
}
