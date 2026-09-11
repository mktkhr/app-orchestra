// Package domain holds the inventory service's core types. Pure; no I/O, no
// dependency on the generated OpenAPI code or on any other layer.
package domain

// Status is where a stock item sits in the inventory workflow. The English
// value is what travels over the wire; the Japanese meaning is carried
// alongside it in the spec's x-enum-labels, not here.
type Status string

// The four statuses the inventory service knows about, verbatim from
// docs/plans/orchestration.md.
const (
	StatusAllocated   Status = "allocated"
	StatusStaged      Status = "staged"
	StatusQuarantined Status = "quarantined"
	StatusConsigned   Status = "consigned"
)

// Valid reports whether s is one of the four known statuses.
func (s Status) Valid() bool {
	switch s {
	case StatusAllocated, StatusStaged, StatusQuarantined, StatusConsigned:
		return true
	default:
		return false
	}
}

// Item is a single stock item.
type Item struct {
	ID       string
	Name     string
	Status   Status
	Quantity int
}

// NewItem is the data needed to create an Item; it has no ID because the
// repository assigns one.
type NewItem struct {
	Name     string
	Status   Status
	Quantity int
}
