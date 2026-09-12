package domain

// Aggregate names how Transform reduces the rows sharing a group. A string,
// not an int enum, because it is compared against the wire value directly
// (docs/plans/dashboard.md, "The shape everything shares") and this
// package may not import encoding/json to unmarshal one
// (harness/quality/go/golangci.yml, depguard).
type Aggregate string

// The three aggregates a Transform can name.
const (
	AggregateCount Aggregate = "count"
	AggregateSum   Aggregate = "sum"
	AggregateAvg   Aggregate = "avg"
)

// ChartKind names which of @mui/x-charts' three chart types a Chart draws
// as.
type ChartKind string

// The three chart kinds a Chart can name.
const (
	ChartKindBar  ChartKind = "bar"
	ChartKindLine ChartKind = "line"
	ChartKindPie  ChartKind = "pie"
)

// Transform is a panel's optional grouping: group the rows by GroupBy and
// reduce each group with Aggregate, reading Field for sum and avg (empty
// for count). It runs in the browser (docs/specs/dashboard.md, P3) - this
// type only carries what a panel or a contract says, and computes nothing
// itself.
type Transform struct {
	GroupBy   string
	Aggregate Aggregate
	// Field is the field Aggregate reads. Empty when Aggregate is
	// AggregateCount, which reads nothing.
	Field string
}

// Chart is a panel's or a contract's chart axes: which field names the
// category, which names the value, and which of the three kinds to draw.
type Chart struct {
	Category string
	Value    string
	Kind     ChartKind
}

// View is how a panel draws its result, beside the call that says what to
// fetch (docs/specs/dashboard.md, P1). Both halves are independent and
// either may be absent: a table with a Transform is a perfectly good
// panel, and a Chart over an endpoint that already returns aggregates
// needs no Transform at all (docs/specs/dashboard.md, section 3).
type View struct {
	Transform *Transform
	Chart     *Chart
}
