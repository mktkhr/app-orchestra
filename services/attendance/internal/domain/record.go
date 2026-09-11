// Package domain holds the attendance service's core types. Pure; no I/O, no
// dependency on the generated OpenAPI code or on any other layer.
package domain

// Kind is what sort of attendance arrangement a record represents. The
// English value is what travels over the wire; the Japanese meaning is
// carried alongside it in the spec's x-enum-labels, not here.
//
// substitute (振替休日) and compensatory (代休) are different things in
// Japanese labour practice: a substitute holiday is arranged before the work
// happens, a compensatory day off is granted after it. Nothing in the
// English words distinguishes them.
type Kind string

// The four kinds the attendance service knows about, verbatim from
// docs/plans/orchestration.md.
const (
	KindDeemed       Kind = "deemed"
	KindSubstitute   Kind = "substitute"
	KindCompensatory Kind = "compensatory"
	KindOnCall       Kind = "on_call"
)

// Valid reports whether k is one of the four known kinds.
func (k Kind) Valid() bool {
	switch k {
	case KindDeemed, KindSubstitute, KindCompensatory, KindOnCall:
		return true
	default:
		return false
	}
}

// Record is a single attendance record.
type Record struct {
	ID       string
	Employee string
	Kind     Kind
	Date     string
}

// NewRecord is the data needed to create a Record; it has no ID because the
// repository assigns one.
type NewRecord struct {
	Employee string
	Kind     Kind
	Date     string
}
